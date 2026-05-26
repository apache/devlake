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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

var _ plugin.SubTaskEntryPoint = ExtractStory

var ExtractStoryMeta = plugin.SubTaskMeta{
	Name:             "ExtractStory",
	EntryPoint:       ExtractStory,
	EnabledByDefault: true,
	Description:      "Extract raw data into tool layer table _tool_asana_stories",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type asanaApiStory struct {
	Gid             string    `json:"gid"`
	ResourceType    string    `json:"resource_type"`
	ResourceSubtype string    `json:"resource_subtype"`
	Text            string    `json:"text"`
	HtmlText        string    `json:"html_text"`
	IsPinned        bool      `json:"is_pinned"`
	IsEdited        bool      `json:"is_edited"`
	StickerName     string    `json:"sticker_name"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       *struct {
		Gid  string `json:"gid"`
		Name string `json:"name"`
	} `json:"created_by"`
	Target *struct {
		Gid string `json:"gid"`
	} `json:"target"`
}

func ExtractStory(taskCtx plugin.SubTaskContext) errors.Error {
	taskData := taskCtx.GetData().(*AsanaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: taskData.Options.ConnectionId,
				ProjectId:    taskData.Options.ProjectId,
			},
			Table: rawStoryTable,
		},
		Extract: func(resData *api.RawData) ([]interface{}, errors.Error) {
			apiStory := &asanaApiStory{}
			err := errors.Convert(json.Unmarshal(resData.Data, apiStory))
			if err != nil {
				return nil, err
			}

			// Extract task GID from input
			var input struct {
				Gid string `json:"gid"`
			}
			if err := errors.Convert(json.Unmarshal(resData.Input, &input)); err != nil {
				return nil, err
			}

			createdByGid := ""
			createdByName := ""
			if apiStory.CreatedBy != nil {
				createdByGid = apiStory.CreatedBy.Gid
				createdByName = apiStory.CreatedBy.Name
			}
			targetGid := ""
			if apiStory.Target != nil {
				targetGid = apiStory.Target.Gid
			}

			toolStory := &models.AsanaStory{
				ConnectionId:    taskData.Options.ConnectionId,
				Gid:             apiStory.Gid,
				ResourceType:    apiStory.ResourceType,
				ResourceSubtype: apiStory.ResourceSubtype,
				Text:            apiStory.Text,
				HtmlText:        apiStory.HtmlText,
				IsPinned:        apiStory.IsPinned,
				IsEdited:        apiStory.IsEdited,
				StickerName:     apiStory.StickerName,
				CreatedAt:       apiStory.CreatedAt,
				CreatedByGid:    createdByGid,
				CreatedByName:   createdByName,
				TaskGid:         input.Gid,
				TargetGid:       targetGid,
			}
			return []interface{}{toolStory}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
