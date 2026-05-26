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

var _ plugin.SubTaskEntryPoint = ExtractSubtask

var ExtractSubtaskMeta = plugin.SubTaskMeta{
	Name:             "ExtractSubtask",
	EntryPoint:       ExtractSubtask,
	EnabledByDefault: true,
	Description:      "Extract raw subtask data into tool layer table _tool_asana_tasks",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ExtractSubtask(taskCtx plugin.SubTaskContext) errors.Error {
	taskData := taskCtx.GetData().(*AsanaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: taskData.Options.ConnectionId,
				ProjectId:    taskData.Options.ProjectId,
			},
			Table: rawSubtaskTable,
		},
		Extract: func(resData *api.RawData) ([]interface{}, errors.Error) {
			apiTask := &asanaApiTask{}
			err := errors.Convert(json.Unmarshal(resData.Data, apiTask))
			if err != nil {
				return nil, err
			}

			// Get parent GID from input
			var input struct {
				Gid string `json:"gid"`
			}
			if err := errors.Convert(json.Unmarshal(resData.Input, &input)); err != nil {
				return nil, err
			}

			assigneeGid := ""
			assigneeName := ""
			if apiTask.Assignee != nil {
				assigneeGid = apiTask.Assignee.Gid
				assigneeName = apiTask.Assignee.Name
			}
			creatorGid := ""
			creatorName := ""
			if apiTask.CreatedBy != nil {
				creatorGid = apiTask.CreatedBy.Gid
				creatorName = apiTask.CreatedBy.Name
			}
			sectionGid := ""
			projectGid := taskData.Options.ProjectId
			for _, m := range apiTask.Memberships {
				if m.Project != nil {
					projectGid = m.Project.Gid
				}
				if m.Section != nil && m.Section.Gid != "" {
					sectionGid = m.Section.Gid
					break
				}
			}

			var dueOn *time.Time
			if apiTask.DueOn != "" {
				dueOn = parseAsanaDate(apiTask.DueOn)
			}

			toolTask := &models.AsanaTask{
				ConnectionId:    taskData.Options.ConnectionId,
				Gid:             apiTask.Gid,
				Name:            apiTask.Name,
				Notes:           apiTask.Notes,
				ResourceType:    apiTask.ResourceType,
				ResourceSubtype: apiTask.ResourceSubtype,
				Completed:       apiTask.Completed,
				CompletedAt:     apiTask.CompletedAt,
				DueOn:           dueOn,
				CreatedAt:       apiTask.CreatedAt,
				ModifiedAt:      apiTask.ModifiedAt,
				PermalinkUrl:    apiTask.PermalinkUrl,
				ProjectGid:      projectGid,
				SectionGid:      sectionGid,
				AssigneeGid:     assigneeGid,
				AssigneeName:    assigneeName,
				CreatorGid:      creatorGid,
				CreatorName:     creatorName,
				ParentGid:       input.Gid, // Parent is the task that has subtasks
				NumSubtasks:     apiTask.NumSubtasks,
			}
			return []interface{}{toolTask}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
