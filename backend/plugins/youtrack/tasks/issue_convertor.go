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

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

var ConvertIssuesMeta = plugin.SubTaskMeta{
	Name:             "Convert Issues",
	EntryPoint:       ConvertIssues,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_youtrack_issues into domain layer tables issues, board_issues and issue_assignees",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.YoutrackIssue{}.TableName(), RAW_ISSUES_TABLE},
	ProductTables:    []string{ticket.Issue{}.TableName(), ticket.BoardIssue{}.TableName(), ticket.IssueAssignee{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertIssues

func ConvertIssues(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*YoutrackTaskData)
	connectionId := data.Options.ConnectionId

	issueIdGen := didgen.NewDomainIdGenerator(&models.YoutrackIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.YoutrackAccount{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.YoutrackProject{})
	boardId := boardIdGen.Generate(connectionId, data.Options.ProjectId)

	// OriginalProject is the scope's shortName, loaded once by
	// PrepareTaskData.
	if data.Project == nil {
		return errors.Default.New("youtrack task data carries no project scope")
	}
	shortName := data.Project.ShortName

	// Issues and board_issues are emitted for every issue, so the divider's
	// lazy wipe always fires for them; issue_assignees is emitted only for
	// assigned issues, so once no issue in the project has an assignee the
	// stale rows would survive. Delete them first.
	params := utils.ToJsonString(models.YoutrackApiParams{
		ConnectionId: connectionId,
		ProjectId:    data.Options.ProjectId,
	})
	if err := db.Delete(
		&ticket.IssueAssignee{},
		dal.Where("_raw_data_table = ? AND _raw_data_params = ?", "_raw_"+RAW_ISSUES_TABLE, params),
	); err != nil {
		return err
	}

	cursor, err := db.Cursor(
		dal.From(&models.YoutrackIssue{}),
		dal.Where("connection_id = ? AND project_id = ?", connectionId, data.Options.ProjectId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Options: models.YoutrackApiParams{
				ConnectionId: connectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_ISSUES_TABLE,
		},
		InputRowType: reflect.TypeOf(models.YoutrackIssue{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			issue := inputRow.(*models.YoutrackIssue)
			domainIssue := &ticket.Issue{
				DomainEntity:    domainlayer.DomainEntity{Id: issueIdGen.Generate(connectionId, issue.Id)},
				Url:             issueUrl(data.Connection.Endpoint, issue.IdReadable),
				IssueKey:        issue.IdReadable,
				Title:           issue.Summary,
				Description:     issue.Description,
				Type:            issue.StdType,
				OriginalType:    issue.TypeName,
				Status:          issue.StdStatus,
				OriginalStatus:  issue.StateName,
				Priority:        issue.PriorityName,
				StoryPoint:      issue.StoryPoint,
				DueDate:         issue.DueDate,
				ResolutionDate:  issue.Resolved,
				CreatedDate:     &issue.Created,
				UpdatedDate:     &issue.Updated,
				LeadTimeMinutes: issue.LeadTimeMinutes,
				OriginalProject: shortName,
			}
			if issue.ReporterId != "" {
				domainIssue.CreatorId = accountIdGen.Generate(connectionId, issue.ReporterId)
				domainIssue.CreatorName = issue.ReporterName
			}
			if issue.AssigneeId != "" {
				domainIssue.AssigneeId = accountIdGen.Generate(connectionId, issue.AssigneeId)
				domainIssue.AssigneeName = issue.AssigneeName
			}
			boardIssue := &ticket.BoardIssue{
				BoardId: boardId,
				IssueId: domainIssue.Id,
			}
			results := []interface{}{domainIssue, boardIssue}
			if domainIssue.AssigneeId != "" {
				results = append(results, &ticket.IssueAssignee{
					IssueId:      domainIssue.Id,
					AssigneeId:   domainIssue.AssigneeId,
					AssigneeName: domainIssue.AssigneeName,
				})
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
