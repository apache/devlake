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
	"net/http"
	"net/url"
	"strconv"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/tempo/models"
)

var CollectTeamsMeta = plugin.SubTaskMeta{
	Name:             "collect-teams",
	Description:      "Collect teams from Tempo API",
	EntryPoint:       CollectTeams,
	EnabledByDefault: true,
	Dependencies:     nil,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func CollectTeams(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TempoTaskData)

	// Not a stateful (incremental) collector: GET /4/teams has no
	// "updatedFrom" filter, so an incremental run would append the same
	// teams to the raw table again instead of refreshing them.
	apiCollector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: teamRawDataSubTaskArgs(taskCtx, data.Options),
		ApiClient:          data.ApiClient,
		UrlTemplate:        "teams",
		PageSize:           50,
		// No GetTotalPages: Tempo v4 pagination metadata (PageableMetadata)
		// has no total, so pages are fetched until one comes back short.
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			return buildTeamQuery(data.Options, reqData.Pager), nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var response struct {
				Results []json.RawMessage `json:"results"`
			}
			err := api.UnmarshalResponse(res, &response)
			if err != nil {
				return nil, err
			}
			return response.Results, nil
		},
	})

	if err != nil {
		return err
	}

	return apiCollector.Execute()
}

// teamRawDataSubTaskArgs scopes the raw team data and the extracted teams per
// team, like the scope itself (TempoTeam.ScopeParams). The extracted rows are
// the team scopes: with connection-wide params, extracting one team scope
// deleted and re-created every team scope of the connection.
func teamRawDataSubTaskArgs(taskCtx plugin.SubTaskContext, opts *TempoOptions) api.RawDataSubTaskArgs {
	return api.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: models.TempoApiParams{
			ConnectionId: opts.ConnectionId,
			TeamId:       opts.TeamId,
		},
		Table: RAW_TEAM_TABLE,
	}
}

func buildTeamQuery(opts *TempoOptions, pager *api.Pager) url.Values {
	if pager == nil {
		pager = &api.Pager{Page: 1, Skip: 0, Size: 50}
	}
	query := url.Values{}
	query.Set("offset", strconv.Itoa(pager.Skip))
	query.Set("limit", strconv.Itoa(pager.Size))
	if opts.TeamId != 0 {
		// only the team of this scope, not every team of the Tempo instance
		query.Set("teamIds", strconv.FormatInt(opts.TeamId, 10))
	}
	return query
}
