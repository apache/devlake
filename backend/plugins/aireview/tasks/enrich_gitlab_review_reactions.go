/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

var EnrichGitlabReviewReactionsMeta = plugin.SubTaskMeta{
	Name:             "enrichGitlabReviewReactions",
	EntryPoint:       EnrichGitlabReviewReactions,
	EnabledByDefault: true,
	Description:      "Enrich AI reviews with reaction data from GitLab award emoji API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAiReviewsMeta},
}

// gitlabConn holds the minimal fields needed to call the GitLab API.
// Token uses the encdec serializer so GORM decrypts it automatically.
type gitlabConn struct {
	ID       uint64 `gorm:"primaryKey;column:id"`
	Endpoint string `gorm:"column:endpoint"`
	Token    string `gorm:"column:token;serializer:encdec"`
}

func (gitlabConn) TableName() string { return "_tool_gitlab_connections" }

// gitlabNoteInfo holds the GitLab-specific data needed to call the award emoji API
type gitlabNoteInfo struct {
	ReviewId        string `gorm:"column:review_id"`
	NoteGitlabId    int    `gorm:"column:note_gitlab_id"`
	ConnectionId    uint64 `gorm:"column:connection_id"`
	MergeRequestIid int    `gorm:"column:merge_request_iid"`
	ProjectId       int    `gorm:"column:project_id"`
}

// awardEmoji represents a single GitLab award emoji from the API response
type awardEmoji struct {
	Name string `json:"name"`
}

// EnrichGitlabReviewReactions fetches award emojis from the GitLab API
// for AI review notes and updates the reaction counts on each review.
func EnrichGitlabReviewReactions(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	logger.Info("Starting GitLab review reaction enrichment via award emoji API")

	// Step 1: Get GitLab reviews with their domain comment IDs
	var reviewClauses []dal.Clause
	if data.Options.ProjectName != "" {
		reviewClauses = []dal.Clause{
			dal.Select("ar.id as review_id, ar.review_id as domain_comment_id"),
			dal.From("_tool_aireview_reviews ar"),
			dal.Join("JOIN pull_requests pr ON ar.pull_request_id = pr.id"),
			dal.Join("JOIN project_mapping pm ON pr.base_repo_id = pm.row_id"),
			dal.Where("ar.source_platform = ? AND pm.project_name = ? AND pm.`table` = ?", "gitlab", data.Options.ProjectName, "repos"),
		}
	} else {
		reviewClauses = []dal.Clause{
			dal.Select("ar.id as review_id, ar.review_id as domain_comment_id"),
			dal.From("_tool_aireview_reviews ar"),
			dal.Where("ar.source_platform = ? AND ar.repo_id = ?", "gitlab", data.Options.RepoId),
		}
	}

	type reviewRow struct {
		ReviewId        string `gorm:"column:review_id"`
		DomainCommentId string `gorm:"column:domain_comment_id"`
	}

	cursor, err := db.Cursor(reviewClauses...)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query GitLab reviews")
	}
	defer cursor.Close()

	// Parse domain comment IDs to extract connection_id and note gitlab_id
	// Format: "gitlab:GitlabMrComment:CONNECTION_ID:GITLAB_ID"
	type noteKey struct {
		ConnectionId uint64
		GitlabId     int
	}
	noteToReviewId := make(map[noteKey]string)

	for cursor.Next() {
		var row reviewRow
		if fetchErr := db.Fetch(cursor, &row); fetchErr != nil {
			return errors.Default.Wrap(fetchErr, "failed to fetch GitLab review")
		}
		connId, gitlabId, parseErr := parseGitlabDomainId(row.DomainCommentId)
		if parseErr != nil {
			logger.Warn(nil, "skipping review with unparseable domain ID: %s", row.DomainCommentId)
			continue
		}
		noteToReviewId[noteKey{connId, gitlabId}] = row.ReviewId
	}

	if len(noteToReviewId) == 0 {
		logger.Info("No GitLab reviews found, skipping award emoji enrichment")
		return nil
	}

	// Group by connection_id
	connectionNotes := make(map[uint64][]noteKey)
	for nk := range noteToReviewId {
		connectionNotes[nk.ConnectionId] = append(connectionNotes[nk.ConnectionId], nk)
	}

	logger.Info("Found %d GitLab reviews across %d connections", len(noteToReviewId), len(connectionNotes))

	httpClient := &http.Client{Timeout: 30 * time.Second}
	totalEnriched := 0
	totalWithReactions := 0

	// Step 2: For each connection, load credentials and resolve note details
	for connId, notes := range connectionNotes {
		// Load GitLab connection (token is auto-decrypted by GORM serializer)
		var conn gitlabConn
		connErr := db.First(&conn, dal.Where("id = ?", connId))
		if connErr != nil {
			logger.Warn(connErr, "failed to load GitLab connection %d, skipping %d reviews", connId, len(notes))
			continue
		}

		// Collect note gitlab_ids for batch query
		gitlabIds := make([]int, 0, len(notes))
		for _, nk := range notes {
			gitlabIds = append(gitlabIds, nk.GitlabId)
		}

		// Query _tool_gitlab_mr_comments joined with _tool_gitlab_merge_requests
		// to get merge_request_iid and project_id for each note
		infoCursor, infoErr := db.Cursor(
			dal.Select("gmc.gitlab_id as note_gitlab_id, gmc.connection_id, gmc.merge_request_iid, gmr.project_id"),
			dal.From("_tool_gitlab_mr_comments gmc"),
			dal.Join("JOIN _tool_gitlab_merge_requests gmr ON gmc.connection_id = gmr.connection_id AND gmc.merge_request_id = gmr.gitlab_id"),
			dal.Where("gmc.connection_id = ? AND gmc.gitlab_id IN (?)", connId, gitlabIds),
		)
		if infoErr != nil {
			logger.Warn(infoErr, "failed to query GitLab note details for connection %d, skipping", connId)
			continue
		}
		defer infoCursor.Close()

		// Collect note info
		var noteInfos []gitlabNoteInfo
		for infoCursor.Next() {
			var info gitlabNoteInfo
			if fetchErr := db.Fetch(infoCursor, &info); fetchErr != nil {
				logger.Warn(errors.Default.Wrap(fetchErr, ""), "failed to fetch note info")
				continue
			}
			info.ReviewId = noteToReviewId[noteKey{connId, info.NoteGitlabId}]
			noteInfos = append(noteInfos, info)
		}

		endpoint := strings.TrimRight(conn.Endpoint, "/")

		// Step 3: Call award emoji API for each note
		for _, info := range noteInfos {
			emojis, fetchErr := fetchAwardEmojis(
				taskCtx.GetContext(), httpClient, endpoint, conn.Token,
				info.ProjectId, info.MergeRequestIid, info.NoteGitlabId,
			)
			if fetchErr != nil {
				logger.Warn(nil, "failed to fetch award emojis for note %d: %s", info.NoteGitlabId, fetchErr)
				continue
			}

			thumbsUp, thumbsDown, total := countReactionEmojis(emojis)

			updateErr := db.Exec(
				"UPDATE _tool_aireview_reviews SET reactions_total_count = ?, reactions_thumbs_up = ?, reactions_thumbs_down = ? WHERE id = ?",
				total, thumbsUp, thumbsDown, info.ReviewId,
			)
			if updateErr != nil {
				logger.Warn(updateErr, "failed to update reactions for review %s", info.ReviewId)
				continue
			}

			totalEnriched++
			if total > 0 {
				totalWithReactions++
			}
		}
	}

	logger.Info("GitLab reaction enrichment complete: %d reviews enriched, %d had reactions", totalEnriched, totalWithReactions)
	return nil
}

