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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/claude_code/models"
)

// connectorUsageRecord is the JSON shape returned by /v1/organizations/analytics/connectors.
type connectorUsageRecord struct {
	ConnectorName     string             `json:"connector_name"`
	DistinctUserCount int                `json:"distinct_user_count"`
	ChatMetrics       connectorUsageChat `json:"chat_metrics"`
	ClaudeCodeMetrics connectorUsageCC   `json:"claude_code_metrics"`
}

type connectorUsageChat struct {
	DistinctConversationConnectorUsedCount int `json:"distinct_conversation_connector_used_count"`
}

type connectorUsageCC struct {
	DistinctSessionConnectorUsedCount int `json:"distinct_session_connector_used_count"`
}

// ExtractConnectorUsage parses raw connector usage records into tool-layer tables.
func ExtractConnectorUsage(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*ClaudeCodeTaskData)
	if !ok {
		return errors.Default.New("task data is not ClaudeCodeTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	if strings.TrimSpace(connection.Organization) == "" {
		taskCtx.GetLogger().Info("No organization configured, skipping connector usage extraction")
		return nil
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:   taskCtx,
			Table: rawConnectorUsageTable,
			Options: claudeCodeRawParams{
				ConnectionId: data.Options.ConnectionId,
				ScopeId:      data.Options.ScopeId,
				Organization: connection.Organization,
				Endpoint:     "analytics/connectors",
			},
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			var record connectorUsageRecord
			if err := errors.Convert(json.Unmarshal(row.Data, &record)); err != nil {
				return nil, err
			}

			date, parseErr := parseAnalyticsDate(row.Input)
			if parseErr != nil {
				return nil, parseErr
			}

			connectorName := strings.TrimSpace(record.ConnectorName)
			if connectorName == "" {
				return nil, nil
			}

			connector := &models.ClaudeCodeConnectorUsage{
				ConnectionId:          data.Options.ConnectionId,
				ScopeId:               data.Options.ScopeId,
				Date:                  date,
				ConnectorName:         connectorName,
				DistinctUserCount:     record.DistinctUserCount,
				ChatConversationCount: record.ChatMetrics.DistinctConversationConnectorUsedCount,
				CCSessionCount:        record.ClaudeCodeMetrics.DistinctSessionConnectorUsedCount,
			}
			return []interface{}{connector}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
