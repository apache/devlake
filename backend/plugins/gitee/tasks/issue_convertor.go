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
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/gitee/models"
)

var ConvertIssuesMeta = plugin.SubTaskMeta{
	Name:             "convertIssues",
	EntryPoint:       ConvertIssues,
	EnabledByDefault: true,
	Description:      "Convert tool layer table gitee_issues into  domain layer table issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertIssues(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RAW_ISSUE_TABLE)
	repoId := data.Repo.GiteeId

	issue := &models.GiteeIssue{}
	cursor, err := db.Cursor(
		dal.From(issue),
		dal.Where("repo_id = ? and connection_id=?", repoId, data.Options.ConnectionId),
	)

	if err != nil {
		return err
	}
	defer cursor.Close()

	issueIdGen := didgen.NewDomainIdGenerator(&models.GiteeIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.GiteeAccount{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.GiteeRepo{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(models.GiteeIssue{}),
		Input:              cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			issue := inputRow.(*models.GiteeIssue)
			domainIssue := &ticket.Issue{
				DomainEntity:    domainlayer.DomainEntity{Id: issueIdGen.Generate(data.Options.ConnectionId, issue.GiteeId)},
				IssueKey:        issue.Number,
				Title:           issue.Title,
				Description:     issue.Body,
				Priority:        issue.Priority,
				Type:            issue.Type,
				AssigneeName:    issue.AssigneeName,
				CreatorName:     issue.AuthorName,
				LeadTimeMinutes: issue.LeadTimeMinutes,
				Url:             issue.Url,
				CreatedDate:     &issue.GiteeCreatedAt,
				UpdatedDate:     &issue.GiteeUpdatedAt,
				ResolutionDate:  issue.ClosedAt,
				Severity:        issue.Severity,
				Component:       issue.Component,
			}
			var result []interface{}
			if issue.AssigneeId != 0 {
				domainIssue.AssigneeId = accountIdGen.Generate(data.Options.ConnectionId, issue.AssigneeId)
				issueAssignee := &ticket.IssueAssignee{
					IssueId:      domainIssue.Id,
					AssigneeId:   domainIssue.AssigneeId,
					AssigneeName: domainIssue.AssigneeName,
				}
				result = append(result, issueAssignee)
			}
			if issue.AuthorId != 0 {
				domainIssue.CreatorId = accountIdGen.Generate(data.Options.ConnectionId, issue.AuthorId)
			}
			domainIssue.Status = convertGiteeIssueStatus(issue.State)
			domainIssue.OriginalStatus = issue.State
			result = append(result, domainIssue)
			boardIssue := &ticket.BoardIssue{
				BoardId: boardIdGen.Generate(data.Options.ConnectionId, repoId),
				IssueId: domainIssue.Id,
			}
			result = append(result, boardIssue)
			return result, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}

// convertGiteeIssueStatus maps a Gitee issue state (open, progressing,
// closed, rejected) to a domain ticket status.
func convertGiteeIssueStatus(state string) string {
	switch state {
	case "closed", "rejected":
		return ticket.DONE
	case "progressing":
		return ticket.IN_PROGRESS
	default:
		return ticket.TODO
	}
}
