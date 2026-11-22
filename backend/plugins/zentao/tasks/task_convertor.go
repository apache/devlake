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
	"reflect"
	"strconv"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/zentao/models"
)

var _ plugin.SubTaskEntryPoint = ConvertTask

var ConvertTaskMeta = plugin.SubTaskMeta{
	Name:             "convertTask",
	EntryPoint:       ConvertTask,
	EnabledByDefault: true,
	Description:      "convert Zentao task",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertTask(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*ZentaoTaskData)
	db := taskCtx.GetDal()
	//bugIdGen := didgen.NewDomainIdGenerator(&models.ZentaoBug{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.ZentaoProject{})
	executionIdGen := didgen.NewDomainIdGenerator(&models.ZentaoExecution{})
	taskIdGen := didgen.NewDomainIdGenerator(&models.ZentaoTask{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.ZentaoAccount{})
	stdTypeMappings := getStdTypeMappings(data)
	cursor, err := db.Cursor(
		dal.From(&models.ZentaoTask{}),
		dal.Where(`project = ? and connection_id = ?`, data.Options.ProjectId, data.Options.ConnectionId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()
	convertor, err := api.NewDataConverter(api.DataConverterArgs{
		InputRowType: reflect.TypeOf(models.ZentaoTask{}),
		Input:        cursor,
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_TASK_TABLE,
		},
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			toolEntity := inputRow.(*models.ZentaoTask)
			originalEstimateMinutes := int64(toolEntity.Estimate * 60)
			timeSpentMinutes := int64(toolEntity.Consumed * 60)
			timeRemainingMinutes := int64(toolEntity.Left * 60)

			domainEntity := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: taskIdGen.Generate(toolEntity.ConnectionId, toolEntity.ID),
				},
				IssueKey:                strconv.FormatInt(toolEntity.ID, 10),
				Title:                   toolEntity.Name,
				Description:             toolEntity.Description,
				Type:                    toolEntity.StdType,
				OriginalType:            "task",
				OriginalStatus:          toolEntity.Status,
				ResolutionDate:          toolEntity.ClosedDate.ToNullableTime(),
				CreatedDate:             toolEntity.OpenedDate.ToNullableTime(),
				UpdatedDate:             toolEntity.LastEditedDate.ToNullableTime(),
				Priority:                getPriority(toolEntity.Pri),
				CreatorName:             toolEntity.OpenedByName,
				AssigneeName:            toolEntity.AssignedToName,
				Url:                     convertIssueURL(toolEntity.Url, "task", toolEntity.ID),
				OriginalProject:         getOriginalProject(data),
				Status:                  toolEntity.StdStatus,
				OriginalEstimateMinutes: &originalEstimateMinutes,
				TimeSpentMinutes:        &timeSpentMinutes,
				TimeRemainingMinutes:    &timeRemainingMinutes,
			}
			if mappingType, ok := stdTypeMappings[domainEntity.OriginalType]; ok && mappingType != "" {
				domainEntity.Type = mappingType
			}
			// Parent is the id of the parent task, or -1 if this task is itself a parent task
			if toolEntity.Parent > 0 {
				domainEntity.ParentIssueId = taskIdGen.Generate(data.Options.ConnectionId, toolEntity.Parent)
			}
			if toolEntity.OpenedById != 0 {
				domainEntity.CreatorId = accountIdGen.Generate(data.Options.ConnectionId, toolEntity.OpenedById)
			}
			if toolEntity.AssignedToId != 0 {
				domainEntity.AssigneeId = accountIdGen.Generate(data.Options.ConnectionId, toolEntity.AssignedToId)
			}
			if toolEntity.DueDate != nil {
				domainEntity.DueDate = toolEntity.DueDate
			}
			closedDate := toolEntity.ClosedDate
			openedDate := toolEntity.OpenedDate
			if closedDate != nil && closedDate.ToTime().After(openedDate.ToTime()) {
				temp := uint(closedDate.ToNullableTime().Sub(openedDate.ToTime()).Minutes())
				domainEntity.LeadTimeMinutes = &temp
			}
			var results []interface{}
			if domainEntity.AssigneeId != "" {
				issueAssignee := &ticket.IssueAssignee{
					IssueId:      domainEntity.Id,
					AssigneeId:   domainEntity.AssigneeId,
					AssigneeName: domainEntity.AssigneeName,
				}
				results = append(results, issueAssignee)
			}
			domainBoardIssue := &ticket.BoardIssue{
				BoardId: boardIdGen.Generate(data.Options.ConnectionId, data.Options.ProjectId),
				IssueId: domainEntity.Id,
			}

			// Parent < 0 means that this is a parent task, not a subtask, so we don't need to create a sprint issue
			if toolEntity.Execution > 0 && toolEntity.Parent >= 0 {
				sprintIssueTask := &ticket.SprintIssue{
					SprintId: executionIdGen.Generate(toolEntity.ConnectionId, toolEntity.Execution),
					IssueId:  domainEntity.Id,
				}
				results = append(results, sprintIssueTask)
			}

			results = append(results, domainEntity, domainBoardIssue)

			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return convertor.Execute()
}
