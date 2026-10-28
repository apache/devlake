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
	"fmt"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

var CollectCommitsMeta = plugin.SubTaskMeta{
	Name:             "CollectCommits",
	EntryPoint:       CollectCommits,
	EnabledByDefault: true,
	Description:      "Collect commits data from Codecov API for main/master branch",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

// CommitResponse represents a single commit from the Codecov API
type CommitResponse struct {
	CommitID  string `json:"commitid"`
	Branch    string `json:"branch"`
	Message   string `json:"message"`
	Parent    string `json:"parent"`
	Timestamp string `json:"timestamp"`
	Author    struct {
		Name string `json:"name"`
	} `json:"author"`
}

func CollectCommits(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	logger := taskCtx.GetLogger()
	db := taskCtx.GetDal()

	// Extract owner and repo from FullName
	owner, repo, err := ParseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	// Use branch from repo scope, falling back to "main"
	branch := "main"
	if data.Repo != nil && data.Repo.Branch != "" {
		branch = data.Repo.Branch
	}

	// Use sync policy time range, default to last 90 days
	var cutoffDate time.Time
	syncPolicy := taskCtx.TaskContext().SyncPolicy()
	if syncPolicy != nil && syncPolicy.TimeAfter != nil {
		// Truncate to start of day to include all commits from the cutoff day
		t := *syncPolicy.TimeAfter
		cutoffDate = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		logger.Info("[Codecov] Collecting commits for %s/%s branch=%s from %s (sync policy, inclusive)", owner, repo, branch, cutoffDate.Format("2006-01-02"))
	} else {
		// Default to last 90 days if no sync policy
		now := time.Now()
		cutoffDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -90)
		logger.Info("[Codecov] Collecting commits for %s/%s branch=%s from %s (default 90 days, inclusive)", owner, repo, branch, cutoffDate.Format("2006-01-02"))
	}

	// Manual pagination with early termination
	// Codecov API doesn't support start_date filter, so we paginate until we hit old commits
	apiClient := data.ApiClient
	pageSize := 100
	page := 1
	totalCollected := 0
	stopPagination := false

	for !stopPagination {
		// Build the request URL
		reqUrl := fmt.Sprintf("%s/commits?branch=%s&page=%d&page_size=%d",
			RepoAPIPrefix(data.Service, owner, repo), branch, page, pageSize)

		res, err := apiClient.Get(reqUrl, nil, nil)
		if err != nil {
			return errors.Default.Wrap(err, fmt.Sprintf("failed to fetch commits page %d", page))
		}

		var response struct {
			Count      int              `json:"count"`
			Next       *string          `json:"next"`
			Results    []CommitResponse `json:"results"`
			TotalPages int              `json:"total_pages"`
		}

		if err := helper.UnmarshalResponse(res, &response); err != nil {
			return errors.Default.Wrap(err, "failed to parse commits response")
		}

		if len(response.Results) == 0 {
			logger.Info("[Codecov] No more commits found on page %d", page)
			break
		}

		// Process each commit
		for _, commit := range response.Results {
			// Parse timestamp
			var commitTimestamp *time.Time
			if commit.Timestamp != "" {
				parsed, parseErr := time.Parse(time.RFC3339, commit.Timestamp)
				if parseErr == nil {
					commitTimestamp = &parsed

					// Check if this commit is older than our cutoff date
					if parsed.Before(cutoffDate) {
						logger.Info("[Codecov] Reached commit older than cutoff (%s < %s), stopping pagination",
							parsed.Format("2006-01-02"), cutoffDate.Format("2006-01-02"))
						stopPagination = true
						break
					}
				}
			}

			// Save commit to database
			codecovCommit := &models.CodecovCommit{
				NoPKModel:       common.NoPKModel{},
				ConnectionId:    data.Options.ConnectionId,
				RepoId:          data.Options.FullName,
				CommitSha:       commit.CommitID,
				Branch:          commit.Branch,
				CommitTimestamp: commitTimestamp,
				Message:         commit.Message,
				Author:          commit.Author.Name,
				ParentSha:       commit.Parent,
			}

			// Upsert: PK is (connection_id, repo_id, commit_sha) so ON CONFLICT fires on duplicates
			if err := db.CreateOrUpdate(codecovCommit); err != nil {
				logger.Warn(err, "failed to save commit %s", commit.CommitID)
			} else {
				totalCollected++
			}
		}

		// Check if there are more pages
		if response.Next == nil || stopPagination {
			break
		}

		page++
		logger.Info("[Codecov] Processed page %d, collected %d commits so far", page-1, totalCollected)
	}

	logger.Info("[Codecov] Finished collecting commits: %d total commits within sync policy window", totalCollected)
	return nil
}
