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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

const rawTaskTable = "asana_tasks"

var _ plugin.SubTaskEntryPoint = CollectTask

var CollectTaskMeta = plugin.SubTaskMeta{
	Name:             "CollectTask",
	EntryPoint:       CollectTask,
	EnabledByDefault: true,
	Description:      "Collect task data from Asana API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type asanaTaskListResponse struct {
	Data     []json.RawMessage `json:"data"`
	NextPage *struct {
		Offset string `json:"offset"`
		Path   string `json:"path"`
		URI    string `json:"uri"`
	} `json:"next_page"`
}

func CollectTask(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AsanaTaskData)
	collector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: rawTaskTable,
		},
		ApiClient:   data.ApiClient,
		PageSize:    100,
		UrlTemplate: "projects/{{ .Params.ProjectId }}/tasks",
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("limit", "100")
			// Request all fields needed for transformation including section name
			query.Set("opt_fields", "gid,name,notes,resource_type,resource_subtype,completed,completed_at,due_on,created_at,modified_at,permalink_url,assignee,assignee.name,created_by,created_by.name,parent,num_subtasks,memberships.section,memberships.section.name,memberships.project")
			if reqData.CustomData != nil {
				if offset, ok := reqData.CustomData.(string); ok && offset != "" {
					query.Set("offset", offset)
				}
			}
			return query, nil
		},
		GetNextPageCustomData: func(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
			var resp asanaTaskListResponse
			err := api.UnmarshalResponse(prevPageResponse, &resp)
			if err != nil {
				return nil, err
			}
			if resp.NextPage != nil && resp.NextPage.Offset != "" {
				return resp.NextPage.Offset, nil
			}
			return nil, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var w asanaTaskListResponse
			err := api.UnmarshalResponse(res, &w)
			if err != nil {
				return nil, err
			}
			return w.Data, nil
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
