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
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/cursor/models"
)

type aiCodeCommitRecord struct {
	CommitHash          string  `json:"commitHash"`
	UserId              string  `json:"userId"`
	UserEmail           string  `json:"userEmail"`
	RepoName            string  `json:"repoName"`
	BranchName          string  `json:"branchName"`
	IsPrimaryBranch     *bool   `json:"isPrimaryBranch"`
	CommitSource        string  `json:"commitSource"`
	TotalLinesAdded     int     `json:"totalLinesAdded"`
	TotalLinesDeleted   int     `json:"totalLinesDeleted"`
	TabLinesAdded       int     `json:"tabLinesAdded"`
	TabLinesDeleted     int     `json:"tabLinesDeleted"`
	ComposerLinesAdded  int     `json:"composerLinesAdded"`
	ComposerLinesDeleted int    `json:"composerLinesDeleted"`
	NonAiLinesAdded     *int    `json:"nonAiLinesAdded"`
	NonAiLinesDeleted   *int    `json:"nonAiLinesDeleted"`
	CommitTs            string  `json:"commitTs"`
	CreatedAt           string  `json:"createdAt"`
}

// ExtractAiCodeCommits parses raw AI code commit records into tool-layer tables.
func ExtractAiCodeCommits(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if data.Connection.KeyTier != models.KeyTierEnterprise {
		taskCtx.GetLogger().Info("Skipping AI code commit extraction: key tier is %q, not enterprise", data.Connection.KeyTier)
		return nil
	}

	extractor, err := newCursorStatefulExtractor(&cursorStatefulExtractorArgs[aiCodeCommitRecord]{
		SubtaskCommonArgs: cursorSubtaskCommonArgs(taskCtx, data, rawAiCodeCommitsTable),
		ConnectionId:      data.Options.ConnectionId,
		ScopeId:           data.Options.ScopeId,
		ToolTable:         models.CursorAiCodeCommit{}.TableName(),
		Extract: func(record *aiCodeCommitRecord, row *helper.RawData) ([]any, errors.Error) {
			commitHash := strings.TrimSpace(record.CommitHash)
			if commitHash == "" {
				return nil, nil
			}

			commit := &models.CursorAiCodeCommit{
				ConnectionId:        data.Options.ConnectionId,
				ScopeId:             data.Options.ScopeId,
				CommitHash:          commitHash,
				UserId:              strings.TrimSpace(record.UserId),
				UserEmail:           strings.TrimSpace(record.UserEmail),
				RepoName:            strings.TrimSpace(record.RepoName),
				BranchName:          strings.TrimSpace(record.BranchName),
				IsPrimaryBranch:     record.IsPrimaryBranch,
				CommitSource:        strings.TrimSpace(record.CommitSource),
				TotalLinesAdded:     record.TotalLinesAdded,
				TotalLinesDeleted:   record.TotalLinesDeleted,
				TabLinesAdded:       record.TabLinesAdded,
				TabLinesDeleted:     record.TabLinesDeleted,
				ComposerLinesAdded:  record.ComposerLinesAdded,
				ComposerLinesDeleted: record.ComposerLinesDeleted,
				NonAiLinesAdded:     record.NonAiLinesAdded,
				NonAiLinesDeleted:   record.NonAiLinesDeleted,
			}
			if ts := parseOptionalISOTime(record.CommitTs); ts != nil {
				commit.CommitTs = ts
			}
			if ts := parseOptionalISOTime(record.CreatedAt); ts != nil {
				commit.IngestedAt = ts
			}
			return []any{commit}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func parseOptionalISOTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05.000000",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}
