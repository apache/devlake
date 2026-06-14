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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/gh-copilot/models"
)

// userTeamRecord represents a single line from the user-teams-1-day JSONL report.
type userTeamRecord struct {
	Day            string `json:"day"`
	UserId         int64  `json:"user_id"`
	UserLogin      string `json:"user_login"`
	OrganizationId string `json:"organization_id"`
	EnterpriseId   string `json:"enterprise_id"`
	TeamId         int64  `json:"team_id"`
	Slug           string `json:"slug"`
}

// ExtractUserTeams parses user-team JSONL records into the GhCopilotUserTeam model.
func ExtractUserTeams(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	params := copilotRawParams{
		ConnectionId: data.Options.ConnectionId,
		ScopeId:      data.Options.ScopeId,
		Organization: connection.Organization,
		Endpoint:     connection.Endpoint,
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Table:   rawUserTeamsTable,
			Options: params,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			var rec userTeamRecord
			if err := errors.Convert(json.Unmarshal(row.Data, &rec)); err != nil {
				return nil, err
			}

			day, parseErr := time.Parse("2006-01-02", rec.Day)
			if parseErr != nil {
				return nil, errors.BadInput.Wrap(parseErr, "invalid day in user-teams report")
			}

			return []interface{}{
				&models.GhCopilotUserTeam{
					ConnectionId:   data.Options.ConnectionId,
					ScopeId:        data.Options.ScopeId,
					Day:            day,
					UserId:         rec.UserId,
					TeamId:         rec.TeamId,
					UserLogin:      rec.UserLogin,
					OrganizationId: rec.OrganizationId,
					EnterpriseId:   rec.EnterpriseId,
					TeamSlug:       rec.Slug,
				},
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
