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
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ConvertIssueCommentsMeta)
}

var ConvertIssueCommentsMeta = plugin.SubTaskMeta{
	Name:             "Convert Issue Comments",
	EntryPoint:       ConvertIssueComments,
	EnabledByDefault: true,
	Description:      "ConvertIssueComments data from Github api",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{
		models.GithubIssueComment{}.TableName(), // cursor
		models.GithubIssue{}.TableName(),        // cursor and id generator
		models.GithubAccount{}.TableName(),      // id generator
		RAW_COMMENTS_TABLE},
	ProductTables: []string{ticket.IssueComment{}.TableName()},
}

func ConvertIssueComments(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	issueIdGen := didgen.NewDomainIdGenerator(&models.GithubIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubIssueComment]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_COMMENTS_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.From(&models.GithubIssueComment{}),
				dal.Join("left join _tool_github_issues " +
					"on _tool_github_issues.github_id = _tool_github_issue_comments.issue_id"),
				dal.Where("repo_id = ? and _tool_github_issues.connection_id = ?", repoId, data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_github_issue_comments.github_updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubIssueComment *models.GithubIssueComment) ([]interface{}, errors.Error) {
			domainIssueComment := &ticket.IssueComment{
				DomainEntity: domainlayer.DomainEntity{
					Id: issueIdGen.Generate(data.Options.ConnectionId, githubIssueComment.GithubId),
				},
				IssueId:     issueIdGen.Generate(data.Options.ConnectionId, githubIssueComment.IssueId),
				Body:        githubIssueComment.Body,
				AccountId:   accountIdGen.Generate(data.Options.ConnectionId, githubIssueComment.AuthorUserId),
				CreatedDate: githubIssueComment.GithubCreatedAt,
			}
			if !githubIssueComment.GithubUpdatedAt.IsZero() {
				domainIssueComment.UpdatedDate = &githubIssueComment.GithubUpdatedAt
			}
			return []interface{}{
				domainIssueComment,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
