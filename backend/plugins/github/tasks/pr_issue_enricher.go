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
	"regexp"
	"strconv"
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&EnrichPullRequestIssuesMeta)
}

var EnrichPullRequestIssuesMeta = plugin.SubTaskMeta{
	Name:             "Enrich PR Issues",
	EntryPoint:       EnrichPullRequestIssues,
	EnabledByDefault: true,
	Description:      "Create tool layer table github_pull_request_issues from github_pull_requests",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{
		models.GithubPullRequest{}.TableName(), // cursor
		RAW_PULL_REQUEST_TABLE},
	ProductTables: []string{models.GithubPrIssue{}.TableName()},
}

func EnrichPullRequestIssues(taskCtx plugin.SubTaskContext) (err errors.Error) {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	var prBodyCloseRegex *regexp.Regexp
	prBodyClosePattern := data.Options.ScopeConfig.PrBodyClosePattern
	//the pattern before the issue number, sometimes, the issue number is #1098, sometimes it is https://xxx/#1098
	prBodyClosePattern = strings.Replace(prBodyClosePattern, "%s", data.Options.Name, 1)
	if len(prBodyClosePattern) > 0 {
		prBodyCloseRegex, err = errors.Convert01(regexp.Compile(prBodyClosePattern))
		if err != nil {
			return errors.Default.Wrap(err, "regexp Compile prBodyClosePattern failed")
		}
	}
	charPattern := regexp.MustCompile(`[\/a-zA-Z\s,]+`)

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubPullRequest]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_PULL_REQUEST_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			SubtaskConfig: map[string]string{
				"prBodyClosePattern": prBodyClosePattern,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.From(&models.GithubPullRequest{}),
				dal.Where("repo_id = ? and connection_id = ?", repoId, data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("github_updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		BeforeConvert: func(githubPullRequest *models.GithubPullRequest, stateManager *api.SubtaskStateManager) errors.Error {
			if !stateManager.IsIncremental() {
				return nil
			}
			return db.Delete(
				&models.GithubPrIssue{},
				dal.Where("connection_id = ? AND pull_request_id = ?", data.Options.ConnectionId, githubPullRequest.GithubId),
			)
		},
		Convert: func(githubPullRequst *models.GithubPullRequest) ([]interface{}, errors.Error) {
			results := make([]interface{}, 0, 1)

			//find the matched string in body
			issueNumberListStr := ""

			if prBodyCloseRegex != nil {
				issueNumberListStr = prBodyCloseRegex.FindString(githubPullRequst.Body)
			}

			if issueNumberListStr == "" {
				return nil, nil
			}

			issueNumberListStr = charPattern.ReplaceAllString(issueNumberListStr, "#")
			//split the string by '#'
			issueNumberList := strings.Split(issueNumberListStr, "#")

			for _, issueNumberStr := range issueNumberList {
				issue := &models.GithubIssue{}
				issueNumberStr = strings.TrimSpace(issueNumberStr)
				//change the issueNumberStr to int, if cannot be changed, just continue
				issueNumber, numFormatErr := strconv.Atoi(issueNumberStr)
				if numFormatErr != nil {
					continue
				}
				err = db.All(
					issue,
					dal.Where("number = ? and repo_id = ? and connection_id = ?",
						issueNumber, repoId, data.Options.ConnectionId),
					dal.Limit(1),
				)
				if err != nil {
					return nil, err
				}
				if issue.Number == 0 {
					continue
				}
				githubPullRequstIssue := &models.GithubPrIssue{
					ConnectionId:      data.Options.ConnectionId,
					PullRequestId:     githubPullRequst.GithubId,
					IssueId:           issue.GithubId,
					PullRequestNumber: githubPullRequst.Number,
					IssueNumber:       issue.Number,
				}
				results = append(results, githubPullRequstIssue)
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
