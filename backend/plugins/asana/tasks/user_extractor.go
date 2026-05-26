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

var _ plugin.SubTaskEntryPoint = ExtractUser

var ExtractUserMeta = plugin.SubTaskMeta{
	Name:             "ExtractUser",
	EntryPoint:       ExtractUser,
	EnabledByDefault: true,
	Description:      "Extract raw data into tool layer table _tool_asana_users",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
}

type asanaApiUser struct {
	Gid          string `json:"gid"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	ResourceType string `json:"resource_type"`
	Photo        *struct {
		Image128x128 string `json:"image_128x128"`
	} `json:"photo"`
}

func ExtractUser(taskCtx plugin.SubTaskContext) errors.Error {
	taskData := taskCtx.GetData().(*AsanaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: taskData.Options.ConnectionId,
				ProjectId:    taskData.Options.ProjectId,
			},
			Table: rawUserTable,
		},
		Extract: func(resData *api.RawData) ([]interface{}, errors.Error) {
			apiUser := &asanaApiUser{}
			err := errors.Convert(json.Unmarshal(resData.Data, apiUser))
			if err != nil {
				return nil, err
			}
			photoUrl := ""
			if apiUser.Photo != nil {
				photoUrl = apiUser.Photo.Image128x128
			}
			toolUser := &models.AsanaUser{
				ConnectionId: taskData.Options.ConnectionId,
				Gid:          apiUser.Gid,
				Name:         apiUser.Name,
				Email:        apiUser.Email,
				ResourceType: apiUser.ResourceType,
				PhotoUrl:     photoUrl,
			}
			return []interface{}{toolUser}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
