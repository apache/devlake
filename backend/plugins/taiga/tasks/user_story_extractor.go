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

var _ plugin.SubTaskEntryPoint = ExtractUserStories

var ExtractUserStoriesMeta = plugin.SubTaskMeta{
	Name:             "extractUserStories",
	EntryPoint:       ExtractUserStories,
	EnabledByDefault: true,
	Description:      "extract Taiga user stories",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ExtractUserStories(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TaigaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: TaigaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_USER_STORY_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var apiUserStory struct {
				Id              uint64 `json:"id"`
				Ref             int    `json:"ref"`
				Subject         string `json:"subject"`
				Status          uint64 `json:"status"`
				StatusExtraInfo struct {
					Name string `json:"name"`
				} `json:"status_extra_info"`
				IsClosed            bool       `json:"is_closed"`
				CreatedDate         *time.Time `json:"created_date"`
				ModifiedDate        *time.Time `json:"modified_date"`
				FinishDate          *time.Time `json:"finish_date"`
				AssignedTo          *uint64    `json:"assigned_to"`
				AssignedToExtraInfo *struct {
					FullNameDisplay string `json:"full_name_display"`
				} `json:"assigned_to_extra_info"`
				TotalPoints   *float64 `json:"total_points"`
				MilestoneId   *uint64  `json:"milestone"`
				MilestoneName *string  `json:"milestone_name"`
				Priority      *int     `json:"priority"`
				IsBlocked     bool     `json:"is_blocked"`
				BlockedNote   string   `json:"blocked_note"`
			}
			err := json.Unmarshal(row.Data, &apiUserStory)
			if err != nil {
				return nil, errors.Default.Wrap(err, "error unmarshalling user story")
			}

			var assignedTo uint64
			var assignedToName string
			if apiUserStory.AssignedTo != nil {
				assignedTo = *apiUserStory.AssignedTo
			}
			if apiUserStory.AssignedToExtraInfo != nil {
				assignedToName = apiUserStory.AssignedToExtraInfo.FullNameDisplay
			}
			var totalPoints float64
			if apiUserStory.TotalPoints != nil {
				totalPoints = *apiUserStory.TotalPoints
			}
			var milestoneId uint64
			if apiUserStory.MilestoneId != nil {
				milestoneId = *apiUserStory.MilestoneId
			}
			var milestoneName string
			if apiUserStory.MilestoneName != nil {
				milestoneName = *apiUserStory.MilestoneName
			}
			var priority int
			if apiUserStory.Priority != nil {
				priority = *apiUserStory.Priority
			}

			userStory := &models.TaigaUserStory{
				ConnectionId:   data.Options.ConnectionId,
				ProjectId:      data.Options.ProjectId,
				UserStoryId:    apiUserStory.Id,
				Ref:            apiUserStory.Ref,
				Subject:        apiUserStory.Subject,
				Status:         apiUserStory.StatusExtraInfo.Name,
				IsClosed:       apiUserStory.IsClosed,
				CreatedDate:    apiUserStory.CreatedDate,
				ModifiedDate:   apiUserStory.ModifiedDate,
				FinishedDate:   apiUserStory.FinishDate,
				AssignedTo:     assignedTo,
				AssignedToName: assignedToName,
				TotalPoints:    totalPoints,
				MilestoneId:    milestoneId,
				MilestoneName:  milestoneName,
				Priority:       priority,
				IsBlocked:      apiUserStory.IsBlocked,
				BlockedNote:    apiUserStory.BlockedNote,
			}

			return []interface{}{userStory}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
