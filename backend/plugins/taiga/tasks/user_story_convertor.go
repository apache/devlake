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
	"fmt"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/taiga/models"
)

var ConvertUserStoriesMeta = plugin.SubTaskMeta{
	Name:             "convertUserStories",
	EntryPoint:       ConvertUserStories,
	EnabledByDefault: true,
	Description:      "convert Taiga user stories",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertUserStories(subtaskCtx plugin.SubTaskContext) errors.Error {
	logger := subtaskCtx.GetLogger()
	data := subtaskCtx.GetData().(*TaigaTaskData)
	db := subtaskCtx.GetDal()

	issueIdGen := didgen.NewDomainIdGenerator(&models.TaigaUserStory{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.TaigaProject{})
	boardId := boardIdGen.Generate(data.Options.ConnectionId, data.Options.ProjectId)

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.TaigaUserStory]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: subtaskCtx,
			Table:          RAW_USER_STORY_TABLE,
			Params: TaigaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.Select("*"),
				dal.From(&models.TaigaUserStory{}),
				dal.Where("connection_id = ?", data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(userStory *models.TaigaUserStory) ([]interface{}, errors.Error) {
			var result []interface{}

			// Map Taiga is_closed to DevLake standard status
			status := "TODO"
			if userStory.IsClosed {
				status = "DONE"
			}

			issue := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: issueIdGen.Generate(userStory.ConnectionId, userStory.UserStoryId),
				},
				IssueKey:       fmt.Sprintf("#%d", userStory.Ref),
				Title:          userStory.Subject,
				Type:           "USER_STORY",
				OriginalType:   "User Story",
				Status:         status,
				OriginalStatus: userStory.Status,
				CreatedDate:    userStory.CreatedDate,
				UpdatedDate:    userStory.ModifiedDate,
				ResolutionDate: userStory.FinishedDate,
				AssigneeId:     fmt.Sprintf("%d", userStory.AssignedTo),
				AssigneeName:   userStory.AssignedToName,
			}

			if userStory.TotalPoints > 0 {
				issue.StoryPoint = &userStory.TotalPoints
			}

			// Calculate lead time: creation → resolution for closed stories
			if userStory.IsClosed && issue.CreatedDate != nil && issue.ResolutionDate != nil {
				leadTimeMinutes := uint(issue.ResolutionDate.Sub(*issue.CreatedDate).Minutes())
				if leadTimeMinutes > 0 {
					issue.LeadTimeMinutes = &leadTimeMinutes
				}
			}

			result = append(result, issue)

			boardIssue := &ticket.BoardIssue{
				BoardId: boardId,
				IssueId: issue.Id,
			}
			result = append(result, boardIssue)

			logger.Debug("converted user story %d", userStory.UserStoryId)
			return result, nil
		},
	})

	if err != nil {
		return err
	}

	return converter.Execute()
}
