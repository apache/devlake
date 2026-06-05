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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	plugin "github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/bitbucket/models"
)

var ExtractApiPullRequestsMeta = plugin.SubTaskMeta{
	Name:             "Extract Pull Requests",
	EntryPoint:       ExtractApiPullRequests,
	EnabledByDefault: true,
	Required:         false,
	Description:      "Extract raw PullRequests data into tool layer table bitbucket_pull_requests",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
}

type BitbucketApiPullRequest struct {
	BitbucketId  int `json:"id"`
	CommentCount int `json:"comment_count"`
	//TaskCount    int    `json:"task_count"`
	Type        string `json:"type"`
	State       string `json:"state"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MergeCommit *struct {
		Hash string `json:"hash"`
		// date only return when fields defined
		Date *common.Iso8601Time `json:"date"`
	} `json:"merge_commit"`
	Links *struct {
		Html struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
	ClosedBy           *BitbucketAccountResponse `json:"closed_by"`
	Author             *BitbucketAccountResponse `json:"author"`
	BitbucketCreatedAt *common.Iso8601Time       `json:"created_on"`
	BitbucketUpdatedAt *common.Iso8601Time       `json:"updated_on"`
	BaseRef            *struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
		Repo *models.BitbucketApiRepo `json:"repository"`
	} `json:"destination"`
	HeadRef *struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
		Repo *models.BitbucketApiRepo `json:"repository"`
	} `json:"source"`
	Participants []BitbucketParticipant `json:"participants"`
}

type BitbucketParticipant struct {
	User           *BitbucketAccountResponse `json:"user"`
	Role           string                    `json:"role"`
	State          *string                   `json:"state"`
	Approved       bool                      `json:"approved"`
	ParticipatedOn *common.Iso8601Time       `json:"participated_on"`
}

func ExtractApiPullRequests(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RAW_PULL_REQUEST_TABLE)
	var err errors.Error
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			rawL := &BitbucketApiPullRequest{}
			err := errors.Convert(json.Unmarshal(row.Data, rawL))
			if err != nil {
				return nil, err
			}
			// need to extract 2 kinds of entities here
			results := make([]interface{}, 0, 1)
			if rawL.BitbucketId == 0 {
				return nil, nil
			}

			bitbucketPr, err := convertBitbucketPullRequest(rawL, data.Options.ConnectionId, data.Options.FullName)
			if err != nil {
				return nil, err
			}
			if rawL.Author != nil {
				bitbucketUser, err := convertAccount(rawL.Author, data.Options.ConnectionId)
				if err != nil {
					return nil, err
				}
				results = append(results, bitbucketUser)
				bitbucketPr.AuthorName = bitbucketUser.DisplayName
				bitbucketPr.AuthorId = bitbucketUser.AccountId
			}
			if rawL.MergeCommit != nil {
				bitbucketPr.MergeCommitSha = rawL.MergeCommit.Hash
				bitbucketPr.MergedAt = rawL.MergeCommit.Date.ToNullableTime()
			}
			results = append(results, bitbucketPr)

			// Extract participants/reviewers
			for _, participant := range rawL.Participants {
				if participant.User == nil {
					continue
				}
				reviewer := &models.BitbucketPrReviewer{
					ConnectionId:  data.Options.ConnectionId,
					RepoId:        data.Options.FullName,
					PullRequestId: rawL.BitbucketId,
					AccountId:     participant.User.AccountId,
					DisplayName:   participant.User.DisplayName,
					Role:          participant.Role,
					Approved:      participant.Approved,
				}
				if participant.State != nil {
					reviewer.State = *participant.State
				}
				if participant.ParticipatedOn != nil {
					reviewer.ParticipatedOn = participant.ParticipatedOn.ToNullableTime()
				}
				results = append(results, reviewer)

				// Also save the user account
				bitbucketUser, err := convertAccount(participant.User, data.Options.ConnectionId)
				if err != nil {
					return nil, err
				}
				results = append(results, bitbucketUser)
			}

			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
func convertBitbucketPullRequest(pull *BitbucketApiPullRequest, connId uint64, repoId string) (*models.BitbucketPullRequest, errors.Error) {
	bitbucketPull := &models.BitbucketPullRequest{
		ConnectionId:       connId,
		BitbucketId:        pull.BitbucketId,
		Number:             pull.BitbucketId,
		RepoId:             repoId,
		State:              pull.State,
		Title:              pull.Title,
		Description:        pull.Description,
		Url:                pull.Links.Html.Href,
		Type:               pull.Type,
		CommentCount:       pull.CommentCount,
		BitbucketCreatedAt: pull.BitbucketCreatedAt.ToTime(),
		BitbucketUpdatedAt: pull.BitbucketUpdatedAt.ToTime(),
	}
	if pull.BaseRef != nil {
		if pull.BaseRef.Repo != nil {
			bitbucketPull.BaseRepoId = pull.BaseRef.Repo.FullName
		}
		bitbucketPull.BaseRef = pull.BaseRef.Branch.Name
		bitbucketPull.BaseCommitSha = pull.BaseRef.Commit.Hash
	}
	if pull.HeadRef != nil {
		if pull.HeadRef.Repo != nil {
			bitbucketPull.HeadRepoId = pull.HeadRef.Repo.FullName
		}
		bitbucketPull.HeadRef = pull.HeadRef.Branch.Name
		bitbucketPull.HeadCommitSha = pull.HeadRef.Commit.Hash
	}
	if pull.ClosedBy != nil {
		bitbucketPull.MergedByName = pull.ClosedBy.DisplayName
		bitbucketPull.MergedById = pull.ClosedBy.AccountId
	}

	return bitbucketPull, nil
}
