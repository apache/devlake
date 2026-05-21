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
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const rawCopilotTeamTable = "copilot_api_teams"

var CollectTeamsMeta = plugin.SubTaskMeta{
	Name:             "collectTeams",
	EntryPoint:       CollectTeams,
	EnabledByDefault: true,
	Description:      "Collect teams data from GitHub API for the configured organization.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{},
	ProductTables:    []string{rawCopilotTeamTable},
}

func CollectTeams(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	org := strings.TrimSpace(connection.Organization)
	if org == "" {
		taskCtx.GetLogger().Warn(nil, "skipping team collection: no organization configured on connection %d", connection.ID)
		return nil
	}

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), connection)
	if err != nil {
		return err
	}

	collector, cErr := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx:   taskCtx,
		Table: rawCopilotTeamTable,
		Options: copilotRawParams{
			ConnectionId: data.Options.ConnectionId,
			ScopeId:      data.Options.ScopeId,
			Organization: org,
			Endpoint:     connection.Endpoint,
		},
	})
	if cErr != nil {
		return cErr
	}

	cErr = collector.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   apiClient,
		PageSize:    100,
		UrlTemplate: fmt.Sprintf("orgs/%s/teams", org),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("page", fmt.Sprintf("%v", reqData.Pager.Page))
			query.Set("per_page", fmt.Sprintf("%v", reqData.Pager.Size))
			return query, nil
		},
		GetTotalPages: getTotalPagesFromResponse,
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var items []json.RawMessage
			e := helper.UnmarshalResponse(res, &items)
			if e != nil {
				return nil, e
			}
			return items, nil
		},
		AfterResponse: ignore404,
	})
	if cErr != nil {
		return cErr
	}

	return collector.Execute()
}
