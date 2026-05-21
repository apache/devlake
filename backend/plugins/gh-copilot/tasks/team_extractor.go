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
	"github.com/apache/incubator-devlake/plugins/gh-copilot/models"
)

var ExtractTeamsMeta = plugin.SubTaskMeta{
	Name:             "extractTeams",
	EntryPoint:       ExtractTeams,
	EnabledByDefault: true,
	Description:      "Extract raw team data into tool layer table _tool_copilot_teams",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{rawCopilotTeamTable},
	ProductTables:    []string{models.GhCopilotTeam{}.TableName()},
}

// githubTeamParentResponse represents the parent field in the team API response.
type githubTeamParentResponse struct {
	Id   int    `json:"id"`
	Slug string `json:"slug"`
}

// githubTeamResponse represents a single team object from GET /orgs/{org}/teams.
type githubTeamResponse struct {
	Id                  int                       `json:"id"`
	Url                 string                    `json:"url"`
	HtmlUrl             string                    `json:"html_url"`
	Name                string                    `json:"name"`
	Type                string                    `json:"type"`
	Slug                string                    `json:"slug"`
	Description         string                    `json:"description"`
	Privacy             string                    `json:"privacy"`
	NotificationSetting string                    `json:"notification_setting"`
	Permission          string                    `json:"permission"`
	MembersUrl          string                    `json:"members_url"`
	RepositoriesUrl     string                    `json:"repositories_url"`
	Parent              *githubTeamParentResponse `json:"parent"`
	CreatedAt           *time.Time                `json:"created_at"`
	UpdatedAt           *time.Time                `json:"updated_at"`
	OrgId               *int                      `json:"organization_id"`
}

func ExtractTeams(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()
	org := strings.TrimSpace(connection.Organization)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:   taskCtx,
			Table: rawCopilotTeamTable,
			Options: copilotRawParams{
				ConnectionId: data.Options.ConnectionId,
				ScopeId:      data.Options.ScopeId,
				Organization: org,
				Endpoint:     connection.Endpoint,
			},
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			apiTeam := &githubTeamResponse{}
			if e := json.Unmarshal(row.Data, apiTeam); e != nil {
				return nil, errors.Convert(e)
			}

			team := &models.GhCopilotTeam{
				ConnectionId:        data.Options.ConnectionId,
				Id:                  apiTeam.Id,
				Name:                apiTeam.Name,
				Slug:                apiTeam.Slug,
				Description:         apiTeam.Description,
				Privacy:             apiTeam.Privacy,
				Permission:          apiTeam.Permission,
				NotificationSetting: apiTeam.NotificationSetting,
				GithubCreatedAt:     apiTeam.CreatedAt,
				GithubUpdatedAt:     apiTeam.UpdatedAt,
				OrgId:               apiTeam.OrgId,
				OrgLogin:            org,
			}
			if apiTeam.Parent != nil {
				team.ParentTeamId = &apiTeam.Parent.Id
				team.ParentTeamSlug = apiTeam.Parent.Slug
			}
			return []interface{}{team}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
