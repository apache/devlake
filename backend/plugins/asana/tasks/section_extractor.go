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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

var _ plugin.SubTaskEntryPoint = ExtractSection

var ExtractSectionMeta = plugin.SubTaskMeta{
	Name:             "ExtractSection",
	EntryPoint:       ExtractSection,
	EnabledByDefault: true,
	Description:      "Extract raw data into tool layer table _tool_asana_sections",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type asanaApiSection struct {
	Gid          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
	Project      *struct {
		Gid string `json:"gid"`
	} `json:"project"`
}

func ExtractSection(taskCtx plugin.SubTaskContext) errors.Error {
	taskData := taskCtx.GetData().(*AsanaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: taskData.Options.ConnectionId,
				ProjectId:    taskData.Options.ProjectId,
			},
			Table: rawSectionTable,
		},
		Extract: func(resData *api.RawData) ([]interface{}, errors.Error) {
			apiSection := &asanaApiSection{}
			err := errors.Convert(json.Unmarshal(resData.Data, apiSection))
			if err != nil {
				return nil, err
			}
			projectGid := taskData.Options.ProjectId
			if apiSection.Project != nil {
				projectGid = apiSection.Project.Gid
			}
			toolSection := &models.AsanaSection{
				ConnectionId: taskData.Options.ConnectionId,
				Gid:          apiSection.Gid,
				Name:         apiSection.Name,
				ResourceType: apiSection.ResourceType,
				ProjectGid:   projectGid,
			}
			return []interface{}{toolSection}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
