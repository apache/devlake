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
	"regexp"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractApiPullRequestsMeta)
}

var ExtractApiPullRequestsMeta = plugin.SubTaskMeta{
	Name:             "Extract Pull Requests",
	EntryPoint:       ExtractApiPullRequests,
	EnabledByDefault: true,
	Description:      "Extract raw PullRequests data into tool layer table github_pull_requests",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS, plugin.DOMAIN_TYPE_CODE_REVIEW},
	DependencyTables: []string{RAW_PULL_REQUEST_TABLE},
	ProductTables: []string{
		models.GithubRepoAccount{}.TableName(),
		models.GithubPrLabel{}.TableName(),
		models.GithubPullRequest{}.TableName()},
}

type GithubApiPullRequest struct {
	GithubId int             `json:"id"`
	Number   int             `json:"number"`
	State    string          `json:"state"`
	Title    string          `json:"title"`
	Body     json.RawMessage `json:"body"`
	HtmlUrl  string          `json:"html_url"`
	Labels   []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignee        *GithubAccountResponse `json:"assignee"`
	User            *GithubAccountResponse `json:"user"`
	ClosedAt        *common.Iso8601Time    `json:"closed_at"`
	MergedAt        *common.Iso8601Time    `json:"merged_at"`
	GithubCreatedAt common.Iso8601Time     `json:"created_at"`
	GithubUpdatedAt common.Iso8601Time     `json:"updated_at"`
	MergeCommitSha  string                 `json:"merge_commit_sha"`
	Merged          bool                   `json:"merged"`
	Additions       int                    `json:"additions"`
	Deletions       int                    `json:"deletions"`
	ChangedFiles    int                    `json:"changed_files"`
	Comments        int                    `json:"comments"`
	ReviewComments  int                    `json:"review_comments"`
	Commits         int                    `json:"commits"`
	IsDraft         bool                   `json:"draft"`
	MergedBy        *GithubAccountResponse `json:"merged_by"`
	Head            struct {
		Ref  string         `json:"ref"`
		Sha  string         `json:"sha"`
		Repo *GithubApiRepo `json:"repo"`
	} `json:"head"`
	Base struct {
		Ref  string         `json:"ref"`
		Sha  string         `json:"sha"`
		Repo *GithubApiRepo `json:"repo"`
	} `json:"base"`
}

func ExtractApiPullRequests(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)
	db := taskCtx.GetDal()
	config := data.Options.ScopeConfig
	var labelTypeRegex *regexp.Regexp
	var labelComponentRegex *regexp.Regexp
	var prType = config.PrType
	var err error
	if len(prType) > 0 {
		labelTypeRegex, err = regexp.Compile(prType)
		if err != nil {
			return errors.Default.Wrap(err, "regexp Compile prType failed")
		}
	}
	var prComponent = config.PrComponent
	if len(prComponent) > 0 {
		labelComponentRegex, err = regexp.Compile(prComponent)
		if err != nil {
			return errors.Default.Wrap(err, "regexp Compile prComponent failed")
		}
	}

	extractor, extErr := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[GithubApiPullRequest]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_PULL_REQUEST_TABLE,
			SubtaskConfig: map[string]string{
				"prType":      prType,
				"prComponent": prComponent,
			},
		},
		BeforeExtract: func(body *GithubApiPullRequest, stateManager *api.SubtaskStateManager) errors.Error {
			if stateManager.IsIncremental() {
				return errors.Convert(db.Delete(
					&models.GithubPrLabel{},
					dal.Where("connection_id = ? AND pull_id = ?", data.Options.ConnectionId, body.GithubId),
				))
			}
			return nil
		},
		Extract: func(body *GithubApiPullRequest, row *api.RawData) ([]any, errors.Error) {
			results := make([]interface{}, 0, 1)
			if body.GithubId == 0 {
				return nil, nil
			}
			// Filter bot PRs by username
			if body.User != nil && shouldSkipByUsername(body.User.Login) {
				taskCtx.GetLogger().Debug("Skipping PR #%d from bot user: %s", body.Number, body.User.Login)
				return nil, nil
			}
			githubPr, err := convertGithubPullRequest(body, data.Options.ConnectionId, data.Options.GithubId)
			if err != nil {
				return nil, err
			}
			if body.User != nil {
				githubUser, err := convertAccount(body.User, data.Options.GithubId, data.Options.ConnectionId)
				if err != nil {
					return nil, err
				}
				results = append(results, githubUser)
				githubPr.AuthorName = githubUser.Login
				githubPr.AuthorId = githubUser.AccountId
			}
			for _, label := range body.Labels {
				results = append(results, &models.GithubPrLabel{
					ConnectionId: data.Options.ConnectionId,
					PullId:       githubPr.GithubId,
					LabelName:    label.Name,
				})
				if labelTypeRegex != nil && labelTypeRegex.MatchString(label.Name) {
					githubPr.Type = label.Name
				}
				if labelComponentRegex != nil && labelComponentRegex.MatchString(label.Name) {
					githubPr.Component = label.Name
				}
			}
			results = append(results, githubPr)
			return results, nil
		},
	})

	if extErr != nil {
		return errors.Default.Wrap(extErr, "error initializing Github PR extractor")
	}

	return extractor.Execute()
}

func convertGithubPullRequest(pull *GithubApiPullRequest, connId uint64, repoId int) (*models.GithubPullRequest, errors.Error) {
	githubPull := &models.GithubPullRequest{
		ConnectionId:    connId,
		GithubId:        pull.GithubId,
		RepoId:          repoId,
		Number:          pull.Number,
		State:           pull.State,
		Title:           pull.Title,
		Url:             pull.HtmlUrl,
		GithubCreatedAt: pull.GithubCreatedAt.ToTime(),
		GithubUpdatedAt: pull.GithubUpdatedAt.ToTime(),
		ClosedAt:        common.Iso8601TimeToTime(pull.ClosedAt),
		MergedAt:        common.Iso8601TimeToTime(pull.MergedAt),
		MergeCommitSha:  pull.MergeCommitSha,
		Body:            string(pull.Body),
		BaseRef:         pull.Base.Ref,
		BaseCommitSha:   pull.Base.Sha,
		HeadRef:         pull.Head.Ref,
		HeadCommitSha:   pull.Head.Sha,
		Merged:          pull.Merged,
		Additions:       pull.Additions,
		Deletions:       pull.Deletions,
		Comments:        pull.Comments,
		ReviewComments:  pull.ReviewComments,
		Commits:         pull.Commits,
		IsDraft:         pull.IsDraft,
	}
	if pull.Head.Repo != nil {
		githubPull.HeadRepoId = pull.Head.Repo.GithubId
	}
	if pull.MergedBy != nil {
		githubPull.MergedByName = pull.MergedBy.Login
		githubPull.MergedById = pull.MergedBy.Id
	}

	return githubPull, nil
}
