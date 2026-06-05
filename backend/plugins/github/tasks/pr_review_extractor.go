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
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractApiPullRequestReviewsMeta)
}

var ExtractApiPullRequestReviewsMeta = plugin.SubTaskMeta{
	Name:             "Extract PR Reviews",
	EntryPoint:       ExtractApiPullRequestReviews,
	EnabledByDefault: true,
	Description:      "Extract raw PullRequestReviewers data into tool layer table github_reviewers",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS, plugin.DOMAIN_TYPE_CODE_REVIEW},
	DependencyTables: []string{RAW_PR_REVIEW_TABLE},
	ProductTables: []string{
		models.GithubRepoAccount{}.TableName(),
		models.GithubReviewer{}.TableName(),
		models.GithubPrReview{}.TableName()},
}

type PullRequestReview struct {
	GithubId    int `json:"id"`
	User        *GithubAccountResponse
	Body        string
	State       string
	CommitId    string             `json:"commit_id"`
	SubmittedAt common.Iso8601Time `json:"submitted_at"`
}

func ExtractApiPullRequestReviews(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)
	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[PullRequestReview]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_PR_REVIEW_TABLE,
		},
		Extract: func(body *PullRequestReview, row *api.RawData) ([]any, errors.Error) {
			if strings.HasPrefix(string(row.Data), "{\"message\": \"Not Found\"") {
				return nil, nil
			}
			if body.State == "PENDING" || body.User == nil {
				return nil, nil
			}
			// Filter bot reviews by username
			if shouldSkipByUsername(body.User.Login) {
				taskCtx.GetLogger().Debug("Skipping review #%d from bot user: %s", body.GithubId, body.User.Login)
				return nil, nil
			}
			pull := &SimplePr{}
			err := errors.Convert(json.Unmarshal(row.Input, pull))
			if err != nil {
				return nil, err
			}
			results := make([]interface{}, 0, 1)
			githubReviewer := &models.GithubReviewer{
				ConnectionId:  data.Options.ConnectionId,
				PullRequestId: pull.GithubId,
			}
			githubPrReview := &models.GithubPrReview{
				ConnectionId:   data.Options.ConnectionId,
				GithubId:       body.GithubId,
				Body:           body.Body,
				State:          body.State,
				CommitSha:      body.CommitId,
				GithubSubmitAt: body.SubmittedAt.ToNullableTime(),
				PullRequestId:  pull.GithubId,
			}
			if body.User != nil {
				githubReviewer.ReviewerId = body.User.Id
				githubReviewer.Username = body.User.Login
				githubPrReview.AuthorUserId = body.User.Id
				githubPrReview.AuthorUsername = body.User.Login
				githubUser, err := convertAccount(body.User, data.Options.GithubId, data.Options.ConnectionId)
				if err != nil {
					return nil, err
				}
				results = append(results, githubUser)
			}
			results = append(results, githubReviewer)
			results = append(results, githubPrReview)
			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
