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
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ConvertIssueAssigneeMeta)
}

var ConvertIssueAssigneeMeta = plugin.SubTaskMeta{
	Name:             "Convert Issue Assignees",
	EntryPoint:       ConvertIssueAssignee,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_github_issue_assignees into  domain layer table issue_assignees",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{
		//models.GithubIssueAssignee{}.TableName(), // cursor, not regard as dependency
		models.GithubIssue{}.TableName(), // id generator
		RAW_ISSUE_TABLE,
		//models.GithubAccount{}.TableName(),       // id generator, config will not regard as dependency
	},
	ProductTables: []string{models.GithubIssueAssignee{}.TableName()},
}

func ConvertIssueAssignee(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	issueIdGen := didgen.NewDomainIdGenerator(&models.GithubIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubIssueAssignee]{
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
				dal.From(&models.GithubIssueAssignee{}),
				dal.Where("_tool_github_issue_assignees.repo_id = ? and _tool_github_issue_assignees.connection_id = ?", repoId, data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses,
						dal.Join(`left join _tool_github_issues on _tool_github_issues.github_id = _tool_github_issue_assignees.issue_id and _tool_github_issues.connection_id = _tool_github_issue_assignees.connection_id`),
					)
					clauses = append(clauses, dal.Where("_tool_github_issues.github_updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubIssueAssignee *models.GithubIssueAssignee) ([]interface{}, errors.Error) {
			issueAssignee := &ticket.IssueAssignee{
				IssueId:      issueIdGen.Generate(data.Options.ConnectionId, githubIssueAssignee.IssueId),
				AssigneeId:   accountIdGen.Generate(data.Options.ConnectionId, githubIssueAssignee.AssigneeId),
				AssigneeName: githubIssueAssignee.AssigneeName,
			}
			return []interface{}{issueAssignee}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
