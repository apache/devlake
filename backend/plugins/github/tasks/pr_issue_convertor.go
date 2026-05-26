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
	"github.com/spf13/cast"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ConvertPullRequestIssuesMeta)
}

var ConvertPullRequestIssuesMeta = plugin.SubTaskMeta{
	Name:             "Convert PR Issues",
	EntryPoint:       ConvertPullRequestIssues,
	EnabledByDefault: true,
	Description:      "Convert tool layer table github_pull_request_issues into domain layer table pull_request_issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{
		models.GithubPrIssue{}.TableName(),     // cursor and id generator
		models.GithubPullRequest{}.TableName(), // cursor and id generator
		RAW_PULL_REQUEST_TABLE},
	ProductTables: []string{crossdomain.PullRequestIssue{}.TableName()},
}

func ConvertPullRequestIssues(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	prIdGen := didgen.NewDomainIdGenerator(&models.GithubPullRequest{})
	issueIdGen := didgen.NewDomainIdGenerator(&models.GithubIssue{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubPrIssue]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_PULL_REQUEST_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.From(&models.GithubPrIssue{}),
				dal.Join(`left join _tool_github_pull_requests on _tool_github_pull_requests.github_id = _tool_github_pull_request_issues.pull_request_id`),
				dal.Where("_tool_github_pull_requests.repo_id = ? and _tool_github_pull_requests.connection_id = ?", repoId, data.Options.ConnectionId),
				dal.Orderby("pull_request_id ASC"),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_github_pull_requests.github_updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubPrIssue *models.GithubPrIssue) ([]interface{}, errors.Error) {
			pullRequestIssue := &crossdomain.PullRequestIssue{
				PullRequestId:  prIdGen.Generate(data.Options.ConnectionId, githubPrIssue.PullRequestId),
				IssueId:        issueIdGen.Generate(data.Options.ConnectionId, githubPrIssue.IssueId),
				IssueKey:       cast.ToString(githubPrIssue.IssueNumber),
				PullRequestKey: githubPrIssue.PullRequestNumber,
			}
			return []interface{}{
				pullRequestIssue,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
