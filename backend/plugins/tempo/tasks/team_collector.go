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
	"time"

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

	apiCollector, err := api.NewStatefulApiCollector(api.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: models.TempoApiParams{
			ConnectionId: data.Options.ConnectionId,
		},
		Table: RAW_TEAM_TABLE,
	})
	if err != nil {
		return err
	}

	err = apiCollector.InitCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.TempoApiParams{
				ConnectionId: data.Options.ConnectionId,
			},
			Table: RAW_TEAM_TABLE,
		},
		ApiClient:   data.ApiClient,
		UrlTemplate: "teams",
		PageSize:    50,
		// No GetTotalPages: Tempo v4 pagination metadata (PageableMetadata)
		// has no total, so pages are fetched until one comes back short.
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			pager := reqData.Pager
			if pager == nil {
				pager = &api.Pager{Page: 1, Skip: 0, Size: 50}
			}
			query.Set("offset", strconv.Itoa(pager.Skip))
			query.Set("limit", strconv.Itoa(pager.Size))

			if apiCollector.IsIncremental() && apiCollector.GetSince() != nil {
				since := apiCollector.GetSince()
				query.Set("updatedFrom", since.Format(time.RFC3339))
			}

			return query, nil
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
