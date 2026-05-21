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
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/gh-copilot/models"
)

const rawCopilotTeamUserTable = "copilot_api_team_users"

// simpleCopilotTeam is the minimal struct used as input for the team-member
// collector iterator. Fields must be exported for DalCursorIterator.
type simpleCopilotTeam struct {
	Id       int
	Slug     string
	OrgLogin string
}

var CollectTeamUsersMeta = plugin.SubTaskMeta{
	Name:             "collectTeamUsers",
	EntryPoint:       CollectTeamUsers,
	EnabledByDefault: true,
	Description:      "Collect team members data from GitHub API.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{models.GhCopilotTeam{}.TableName()},
	ProductTables:    []string{rawCopilotTeamUserTable},
}

func CollectTeamUsers(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	org := strings.TrimSpace(connection.Organization)
	if org == "" {
		taskCtx.GetLogger().Warn(nil, "skipping team-user collection: no organization configured on connection %d", connection.ID)
		return nil
	}

	db := taskCtx.GetDal()
	cursor, err := db.Cursor(
		dal.Select("id, slug, org_login"),
		dal.From(models.GhCopilotTeam{}.TableName()),
		dal.Where("connection_id = ? AND org_login = ?", data.Options.ConnectionId, org),
	)
	if err != nil {
		return err
	}
	iterator, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(simpleCopilotTeam{}))
	if err != nil {
		return err
	}

	apiClient, aErr := CreateApiClient(taskCtx.TaskContext(), connection)
	if aErr != nil {
		return aErr
	}

	collector, cErr := helper.NewApiCollector(helper.ApiCollectorArgs{
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
		ApiClient:   apiClient,
		Input:       iterator,
		PageSize:    100,
		UrlTemplate: "orgs/{{ .Input.OrgLogin }}/teams/{{ .Input.Slug }}/members",
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("page", fmt.Sprintf("%v", reqData.Pager.Page))
			query.Set("per_page", fmt.Sprintf("%v", reqData.Pager.Size))
			return query, nil
		},
		GetTotalPages: getTotalPagesFromResponse,
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var item json.RawMessage
			e := helper.UnmarshalResponse(res, &item)
			if e != nil {
				return nil, e
			}
			return []json.RawMessage{item}, nil
		},
		AfterResponse: ignore404,
	})
	if cErr != nil {
		return cErr
	}

	return collector.Execute()
}
