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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/gh-copilot/models"
)

var ExtractTeamUsersMeta = plugin.SubTaskMeta{
	Name:             "extractTeamUsers",
	EntryPoint:       ExtractTeamUsers,
	EnabledByDefault: true,
	Description:      "Extract raw team member data into tool layer table _tool_copilot_team_users",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{rawCopilotTeamUserTable},
	ProductTables:    []string{models.GhCopilotTeamUser{}.TableName()},
}

// githubTeamUserResponse represents a user object from the team members API.
type githubTeamUserResponse struct {
	Id          int    `json:"id"`
	Login       string `json:"login"`
	NodeId      string `json:"node_id"`
	Type        string `json:"type"`
	ViewType    string `json:"view_type"`
	IsSiteAdmin bool   `json:"site_admin"`
}

func ExtractTeamUsers(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()
	org := strings.TrimSpace(connection.Organization)
	if org == "" {
		return errors.BadInput.New("no organization configured on connection")
	}

	db := taskCtx.GetDal()
	// Delete existing team-user records for this connection/org before re-extracting.
	if dErr := db.Delete(
		&models.GhCopilotTeamUser{},
		dal.Where("connection_id = ? AND org_login = ?", data.Options.ConnectionId, org),
	); dErr != nil {
		return dErr
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:   taskCtx,
			Table: rawCopilotTeamUserTable,
			Options: copilotRawParams{
				ConnectionId: data.Options.ConnectionId,
				ScopeId:      data.Options.ScopeId,
				Organization: org,
				Endpoint:     connection.Endpoint,
			},
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			apiUsers := &[]githubTeamUserResponse{}
			if e := json.Unmarshal(row.Data, apiUsers); e != nil {
				return nil, errors.Convert(e)
			}
			team := &simpleCopilotTeam{}
			if e := json.Unmarshal(row.Input, team); e != nil {
				return nil, errors.Convert(e)
			}

			results := make([]interface{}, 0, len(*apiUsers))
			for _, u := range *apiUsers {
				results = append(results, &models.GhCopilotTeamUser{
					ConnectionId: data.Options.ConnectionId,
					TeamId:       team.Id,
					UserId:       u.Id,
					OrgLogin:     team.OrgLogin,
					TeamSlug:     team.Slug,
					UserLogin:    u.Login,
					Type:         u.Type,
					ViewType:     u.ViewType,
					IsSiteAdmin:  u.IsSiteAdmin,
				})
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