// parseGitlabDomainId extracts connection_id and gitlab_id from a domain layer ID.
// Format: "gitlab:GitlabMrComment:CONNECTION_ID:GITLAB_ID"
func parseGitlabDomainId(domainId string) (uint64, int, error) {
	parts := strings.Split(domainId, ":")
	if len(parts) < 4 {
		return 0, 0, fmt.Errorf("expected at least 4 parts, got %d", len(parts))
	}
	connId, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid connection ID %q: %w", parts[2], err)
	}
	gitlabId, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid gitlab ID %q: %w", parts[3], err)
	}
	return connId, gitlabId, nil
}

// fetchAwardEmojis calls the GitLab API to get award emojis for a MR note.
func fetchAwardEmojis(ctx context.Context, client *http.Client, endpoint, token string, projectId, mrIid, noteId int) ([]awardEmoji, error) {
	url := fmt.Sprintf("%s/projects/%d/merge_requests/%d/notes/%d/award_emoji?per_page=100",
		endpoint, projectId, mrIid, noteId)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Private-Token", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("GitLab API returned %d: %s", resp.StatusCode, string(body))
	}

	var emojis []awardEmoji
	if err := json.NewDecoder(resp.Body).Decode(&emojis); err != nil {
		return nil, fmt.Errorf("failed to decode award emoji response: %w", err)
	}
	return emojis, nil
}

// countReactionEmojis counts thumbsup and thumbsdown emojis from a list of award emojis.
func countReactionEmojis(emojis []awardEmoji) (thumbsUp, thumbsDown, total int) {
	total = len(emojis)
	for _, e := range emojis {
		switch e.Name {
		case "thumbsup":
			thumbsUp++
		case "thumbsdown":
			thumbsDown++
		}
	}
	return
}
