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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/jira/models"
)

const staleBoardIssueCheckBatchSize = 100

var _ plugin.SubTaskEntryPoint = CleanupStaleBoardIssues

var CleanupStaleBoardIssuesMeta = plugin.SubTaskMeta{
	Name:             "cleanupStaleBoardIssues",
	EntryPoint:       CleanupStaleBoardIssues,
	EnabledByDefault: true,
	Description:      "remove board associations for issues no longer returned by the board API (e.g. moved to a different board/team)",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

// CleanupStaleBoardIssues batch-checks all issues in _tool_jira_board_issues against the
// board API (100 per request via issue IN (...) JQL) and removes any that are no longer
// returned. Covers both open and closed issues that moved to a different board or team.
func CleanupStaleBoardIssues(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*JiraTaskData)
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	connectionId := data.Options.ConnectionId
	boardId := data.Options.BoardId

	logger.Info("cleaning up stale board issues for connection_id=%d, board_id=%d", connectionId, boardId)

	issueIdGen := didgen.NewDomainIdGenerator(&models.JiraIssue{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.JiraBoard{})

	var allBoardIssues []struct {
		IssueKey string
		IssueId  uint64
	}
	if err := db.All(&allBoardIssues,
		dal.Select("ji.issue_key, ji.issue_id"),
		dal.From("_tool_jira_board_issues tbi"),
		dal.Join("JOIN _tool_jira_issues ji ON ji.issue_id = tbi.issue_id AND ji.connection_id = tbi.connection_id"),
		dal.Where("tbi.connection_id = ? AND tbi.board_id = ?", connectionId, boardId),
	); err != nil {
		return errors.Default.Wrap(err, "failed to query board issues")
	}

	logger.Info("batch-checking %d board issues against the board API", len(allBoardIssues))

	onBoard, err := fetchBoardMembership(data, boardId, allBoardIssues)
	if err != nil {
		return err
	}
	if onBoard == nil {
		logger.Warn(nil, "board %d returned 404 from the board issues API, skipping stale cleanup to preserve existing associations", boardId)
		return nil
	}

	removed := 0
	for _, bi := range allBoardIssues {
		if onBoard[bi.IssueKey] {
			continue
		}

		logger.Info("issue %s is no longer on board %d, updating state and removing association", bi.IssueKey, boardId)

		// Re-fetch from Jira to update _tool_jira_issues with current status/team/resolution
		if updateErr := collectAndExtractSingleIssue(taskCtx, data, db, bi.IssueKey); updateErr != nil {
			logger.Warn(updateErr, "failed to update issue state for %s, will still remove board association", bi.IssueKey)
		}

		if delErr := removeBoardAssociation(db, issueIdGen, boardIdGen, connectionId, boardId, bi.IssueId); delErr != nil {
			logger.Warn(delErr, "failed to remove stale association for %s, will retry next run", bi.IssueKey)
			continue
		}

		removed++
	}

	logger.Info("removed %d stale board associations out of %d checked", removed, len(allBoardIssues))
	return nil
}

func removeBoardAssociation(db dal.Dal, issueIdGen, boardIdGen *didgen.DomainIdGenerator, connectionId, boardId, issueId uint64) errors.Error {
	domainIssueId := issueIdGen.Generate(connectionId, issueId)
	domainBoardId := boardIdGen.Generate(connectionId, boardId)

	tx := db.Begin()
	txErr := tx.Delete(
		&models.JiraBoardIssue{},
		dal.Where("connection_id = ? AND board_id = ? AND issue_id = ?", connectionId, boardId, issueId),
	)
	if txErr == nil {
		txErr = tx.Delete(
			&ticket.BoardIssue{},
			dal.Where("board_id = ? AND issue_id = ?", domainBoardId, domainIssueId),
		)
	}
	if txErr != nil {
		_ = tx.Rollback()
		return txErr
	}
	return tx.Commit()
}

// fetchBoardMembership batch-checks which issue keys are still on the board using
// issue IN (...) JQL — 100 issues per API call instead of one call per issue.
// Returns (nil, nil) when the board is not found (404), signalling the caller to skip cleanup.
// Returns an error when a page is empty while total still claims more results, so the
// caller does not treat a partial/empty map as "not on board".
func fetchBoardMembership(data *JiraTaskData, boardId uint64, issues []struct {
	IssueKey string
	IssueId  uint64
}) (map[string]bool, errors.Error) {
	onBoard := make(map[string]bool, len(issues))

	for i := 0; i < len(issues); i += staleBoardIssueCheckBatchSize {
		end := i + staleBoardIssueCheckBatchSize
		if end > len(issues) {
			end = len(issues)
		}
		batch := issues[i:end]

		keys := make([]string, 0, len(batch))
		for _, bi := range batch {
			keys = append(keys, bi.IssueKey)
		}

		path := fmt.Sprintf("agile/1.0/board/%d/issue", boardId)
		jql := fmt.Sprintf("issue IN (%s)", strings.Join(keys, ","))

		pageSize := 0
		for startAt := 0; ; startAt += pageSize {
			query := url.Values{}
			query.Set("jql", jql)
			query.Set("startAt", fmt.Sprintf("%d", startAt))
			query.Set("maxResults", fmt.Sprintf("%d", staleBoardIssueCheckBatchSize))
			query.Set("fields", "id,key")

			resp, err := data.ApiClient.Get(path, query, nil)
			if err != nil {
				return nil, errors.Default.Wrap(err, "failed to batch-check board membership")
			}

			if resp.StatusCode == http.StatusNotFound {
				_ = resp.Body.Close()
				return nil, nil
			}

			if resp.StatusCode != http.StatusOK {
				_ = resp.Body.Close()
				return nil, errors.Default.New(fmt.Sprintf("unexpected status %d from board issues API", resp.StatusCode))
			}

			blob, readErr := errors.Convert01(io.ReadAll(resp.Body))
			_ = resp.Body.Close()
			if readErr != nil {
				return nil, errors.Default.Wrap(readErr, "failed to read board response")
			}

			var result struct {
				Total  int `json:"total"`
				Issues []struct {
					Key string `json:"key"`
				} `json:"issues"`
			}
			if jsonErr := errors.Convert(json.Unmarshal(blob, &result)); jsonErr != nil {
				return nil, errors.Default.Wrap(jsonErr, "failed to parse board response")
			}

			if len(result.Issues) == 0 {
				if result.Total > startAt {
					return nil, errors.Default.New(fmt.Sprintf(
						"incomplete board membership response for board %d: total=%d but issues=[] at startAt=%d",
						boardId, result.Total, startAt,
					))
				}
				break
			}

			for _, issue := range result.Issues {
				onBoard[issue.Key] = true
			}

			pageSize = len(result.Issues)
			if startAt+pageSize >= result.Total {
				break
			}
		}
	}

	return onBoard, nil
}
