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

var _ plugin.SubTaskEntryPoint = ExtractProject

var ExtractProjectMeta = plugin.SubTaskMeta{
	Name:             "ExtractProject",
	EntryPoint:       ExtractProject,
	EnabledByDefault: true,
	Description:      "Extract raw data into tool layer table _tool_asana_projects",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type asanaApiProject struct {
	Gid          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
	Archived     bool   `json:"archived"`
	PermalinkUrl string `json:"permalink_url"`
	Workspace    *struct {
		Gid string `json:"gid"`
	} `json:"workspace"`
}

func ExtractProject(taskCtx plugin.SubTaskContext) errors.Error {
	taskData := taskCtx.GetData().(*AsanaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: taskData.Options.ConnectionId,
				ProjectId:    taskData.Options.ProjectId,
			},
			Table: rawProjectTable,
		},
		Extract: func(resData *api.RawData) ([]interface{}, errors.Error) {
			apiProject := &asanaApiProject{}
			err := errors.Convert(json.Unmarshal(resData.Data, apiProject))
			if err != nil {
				return nil, err
			}
			workspaceGid := ""
			if apiProject.Workspace != nil {
				workspaceGid = apiProject.Workspace.Gid
			}
			toolProject := &models.AsanaProject{
				Gid:          apiProject.Gid,
				Name:         apiProject.Name,
				ResourceType: apiProject.ResourceType,
				Archived:     apiProject.Archived,
				PermalinkUrl: apiProject.PermalinkUrl,
				WorkspaceGid: workspaceGid,
			}
			toolProject.ConnectionId = taskData.Options.ConnectionId
			toolProject.ScopeConfigId = taskData.Options.ScopeConfigId
			return []interface{}{toolProject}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
