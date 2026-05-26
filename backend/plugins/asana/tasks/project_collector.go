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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

const rawProjectTable = "asana_projects"

var _ plugin.SubTaskEntryPoint = CollectProject

var CollectProjectMeta = plugin.SubTaskMeta{
	Name:             "CollectProject",
	EntryPoint:       CollectProject,
	EnabledByDefault: true,
	Description:      "Collect project data from Asana API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type asanaDataWrapper struct {
	Data json.RawMessage `json:"data"`
}

func CollectProject(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AsanaTaskData)
	collector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: rawProjectTable,
		},
		ApiClient:   data.ApiClient,
		UrlTemplate: "projects/{{ .Params.ProjectId }}",
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var w asanaDataWrapper
			err := api.UnmarshalResponse(res, &w)
			if err != nil {
				return nil, err
			}
			if len(w.Data) == 0 {
				return nil, nil
			}
			return []json.RawMessage{w.Data}, nil
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
