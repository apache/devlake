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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const RAW_EPIC_TABLE = "taiga_api_epics"

var _ plugin.SubTaskEntryPoint = CollectEpics

var CollectEpicsMeta = plugin.SubTaskMeta{
	Name:             "collectEpics",
	EntryPoint:       CollectEpics,
	EnabledByDefault: true,
	Description:      "collect Taiga epics",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func CollectEpics(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TaigaTaskData)
	logger := taskCtx.GetLogger()
	logger.Info("collect epics")

	collector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: TaigaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_EPIC_TABLE,
		},
		ApiClient:   data.ApiClient,
		PageSize:    1000,
		UrlTemplate: "epics",
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("project", fmt.Sprintf("%d", data.Options.ProjectId))
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var result []json.RawMessage
			err := api.UnmarshalResponse(res, &result)
			if err != nil {
				return nil, err
			}
			return result, nil
		},
	})
	if err != nil {
		logger.Error(err, "collect epics error")
		return err
	}
	return collector.Execute()
}
