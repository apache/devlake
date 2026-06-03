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

// activitySummaryRecord is the JSON shape returned by /v1/organizations/analytics/summaries.
type activitySummaryRecord struct {
	StartingDate           string `json:"starting_date"`
	EndingDate             string `json:"ending_date"`
	DailyActiveUserCount   int    `json:"daily_active_user_count"`
	WeeklyActiveUserCount  int    `json:"weekly_active_user_count"`
	MonthlyActiveUserCount int    `json:"monthly_active_user_count"`
	AssignedSeatCount      int    `json:"assigned_seat_count"`
	PendingInviteCount     int    `json:"pending_invite_count"`
}

// ExtractActivitySummary parses raw activity summary records into tool-layer tables.
func ExtractActivitySummary(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*ClaudeCodeTaskData)
	if !ok {
		return errors.Default.New("task data is not ClaudeCodeTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	if strings.TrimSpace(connection.Organization) == "" {
		taskCtx.GetLogger().Info("No organization configured, skipping activity summary extraction")
		return nil
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:   taskCtx,
			Table: rawActivitySummaryTable,
			Options: claudeCodeRawParams{
				ConnectionId: data.Options.ConnectionId,
				ScopeId:      data.Options.ScopeId,
				Organization: connection.Organization,
				Endpoint:     "analytics/summaries",
			},
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			var record activitySummaryRecord
			if err := errors.Convert(json.Unmarshal(row.Data, &record)); err != nil {
				return nil, err
			}

			dateStr := strings.TrimSpace(record.StartingDate)
			if dateStr == "" {
				return nil, nil
			}
			t, parseErr := time.Parse("2006-01-02", dateStr)
			if parseErr != nil {
				return nil, errors.BadInput.Wrap(parseErr, "invalid starting_date in activity summary")
			}

			summary := &models.ClaudeCodeActivitySummary{
				ConnectionId:           data.Options.ConnectionId,
				ScopeId:                data.Options.ScopeId,
				Date:                   utcDate(t),
				DailyActiveUserCount:   record.DailyActiveUserCount,
				WeeklyActiveUserCount:  record.WeeklyActiveUserCount,
				MonthlyActiveUserCount: record.MonthlyActiveUserCount,
				AssignedSeatCount:      record.AssignedSeatCount,
				PendingInviteCount:     record.PendingInviteCount,
			}
			return []interface{}{summary}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
