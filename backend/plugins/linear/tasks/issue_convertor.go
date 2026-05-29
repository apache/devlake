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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
)

var ConvertIssuesMeta = plugin.SubTaskMeta{
	Name:             "Convert Issues",
	EntryPoint:       ConvertIssues,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_linear_issues into domain layer tables issues and board_issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.LinearIssue{}.TableName(), RAW_ISSUES_TABLE},
	ProductTables:    []string{ticket.Issue{}.TableName(), ticket.BoardIssue{}.TableName(), ticket.IssueAssignee{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertIssues

func ConvertIssues(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)
	connectionId := data.Options.ConnectionId

	issueIdGen := didgen.NewDomainIdGenerator(&models.LinearIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.LinearAccount{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.LinearTeam{})
	boardId := boardIdGen.Generate(connectionId, data.Options.TeamId)

	// Preload account display names so issues can carry assignee/creator names
	// and emit issue_assignees rows, mirroring how the account convertor derives
	// the domain account's full name (displayName, falling back to name).
	var accounts []models.LinearAccount
	if err := db.All(&accounts, dal.Where("connection_id = ?", connectionId)); err != nil {
		return err
	}
	accountNames := make(map[string]string, len(accounts))
	for _, account := range accounts {
		name := account.Name
		if account.DisplayName != "" {
			name = account.DisplayName
		}
		accountNames[account.Id] = name
	}

	cursor, err := db.Cursor(
		dal.From(&models.LinearIssue{}),
		dal.Where("connection_id = ? AND team_id = ?", connectionId, data.Options.TeamId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: connectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_ISSUES_TABLE,
		},
		InputRowType: reflect.TypeOf(models.LinearIssue{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			issue := inputRow.(*models.LinearIssue)
			domainIssue := &ticket.Issue{
				DomainEntity:   domainlayer.DomainEntity{Id: issueIdGen.Generate(connectionId, issue.Id)},
				IssueKey:       issue.Identifier,
				Title:          issue.Title,
				Description:    issue.Description,
				Url:            issue.Url,
				Type:           ticket.REQUIREMENT,
				Status:         StatusFromStateType(issue.StateType),
				OriginalStatus: issue.StateName,
				StoryPoint:     issue.Estimate,
				Priority:       issue.PriorityLabel,
				CreatedDate:    &issue.CreatedAt,
				UpdatedDate:    &issue.UpdatedAt,
			}
			if issue.CreatorId != "" {
				domainIssue.CreatorId = accountIdGen.Generate(connectionId, issue.CreatorId)
				domainIssue.CreatorName = accountNames[issue.CreatorId]
			}
			if issue.AssigneeId != "" {
				domainIssue.AssigneeId = accountIdGen.Generate(connectionId, issue.AssigneeId)
				domainIssue.AssigneeName = accountNames[issue.AssigneeId]
			}
			if issue.ParentId != "" {
				domainIssue.ParentIssueId = issueIdGen.Generate(connectionId, issue.ParentId)
				domainIssue.IsSubtask = true
			}
			// Resolution date: completedAt, falling back to canceledAt.
			if issue.CompletedAt != nil {
				domainIssue.ResolutionDate = issue.CompletedAt
			} else if issue.CanceledAt != nil {
				domainIssue.ResolutionDate = issue.CanceledAt
			}
			// Fallback lead time when no history-derived value is present.
			// Guard against a resolution that precedes creation (clock skew or
			// migrated/imported issues): a negative duration cast to uint yields
			// platform-dependent garbage, so leave lead time unset instead.
			if domainIssue.LeadTimeMinutes == nil && domainIssue.ResolutionDate != nil &&
				domainIssue.ResolutionDate.After(issue.CreatedAt) {
				minutes := uint(domainIssue.ResolutionDate.Sub(issue.CreatedAt).Minutes())
				domainIssue.LeadTimeMinutes = &minutes
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
