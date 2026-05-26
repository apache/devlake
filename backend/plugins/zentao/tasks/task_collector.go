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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/zentao/models"
)

const RAW_TASK_TABLE = "zentao_api_tasks"

var _ plugin.SubTaskEntryPoint = CollectTask

func CollectTask(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*ZentaoTaskData)

	cursor, err := taskCtx.GetDal().Cursor(
		dal.Select(`id`),
		dal.From(&models.ZentaoExecution{}),
		dal.Where(`project_id = ? and connection_id = ?`, data.Options.ProjectId, data.Options.ConnectionId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	iterator, err := api.NewDalCursorIterator(taskCtx.GetDal(), cursor, reflect.TypeOf(input{}))
	if err != nil {
		return err
	}

	collector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_TASK_TABLE,
		},
		Input:       iterator,
		ApiClient:   data.ApiClient,
		PageSize:    100,
		UrlTemplate: "/executions/{{ .Input.Id }}/tasks",
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("page", fmt.Sprintf("%v", reqData.Pager.Page))
			query.Set("limit", fmt.Sprintf("%v", reqData.Pager.Size))
			query.Set("status", "all")
			return query, nil
		},
		GetTotalPages: GetTotalPagesFromResponse,
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var data struct {
				Tasks []models.ZentaoTaskRes `json:"tasks"`
			}
			err := api.UnmarshalResponse(res, &data)
			if errors.Is(err, api.ErrEmptyResponse) {
				return nil, nil
			}
			if err != nil {
				return nil, errors.Default.Wrap(err, "error reading endpoint response by Zentao bug collector")
			}

			allTaskRecords := make(map[int64]models.ZentaoTaskRes)
			for _, task := range data.Tasks {
				// extract task's children
				childTasks, err := extractChildrenWithDFS(task)
				if err != nil {
					return nil, errors.Default.New(fmt.Sprintf("extract task: %v children err: %v", task, err))
				}
				for _, task := range childTasks {
					allTaskRecords[task.Id] = task
				}
			}
			var allTask []json.RawMessage
			for _, task := range allTaskRecords {
				taskRawJsonMessage, err := task.ToJsonRawMessage()
				if err != nil {
					return nil, errors.Default.New(err.Error())
				}
				allTask = append(allTask, taskRawJsonMessage)
			}
			return allTask, nil
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

// extractChildrenWithDFS return task's child tasks and itself.
func extractChildrenWithDFS(task models.ZentaoTaskRes) ([]models.ZentaoTaskRes, error) {
	var tasks []models.ZentaoTaskRes
	for _, child := range task.Children {
		childTasks, err := extractChildrenWithDFS(*child)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, childTasks...)
	}
	tasks = append(tasks, task)
	return tasks, nil
}

var CollectTaskMeta = plugin.SubTaskMeta{
	Name:             "collectTask",
	EntryPoint:       CollectTask,
	EnabledByDefault: true,
	Description:      "Collect Task data from Zentao api",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}
