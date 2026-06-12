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
	"strconv"
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ConvertIssuesMeta)
}

var ConvertIssuesMeta = plugin.SubTaskMeta{
	Name:             "Convert Issues",
	EntryPoint:       ConvertIssues,
	EnabledByDefault: true,
	Description:      "Convert tool layer table github_issues into  domain layer table issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{
		models.GithubIssue{}.TableName(),   // cursor
		models.GithubAccount{}.TableName(), // id generator
		//models.GithubRepo{}.TableName(),    // id generator, but config not regard as dependency
		RAW_ISSUE_TABLE},
	ProductTables: []string{
		ticket.Issue{}.TableName(),
		ticket.BoardIssue{}.TableName()},
}

func ConvertIssues(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	issueIdGen := didgen.NewDomainIdGenerator(&models.GithubIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.GithubRepo{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubIssue]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_ISSUE_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.From(&models.GithubIssue{}),
				dal.Where("repo_id = ? and connection_id=?", repoId, data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("github_updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(issue *models.GithubIssue) ([]interface{}, errors.Error) {
			domainIssue := &ticket.Issue{
				DomainEntity:    domainlayer.DomainEntity{Id: issueIdGen.Generate(data.Options.ConnectionId, issue.GithubId)},
				IssueKey:        strconv.Itoa(issue.Number),
				Title:           issue.Title,
				Description:     issue.Body,
				Priority:        issue.Priority,
				Type:            issue.StdType,
				OriginalType:    issue.Type,
				OriginalStatus:  issue.State,
				AssigneeName:    issue.AssigneeName,
				CreatorName:     issue.AuthorName,
				LeadTimeMinutes: issue.LeadTimeMinutes,
				Url:             issue.Url,
				CreatedDate:     &issue.GithubCreatedAt,
				UpdatedDate:     &issue.GithubUpdatedAt,
				ResolutionDate:  issue.ClosedAt,
				Severity:        issue.Severity,
				Component:       issue.Component,
			}
			if issue.AssigneeId != 0 {
				domainIssue.AssigneeId = accountIdGen.Generate(data.Options.ConnectionId, issue.AssigneeId)
			}
			if issue.AuthorId != 0 {
				domainIssue.CreatorId = accountIdGen.Generate(data.Options.ConnectionId, issue.AuthorId)
			}
			if strings.ToUpper(issue.State) == "CLOSED" {
				domainIssue.Status = ticket.DONE
			} else {
				domainIssue.Status = ticket.TODO
			}
			boardIssue := &ticket.BoardIssue{
				BoardId: boardIdGen.Generate(data.Options.ConnectionId, repoId),
				IssueId: domainIssue.Id,
			}
			return []interface{}{
				domainIssue,
				boardIssue,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
