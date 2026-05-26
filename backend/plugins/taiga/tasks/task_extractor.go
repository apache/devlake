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
	"github.com/apache/incubator-devlake/plugins/taiga/models"
)

var _ plugin.SubTaskEntryPoint = ExtractTasks

var ExtractTasksMeta = plugin.SubTaskMeta{
	Name:             "extractTasks",
	EntryPoint:       ExtractTasks,
	EnabledByDefault: true,
	Description:      "extract Taiga tasks",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ExtractTasks(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TaigaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: TaigaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_TASK_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var apiTask struct {
				Id              uint64 `json:"id"`
				Ref             int    `json:"ref"`
				Subject         string `json:"subject"`
				StatusExtraInfo struct {
					Name string `json:"name"`
				} `json:"status_extra_info"`
				IsClosed            bool       `json:"is_closed"`
				CreatedDate         *time.Time `json:"created_date"`
				ModifiedDate        *time.Time `json:"modified_date"`
				FinishedDate        *time.Time `json:"finished_date"`
				AssignedTo          *uint64    `json:"assigned_to"`
				AssignedToExtraInfo *struct {
					FullNameDisplay string `json:"full_name_display"`
				} `json:"assigned_to_extra_info"`
				UserStory   *uint64 `json:"user_story"`
				Milestone   *uint64 `json:"milestone"`
				IsBlocked   bool    `json:"is_blocked"`
				BlockedNote string  `json:"blocked_note"`
			}
			err := json.Unmarshal(row.Data, &apiTask)
			if err != nil {
				return nil, errors.Default.Wrap(err, "error unmarshalling task")
			}

			var assignedTo uint64
			var assignedToName string
			if apiTask.AssignedTo != nil {
				assignedTo = *apiTask.AssignedTo
			}
			if apiTask.AssignedToExtraInfo != nil {
				assignedToName = apiTask.AssignedToExtraInfo.FullNameDisplay
			}
			var userStoryId uint64
			if apiTask.UserStory != nil {
				userStoryId = *apiTask.UserStory
			}
			var milestoneId uint64
			if apiTask.Milestone != nil {
				milestoneId = *apiTask.Milestone
			}

			task := &models.TaigaTask{
				ConnectionId:   data.Options.ConnectionId,
				ProjectId:      data.Options.ProjectId,
				TaskId:         apiTask.Id,
				Ref:            apiTask.Ref,
				Subject:        apiTask.Subject,
				Status:         apiTask.StatusExtraInfo.Name,
				IsClosed:       apiTask.IsClosed,
				CreatedDate:    apiTask.CreatedDate,
				ModifiedDate:   apiTask.ModifiedDate,
				FinishedDate:   apiTask.FinishedDate,
				AssignedTo:     assignedTo,
				AssignedToName: assignedToName,
				UserStoryId:    userStoryId,
				MilestoneId:    milestoneId,
				IsBlocked:      apiTask.IsBlocked,
				BlockedNote:    apiTask.BlockedNote,
			}

			return []interface{}{task}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
