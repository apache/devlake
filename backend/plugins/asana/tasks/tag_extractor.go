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

var _ plugin.SubTaskEntryPoint = ExtractTag

var ExtractTagMeta = plugin.SubTaskMeta{
	Name:             "ExtractTag",
	EntryPoint:       ExtractTag,
	EnabledByDefault: true,
	Description:      "Extract raw data into tool layer tables _tool_asana_tags and _tool_asana_task_tags",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type asanaApiTag struct {
	Gid          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
	Color        string `json:"color"`
	Notes        string `json:"notes"`
	PermalinkUrl string `json:"permalink_url"`
}

func ExtractTag(taskCtx plugin.SubTaskContext) errors.Error {
	taskData := taskCtx.GetData().(*AsanaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: taskData.Options.ConnectionId,
				ProjectId:    taskData.Options.ProjectId,
			},
			Table: rawTagTable,
		},
		Extract: func(resData *api.RawData) ([]interface{}, errors.Error) {
			apiTag := &asanaApiTag{}
			err := errors.Convert(json.Unmarshal(resData.Data, apiTag))
			if err != nil {
				return nil, err
			}

			// Get task GID from input
			var input struct {
				Gid string `json:"gid"`
			}
			if err := errors.Convert(json.Unmarshal(resData.Input, &input)); err != nil {
				return nil, err
			}

			toolTag := &models.AsanaTag{
				ConnectionId: taskData.Options.ConnectionId,
				Gid:          apiTag.Gid,
				Name:         apiTag.Name,
				ResourceType: apiTag.ResourceType,
				Color:        apiTag.Color,
				Notes:        apiTag.Notes,
				PermalinkUrl: apiTag.PermalinkUrl,
			}

			// Create the task-tag relationship
			taskTag := &models.AsanaTaskTag{
				ConnectionId: taskData.Options.ConnectionId,
				TaskGid:      input.Gid,
				TagGid:       apiTag.Gid,
			}

			return []interface{}{toolTag, taskTag}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
