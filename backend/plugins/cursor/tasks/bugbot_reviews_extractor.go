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
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

type bugbotFindingRecord struct {
	CommentId        *string         `json:"comment_id"`
	ResolutionStatus *string         `json:"resolution_status"`
	Severity         *string         `json:"severity"`
	Title            *string         `json:"title"`
	Description      *string         `json:"description"`
	Locations        json.RawMessage `json:"locations"`
}

type bugbotReviewRecord struct {
	RequestId         string              `json:"request_id"`
	Timestamp         string              `json:"timestamp"`
	Repo              string              `json:"repo"`
	RepoNodeId        *string             `json:"repo_node_id"`
	PrNumber          *int                `json:"pr_number"`
	CommitSha         *string             `json:"commit_sha"`
	BugsFound         int                 `json:"bugs_found"`
	CostCents         *float64            `json:"cost_cents"`
	DryRun            bool                `json:"dry_run"`
	PublicationStatus string              `json:"publication_status"`
	Bugs              []bugbotFindingRecord `json:"bugs"`
}

// ExtractBugbotReviews parses raw BugBot review records into review and finding tool tables.
func ExtractBugbotReviews(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if !data.Connection.HasBugbotReviews {
		taskCtx.GetLogger().Info("Skipping BugBot review extraction: connection lacks bugbot-reviews access")
		return nil
	}

	extractor, err := newCursorStatefulExtractor(&cursorStatefulExtractorArgs[bugbotReviewRecord]{
		SubtaskCommonArgs: cursorSubtaskCommonArgs(taskCtx, data, rawBugbotReviewsTable),
		ConnectionId:      data.Options.ConnectionId,
		ScopeId:           data.Options.ScopeId,
		ToolTable:         models.CursorBugBotReview{}.TableName(),
		Extract: func(record *bugbotReviewRecord, row *helper.RawData) ([]any, errors.Error) {
			requestId := strings.TrimSpace(record.RequestId)
			if requestId == "" {
				return nil, nil
			}

			review := &models.CursorBugBotReview{
				ConnectionId:      data.Options.ConnectionId,
				ScopeId:           data.Options.ScopeId,
				RequestId:         requestId,
				Repo:              strings.TrimSpace(record.Repo),
				PrNumber:          record.PrNumber,
				BugsFound:         record.BugsFound,
				CostCents:         record.CostCents,
				DryRun:            record.DryRun,
				PublicationStatus: strings.TrimSpace(record.PublicationStatus),
			}
			if ts := parseOptionalISOTime(record.Timestamp); ts != nil {
				review.Timestamp = ts
			}
			if record.RepoNodeId != nil {
				review.RepoNodeId = strings.TrimSpace(*record.RepoNodeId)
			}
			if record.CommitSha != nil {
				review.CommitSha = strings.TrimSpace(*record.CommitSha)
			}

			results := []any{review}
			for i, bug := range record.Bugs {
				finding := &models.CursorBugBotFinding{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					RequestId:    requestId,
					FindingIndex: i,
				}
				if bug.CommentId != nil {
					finding.CommentId = strings.TrimSpace(*bug.CommentId)
				}
				if bug.ResolutionStatus != nil {
					finding.ResolutionStatus = strings.TrimSpace(*bug.ResolutionStatus)
				}
				if bug.Severity != nil {
					finding.Severity = strings.TrimSpace(*bug.Severity)
				}
				if bug.Title != nil {
					finding.Title = strings.TrimSpace(*bug.Title)
				}
				if bug.Description != nil {
					finding.Description = strings.TrimSpace(*bug.Description)
				}
				if len(bug.Locations) > 0 && string(bug.Locations) != "null" {
					finding.Locations = string(bug.Locations)
				}
				results = append(results, finding)
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
