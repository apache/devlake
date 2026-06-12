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
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ConvertPullRequestReviewsMeta)
}

var ConvertPullRequestReviewsMeta = plugin.SubTaskMeta{
	Name:             "Convert PR Reviews",
	EntryPoint:       ConvertPullRequestReviews,
	EnabledByDefault: true,
	Description:      "ConvertPullRequestReviews data from Github api",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	DependencyTables: []string{
		models.GithubPrReview{}.TableName(),    // cursor and id generator
		models.GithubPullRequest{}.TableName(), // cursor and id generator
		models.GithubAccount{}.TableName(),     // id generator
		RAW_PR_REVIEW_TABLE},
	ProductTables: []string{code.PullRequestComment{}.TableName()},
}

func ConvertPullRequestReviews(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	prReviewUIdGen := didgen.NewDomainIdGenerator(&models.GithubPrReview{})
	prIdGen := didgen.NewDomainIdGenerator(&models.GithubPullRequest{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubPrReview]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_PR_REVIEW_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.From(&models.GithubPrReview{}),
				dal.Join("left join _tool_github_pull_requests " +
					"on _tool_github_pull_requests.github_id = _tool_github_pull_request_reviews.pull_request_id"),
				dal.Where("repo_id = ? and _tool_github_pull_requests.connection_id = ?", repoId, data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_github_pull_requests.github_updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubPullRequestReview *models.GithubPrReview) ([]interface{}, errors.Error) {
			domainPrReview := &code.PullRequestComment{
				DomainEntity: domainlayer.DomainEntity{
					Id: prReviewUIdGen.Generate(data.Options.ConnectionId, githubPullRequestReview.GithubId),
				},
				PullRequestId: prIdGen.Generate(data.Options.ConnectionId, githubPullRequestReview.PullRequestId),
				Body:          githubPullRequestReview.Body,
				AccountId:     accountIdGen.Generate(data.Options.ConnectionId, githubPullRequestReview.AuthorUserId),
				CommitSha:     githubPullRequestReview.CommitSha,
				Type:          "REVIEW",
				Status:        githubPullRequestReview.State,
			}
			if githubPullRequestReview.GithubSubmitAt != nil {
				domainPrReview.CreatedDate = *githubPullRequestReview.GithubSubmitAt
			}
			return []interface{}{
				domainPrReview,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
