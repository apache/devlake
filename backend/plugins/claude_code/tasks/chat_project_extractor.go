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

// chatProjectRecord is the JSON shape returned by /v1/organizations/analytics/apps/chat/projects.
type chatProjectRecord struct {
	ProjectName               string             `json:"project_name"`
	ProjectId                 string             `json:"project_id"`
	DistinctUserCount         int                `json:"distinct_user_count"`
	DistinctConversationCount int                `json:"distinct_conversation_count"`
	MessageCount              int                `json:"message_count"`
	CreatedAt                 string             `json:"created_at"`
	CreatedBy                 chatProjectCreator `json:"created_by"`
}

type chatProjectCreator struct {
	Id           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

// ExtractChatProjects parses raw chat project records into tool-layer tables.
func ExtractChatProjects(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*ClaudeCodeTaskData)
	if !ok {
		return errors.Default.New("task data is not ClaudeCodeTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	if strings.TrimSpace(connection.Organization) == "" {
		taskCtx.GetLogger().Info("No organization configured, skipping chat project extraction")
		return nil
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:   taskCtx,
			Table: rawChatProjectTable,
			Options: claudeCodeRawParams{
				ConnectionId: data.Options.ConnectionId,
				ScopeId:      data.Options.ScopeId,
				Organization: connection.Organization,
				Endpoint:     "analytics/apps/chat/projects",
			},
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			var record chatProjectRecord
			if err := errors.Convert(json.Unmarshal(row.Data, &record)); err != nil {
				return nil, err
			}

			date, parseErr := parseAnalyticsDate(row.Input)
			if parseErr != nil {
				return nil, parseErr
			}

			projectId := strings.TrimSpace(record.ProjectId)
			if projectId == "" {
				return nil, nil
			}

			var createdAt time.Time
			if ts := strings.TrimSpace(record.CreatedAt); ts != "" {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					createdAt = t.UTC()
				}
			}

			project := &models.ClaudeCodeChatProject{
				ConnectionId:      data.Options.ConnectionId,
				ScopeId:           data.Options.ScopeId,
				Date:              date,
				ProjectId:         projectId,
				ProjectName:       strings.TrimSpace(record.ProjectName),
				DistinctUserCount: record.DistinctUserCount,
				ConversationCount: record.DistinctConversationCount,
				MessageCount:      record.MessageCount,
				CreatedAt:         createdAt,
				CreatedById:       strings.TrimSpace(record.CreatedBy.Id),
				CreatedByEmail:    strings.TrimSpace(record.CreatedBy.EmailAddress),
			}
			return []interface{}{project}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
