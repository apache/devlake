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
	"encoding/json"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"

	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractApiPullRequestCommitsMeta)
}

var ExtractApiPullRequestCommitsMeta = plugin.SubTaskMeta{
	Name:             "Extract PR Commits",
	EntryPoint:       ExtractApiPullRequestCommits,
	EnabledByDefault: true,
	Description:      "Extract raw PullRequestCommits data into tool layer table github_commits",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS, plugin.DOMAIN_TYPE_CODE_REVIEW},
	DependencyTables: []string{RAW_PR_COMMIT_TABLE},
	ProductTables: []string{
		models.GithubRepoCommit{}.TableName(),
		models.GithubCommit{}.TableName(),
		models.GithubPrCommit{}.TableName()},
}

type PrCommitsResponse struct {
	Sha    string `json:"sha"`
	Commit PullRequestCommit
	Url    string
}

type PullRequestCommit struct {
	Author struct {
		Id    int
		Name  string
		Email string
		Date  common.Iso8601Time
	}
	Committer struct {
		Name  string
		Email string
		Date  common.Iso8601Time
	}
	Message json.RawMessage
}

func ExtractApiPullRequestCommits(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId
	extractor, err := helper.NewStatefulApiExtractor(&helper.StatefulApiExtractorArgs[PrCommitsResponse]{
		SubtaskCommonArgs: &helper.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_PR_COMMIT_TABLE,
		},
		Extract: func(body *PrCommitsResponse, row *helper.RawData) ([]any, errors.Error) {
			if strings.HasPrefix(string(row.Data), "{\"message\": \"Not Found\"") {
				return nil, nil
			}
			pull := &SimplePr{}
			err := errors.Convert(json.Unmarshal(row.Input, pull))
			if err != nil {
				return nil, err
			}
			results := make([]interface{}, 0, 3)
			githubRepoCommit := &models.GithubRepoCommit{
				ConnectionId: data.Options.ConnectionId,
				RepoId:       repoId,
				CommitSha:    body.Sha,
			}
			results = append(results, githubRepoCommit)
			githubCommit, err := convertPullRequestCommit(body, data.Options.ConnectionId)
			if err != nil {
				return nil, err
			}
			results = append(results, githubCommit)
			githubPullRequestCommit := &models.GithubPrCommit{
				ConnectionId:       data.Options.ConnectionId,
				CommitSha:          body.Sha,
				PullRequestId:      pull.GithubId,
				CommitAuthorName:   githubCommit.AuthorName,
				CommitAuthorEmail:  githubCommit.AuthorEmail,
				CommitAuthoredDate: githubCommit.AuthoredDate,
			}
			results = append(results, githubPullRequestCommit)
			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}

func convertPullRequestCommit(prCommit *PrCommitsResponse, connId uint64) (*models.GithubCommit, errors.Error) {
	githubCommit := &models.GithubCommit{
		Sha:            prCommit.Sha,
		Message:        string(prCommit.Commit.Message),
		AuthorId:       prCommit.Commit.Author.Id,
		AuthorName:     prCommit.Commit.Author.Name,
		AuthorEmail:    prCommit.Commit.Author.Email,
		AuthoredDate:   prCommit.Commit.Author.Date.ToTime(),
		CommitterName:  prCommit.Commit.Committer.Name,
		CommitterEmail: prCommit.Commit.Committer.Email,
		CommittedDate:  prCommit.Commit.Committer.Date.ToTime(),
		Url:            prCommit.Url,
	}
	return githubCommit, nil
}
