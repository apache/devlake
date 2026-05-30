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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/claude_code/models"
)

// userActivityRecord is the JSON shape returned by /v1/organizations/analytics/users.
type userActivityRecord struct {
	User              userActivityUser `json:"user"`
	ChatMetrics       userActivityChat `json:"chat_metrics"`
	ClaudeCodeMetrics userActivityCC   `json:"claude_code_metrics"`
	WebSearchCount    int              `json:"web_search_count"`
}

type userActivityUser struct {
	Id           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

type userActivityChat struct {
	DistinctConversationCount     int `json:"distinct_conversation_count"`
	MessageCount                  int `json:"message_count"`
	DistinctProjectsCreatedCount  int `json:"distinct_projects_created_count"`
	DistinctProjectsUsedCount     int `json:"distinct_projects_used_count"`
	DistinctFilesUploadedCount    int `json:"distinct_files_uploaded_count"`
	DistinctArtifactsCreatedCount int `json:"distinct_artifacts_created_count"`
	ThinkingMessageCount          int `json:"thinking_message_count"`
	DistinctSkillsUsedCount       int `json:"distinct_skills_used_count"`
	ConnectorsUsedCount           int `json:"connectors_used_count"`
}

type userActivityCC struct {
	CoreMetrics userActivityCCCore  `json:"core_metrics"`
	ToolActions userActivityCCTools `json:"tool_actions"`
}

type userActivityCCCore struct {
	CommitCount          int               `json:"commit_count"`
	PullRequestCount     int               `json:"pull_request_count"`
	LinesOfCode          userActivityLines `json:"lines_of_code"`
	DistinctSessionCount int               `json:"distinct_session_count"`
}

type userActivityLines struct {
	AddedCount   int `json:"added_count"`
	RemovedCount int `json:"removed_count"`
}

type userActivityToolAction struct {
	AcceptedCount int `json:"accepted_count"`
	RejectedCount int `json:"rejected_count"`
}

type userActivityCCTools struct {
	EditTool         userActivityToolAction `json:"edit_tool"`
	MultiEditTool    userActivityToolAction `json:"multi_edit_tool"`
	WriteTool        userActivityToolAction `json:"write_tool"`
	NotebookEditTool userActivityToolAction `json:"notebook_edit_tool"`
}

// ExtractUserActivity parses raw user activity records into tool-layer tables.
func ExtractUserActivity(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*ClaudeCodeTaskData)
	if !ok {
		return errors.Default.New("task data is not ClaudeCodeTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	if strings.TrimSpace(connection.Organization) == "" {
		taskCtx.GetLogger().Info("No organization configured, skipping user activity extraction")
		return nil
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:   taskCtx,
			Table: rawUserActivityTable,
			Options: claudeCodeRawParams{
				ConnectionId: data.Options.ConnectionId,
				ScopeId:      data.Options.ScopeId,
				Organization: connection.Organization,
				Endpoint:     "analytics/users",
			},
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			var record userActivityRecord
			if err := errors.Convert(json.Unmarshal(row.Data, &record)); err != nil {
				return nil, err
			}

			date, parseErr := parseAnalyticsDate(row.Input)
			if parseErr != nil {
				return nil, parseErr
			}

			userId := strings.TrimSpace(record.User.Id)
			if userId == "" {
				userId = strings.TrimSpace(record.User.EmailAddress)
			}
			if userId == "" {
				return nil, nil
			}

			activity := &models.ClaudeCodeUserActivity{
				ConnectionId: data.Options.ConnectionId,
				ScopeId:      data.Options.ScopeId,
				Date:         date,
				UserId:       userId,
				UserEmail:    strings.TrimSpace(record.User.EmailAddress),

				ChatConversationCount:     record.ChatMetrics.DistinctConversationCount,
				ChatMessageCount:          record.ChatMetrics.MessageCount,
				ChatProjectsCreatedCount:  record.ChatMetrics.DistinctProjectsCreatedCount,
				ChatProjectsUsedCount:     record.ChatMetrics.DistinctProjectsUsedCount,
				ChatFilesUploadedCount:    record.ChatMetrics.DistinctFilesUploadedCount,
				ChatArtifactsCreatedCount: record.ChatMetrics.DistinctArtifactsCreatedCount,
				ChatThinkingMessageCount:  record.ChatMetrics.ThinkingMessageCount,
				ChatSkillsUsedCount:       record.ChatMetrics.DistinctSkillsUsedCount,
				ChatConnectorsUsedCount:   record.ChatMetrics.ConnectorsUsedCount,

				CCCommitCount:      record.ClaudeCodeMetrics.CoreMetrics.CommitCount,
				CCPullRequestCount: record.ClaudeCodeMetrics.CoreMetrics.PullRequestCount,
				CCLinesAdded:       record.ClaudeCodeMetrics.CoreMetrics.LinesOfCode.AddedCount,
				CCLinesRemoved:     record.ClaudeCodeMetrics.CoreMetrics.LinesOfCode.RemovedCount,
				CCSessionCount:     record.ClaudeCodeMetrics.CoreMetrics.DistinctSessionCount,

				EditToolAccepted:         record.ClaudeCodeMetrics.ToolActions.EditTool.AcceptedCount,
				EditToolRejected:         record.ClaudeCodeMetrics.ToolActions.EditTool.RejectedCount,
				MultiEditToolAccepted:    record.ClaudeCodeMetrics.ToolActions.MultiEditTool.AcceptedCount,
				MultiEditToolRejected:    record.ClaudeCodeMetrics.ToolActions.MultiEditTool.RejectedCount,
				WriteToolAccepted:        record.ClaudeCodeMetrics.ToolActions.WriteTool.AcceptedCount,
				WriteToolRejected:        record.ClaudeCodeMetrics.ToolActions.WriteTool.RejectedCount,
				NotebookEditToolAccepted: record.ClaudeCodeMetrics.ToolActions.NotebookEditTool.AcceptedCount,
				NotebookEditToolRejected: record.ClaudeCodeMetrics.ToolActions.NotebookEditTool.RejectedCount,

				WebSearchCount: record.WebSearchCount,
			}
			return []interface{}{activity}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

// parseAnalyticsDate extracts the date from the raw row input JSON.
// The input is the claudeCodeDayInput or claudeCodeDateRangeInput encoded as JSON.
func parseAnalyticsDate(rawInput json.RawMessage) (time.Time, errors.Error) {
	// Try day input first.
	var dayInput claudeCodeDayInput
	if err := json.Unmarshal(rawInput, &dayInput); err == nil && dayInput.Day != "" {
		t, parseErr := time.Parse("2006-01-02", strings.TrimSpace(dayInput.Day))
		if parseErr == nil {
			return utcDate(t), nil
		}
	}
	// Fall back to date range input (summaries).
	var rangeInput claudeCodeDateRangeInput
	if err := json.Unmarshal(rawInput, &rangeInput); err == nil && rangeInput.StartDate != "" {
		t, parseErr := time.Parse("2006-01-02", strings.TrimSpace(rangeInput.StartDate))
		if parseErr == nil {
			return utcDate(t), nil
		}
	}
	return time.Time{}, errors.BadInput.New("could not parse date from raw input")
}
