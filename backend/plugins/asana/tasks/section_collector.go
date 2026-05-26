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

const rawSectionTable = "asana_sections"

var _ plugin.SubTaskEntryPoint = CollectSection

var CollectSectionMeta = plugin.SubTaskMeta{
	Name:             "CollectSection",
	EntryPoint:       CollectSection,
	EnabledByDefault: true,
	Description:      "Collect section data from Asana API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type asanaListWrapper struct {
	Data []json.RawMessage `json:"data"`
}

func CollectSection(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AsanaTaskData)
	collector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: rawSectionTable,
		},
		ApiClient:   data.ApiClient,
		UrlTemplate: "projects/{{ .Params.ProjectId }}/sections",
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var w asanaListWrapper
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
