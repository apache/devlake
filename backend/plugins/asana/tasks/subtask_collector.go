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
	"reflect"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

const rawSubtaskTable = "asana_subtasks"

var _ plugin.SubTaskEntryPoint = CollectSubtask

var CollectSubtaskMeta = plugin.SubTaskMeta{
	Name:             "CollectSubtask",
	EntryPoint:       CollectSubtask,
	EnabledByDefault: true,
	Description:      "Collect subtask data from Asana API for each task",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func CollectSubtask(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AsanaTaskData)
	db := taskCtx.GetDal()

	// Get all tasks that have subtasks
	clauses := []dal.Clause{
		dal.Select("gid"),
		dal.From(&models.AsanaTask{}),
		dal.Where("connection_id = ? AND project_gid = ? AND num_subtasks > 0",
			data.Options.ConnectionId, data.Options.ProjectId),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}

	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(simpleTask{}))
	if err != nil {
		return err
	}

	collector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: rawSubtaskTable,
		},
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: "tasks/{{ .Input.Gid }}/subtasks",
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("opt_fields", "gid,name,notes,resource_type,resource_subtype,completed,completed_at,due_on,created_at,modified_at,permalink_url,assignee,created_by,parent,num_subtasks,memberships.section,memberships.project")
			query.Set("limit", "100")
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var resp asanaListResponse
			err := api.UnmarshalResponse(res, &resp)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
