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
	"fmt"
	"regexp"
	"strconv"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractApiPrReviewCommentsMeta)
}

var ExtractApiPrReviewCommentsMeta = plugin.SubTaskMeta{
	Name:             "Extract PR Review Comments",
	EntryPoint:       ExtractApiPrReviewComments,
	EnabledByDefault: true,
	Description: "Extract raw comment data  into tool layer table github_pull_request_comments" +
		"and github_issue_comments",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS, plugin.DOMAIN_TYPE_CODE_REVIEW},
	DependencyTables: []string{RAW_PR_REVIEW_COMMENTS_TABLE},
	ProductTables: []string{
		models.GithubRepoAccount{}.TableName(),
		models.GithubPrCommit{}.TableName()},
}

type GithubPrReviewCommentBody struct {
	GithubId        int `json:"id"`
	Body            json.RawMessage
	User            *GithubAccountResponse
	PrUrl           string             `json:"pull_request_url"`
	GithubCreatedAt common.Iso8601Time `json:"created_at"`
	GithubUpdatedAt common.Iso8601Time `json:"updated_at"`
	CommitId        string             `json:"commit_id"`
	PrReviewId      int                `json:"pull_request_review_id"`
}

func ExtractApiPrReviewComments(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)
	db := taskCtx.GetDal()

	prUrlPattern := fmt.Sprintf(`https\:\/\/api\.github\.com\/repos\/%s\/pulls\/(\d+)`, data.Options.Name)
	prUrlRegex, err := regexp.Compile(prUrlPattern)
	if err != nil {
		return errors.Default.Wrap(err, "regexp Compile prUrlPattern failed")
	}

	extractor, extErr := helper.NewStatefulApiExtractor(&helper.StatefulApiExtractorArgs[GithubPrReviewCommentBody]{
		SubtaskCommonArgs: &helper.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_PR_REVIEW_COMMENTS_TABLE,
		},
		Extract: func(body *GithubPrReviewCommentBody, row *helper.RawData) ([]any, errors.Error) {
			results := make([]interface{}, 0, 1)
			githubPrComment := &models.GithubPrComment{
				ConnectionId:    data.Options.ConnectionId,
				GithubId:        body.GithubId,
				Body:            string(body.Body),
				CommitSha:       body.CommitId,
				ReviewId:        body.PrReviewId,
				GithubCreatedAt: body.GithubCreatedAt.ToTime(),
				GithubUpdatedAt: body.GithubUpdatedAt.ToTime(),
				Type:            "DIFF",
			}
			prId, err := enrichGithubPrComment(data, db, prUrlRegex, body.PrUrl)
			if err != nil {
				return nil, errors.Default.Wrap(err, "parse prId failed")
			}
			if prId != 0 {
				githubPrComment.PullRequestId = prId
			}
			if body.User != nil {
				if shouldSkipByUsername(body.User.Login) {
					taskCtx.GetLogger().Debug("Skipping PR review comment #%d from bot user: %s", body.GithubId, body.User.Login)
					return nil, nil
				}
				githubPrComment.AuthorUserId = body.User.Id
				githubPrComment.AuthorUsername = body.User.Login
				githubAccount, err := convertAccount(body.User, data.Options.GithubId, data.Options.ConnectionId)
				if err != nil {
					return nil, err
				}
				results = append(results, githubAccount)
			}
			results = append(results, githubPrComment)
			return results, nil
		},
	})
	if extErr != nil {
		return extErr
	}
	return extractor.Execute()
}

func enrichGithubPrComment(data *GithubTaskData, db dal.Dal, prUrlRegex *regexp.Regexp, prUrl string) (int, errors.Error) {
	groups := prUrlRegex.FindStringSubmatch(prUrl)
	if len(groups) > 1 {
		prNumber, err := strconv.Atoi(groups[1])
		if err != nil {
			return 0, errors.Default.Wrap(err, "parse prId failed")
		}
		pr := &models.GithubPullRequest{}
		err1 := db.First(pr, dal.Where("connection_id = ? and number = ? and repo_id = ?", data.Options.ConnectionId, prNumber, data.Options.GithubId))
		if db.IsErrorNotFound(err1) {
			return 0, nil
		} else if err1 != nil {
			return 0, errors.NotFound.Wrap(err1, "github pull request parse failed ")
		}
		return pr.GithubId, nil
	}
	return 0, nil
}
