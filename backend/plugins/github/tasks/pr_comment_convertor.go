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
	RegisterSubtaskMeta(&ConvertPullRequestCommentsMeta)
}

var ConvertPullRequestCommentsMeta = plugin.SubTaskMeta{
	Name:             "Convert PR Comments",
	EntryPoint:       ConvertPullRequestComments,
	EnabledByDefault: true,
	Description:      "ConvertPullRequestComments data from Github api",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	DependencyTables: []string{
		models.GithubPrComment{}.TableName(),   // cursor
		models.GithubPullRequest{}.TableName(), // cursor
		models.GithubAccount{}.TableName(),     // id generator
		models.GithubPrReview{}.TableName(),    // id generator
		RAW_COMMENTS_TABLE},
	ProductTables: []string{code.PullRequestComment{}.TableName()},
}

func ConvertPullRequestComments(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	prCommentIdGen := didgen.NewDomainIdGenerator(&models.GithubPrComment{})
	prIdGen := didgen.NewDomainIdGenerator(&models.GithubPullRequest{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})
	prReviewIdGen := didgen.NewDomainIdGenerator(&models.GithubPrReview{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubPrComment]{
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
				dal.From(&models.GithubPrComment{}),
				dal.Join("left join _tool_github_pull_requests " +
					"on _tool_github_pull_requests.github_id = _tool_github_pull_request_comments.pull_request_id"),
				dal.Where("repo_id = ? and _tool_github_pull_requests.connection_id = ?", repoId, data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_github_pull_request_comments.github_updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubPullRequestComment *models.GithubPrComment) ([]interface{}, errors.Error) {
			domainPrComment := &code.PullRequestComment{
				DomainEntity: domainlayer.DomainEntity{
					Id: prCommentIdGen.Generate(data.Options.ConnectionId, githubPullRequestComment.GithubId),
				},
				PullRequestId: prIdGen.Generate(data.Options.ConnectionId, githubPullRequestComment.PullRequestId),
				Body:          githubPullRequestComment.Body,
				AccountId:     accountIdGen.Generate(data.Options.ConnectionId, githubPullRequestComment.AuthorUserId),
				CreatedDate:   githubPullRequestComment.GithubCreatedAt,
				CommitSha:     githubPullRequestComment.CommitSha,
				ReviewId:      prReviewIdGen.Generate(data.Options.ConnectionId, githubPullRequestComment.ReviewId),
			}
			domainPrComment.Type = getStdCommentType(githubPullRequestComment.Type)
			return []interface{}{
				domainPrComment,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}

func getStdCommentType(originType string) string {
	if originType == "DIFF" {
		return code.DIFF_COMMENT
	}
	if originType == "REVIEW" {
		return code.REVIEW
	}
	if originType == "NORMAL" {
		return code.NORMAL_COMMENT
	}
	return ""
}
