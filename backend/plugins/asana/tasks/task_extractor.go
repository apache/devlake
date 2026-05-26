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

var _ plugin.SubTaskEntryPoint = ExtractTask

var ExtractTaskMeta = plugin.SubTaskMeta{
	Name:             "ExtractTask",
	EntryPoint:       ExtractTask,
	EnabledByDefault: true,
	Description:      "Extract raw data into tool layer table _tool_asana_tasks",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type asanaApiTask struct {
	Gid             string     `json:"gid"`
	Name            string     `json:"name"`
	Notes           string     `json:"notes"`
	ResourceType    string     `json:"resource_type"`
	ResourceSubtype string     `json:"resource_subtype"`
	Completed       bool       `json:"completed"`
	CompletedAt     *time.Time `json:"completed_at"`
	DueOn           string     `json:"due_on"`
	CreatedAt       time.Time  `json:"created_at"`
	ModifiedAt      *time.Time `json:"modified_at"`
	PermalinkUrl    string     `json:"permalink_url"`
	Assignee        *struct {
		Gid  string `json:"gid"`
		Name string `json:"name"`
	} `json:"assignee"`
	CreatedBy *struct {
		Gid  string `json:"gid"`
		Name string `json:"name"`
	} `json:"created_by"`
	Parent *struct {
		Gid string `json:"gid"`
	} `json:"parent"`
	NumSubtasks int `json:"num_subtasks"`
	Memberships []struct {
		Section *struct {
			Gid  string `json:"gid"`
			Name string `json:"name"`
		} `json:"section"`
		Project *struct {
			Gid string `json:"gid"`
		} `json:"project"`
	} `json:"memberships"`
}

func parseAsanaDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

func ExtractTask(taskCtx plugin.SubTaskContext) errors.Error {
	taskData := taskCtx.GetData().(*AsanaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.AsanaApiParams{
				ConnectionId: taskData.Options.ConnectionId,
				ProjectId:    taskData.Options.ProjectId,
			},
			Table: rawTaskTable,
		},
		Extract: func(resData *api.RawData) ([]interface{}, errors.Error) {
			apiTask := &asanaApiTask{}
			err := errors.Convert(json.Unmarshal(resData.Data, apiTask))
			if err != nil {
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
			parentGid := ""
			if apiTask.Parent != nil {
				parentGid = apiTask.Parent.Gid
			}
			sectionGid := ""
			sectionName := ""
			projectGid := taskData.Options.ProjectId
			for _, m := range apiTask.Memberships {
				if m.Project != nil {
					projectGid = m.Project.Gid
				}
				if m.Section != nil && m.Section.Gid != "" {
					sectionGid = m.Section.Gid
					sectionName = m.Section.Name
					break
				}
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
				DueOn:           parseAsanaDate(apiTask.DueOn),
				CreatedAt:       apiTask.CreatedAt,
				ModifiedAt:      apiTask.ModifiedAt,
				PermalinkUrl:    apiTask.PermalinkUrl,
				ProjectGid:      projectGid,
				SectionGid:      sectionGid,
				SectionName:     sectionName,
				AssigneeGid:     assigneeGid,
				AssigneeName:    assigneeName,
				CreatorGid:      creatorGid,
				CreatorName:     creatorName,
				ParentGid:       parentGid,
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
