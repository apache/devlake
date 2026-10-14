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

package e2e

import (
	"os"
	"testing"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/github/impl"
	"github.com/apache/devlake/plugins/github/models"
	"github.com/apache/devlake/plugins/github/tasks"
)

func TestPrReviewDataFlow(t *testing.T) {
	var plugin impl.Github
	dataflowTester := e2ehelper.NewDataFlowTester(t, "github", plugin)

	taskData := &tasks.GithubTaskData{
		Options: &tasks.GithubOptions{
			ConnectionId: 1,
			Name:         "panjf2000/ants",
			GithubId:     134018330,
		},
	}

	// import raw data table
	dataflowTester.ImportCsvIntoTabler("./raw_tables/_tool_github_pull_requests.csv", models.GithubPullRequest{})
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_github_api_pull_request_reviews.csv", "_raw_github_api_pull_request_reviews")

	// verify extraction
	dataflowTester.FlushTabler(&models.GithubReviewer{})
	dataflowTester.FlushTabler(&models.GithubPrReview{})
	dataflowTester.FlushTabler(&models.GithubRepoAccount{})
	dataflowTester.Subtask(tasks.ExtractApiPullRequestReviewsMeta, taskData)
	dataflowTester.VerifyTable(
		models.GithubPrReview{},
		"./snapshot_tables/_tool_github_pull_request_reviews.csv",
		[]string{
			"connection_id",
			"github_id",
			"pull_request_id",
			"body",
			"author_username",
			"author_user_id",
			"github_submit_at",
			"commit_sha",
			"state",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)
	dataflowTester.VerifyTable(
		models.GithubReviewer{},
		"./snapshot_tables/_tool_github_reviewers.csv",
		[]string{
			"connection_id",
			"reviewer_id",
			"pull_request_id",
			"username",
			"name",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)

	dataflowTester.FlushTabler(&code.PullRequestComment{})
	dataflowTester.Subtask(tasks.ConvertPullRequestReviewsMeta, taskData)
	dataflowTester.VerifyTable(
		code.PullRequestComment{},
		"./snapshot_tables/pull_request_review_comments.csv",
		[]string{
			"pull_request_id",
			"body",
			"account_id",
			"created_date",
			"commit_sha",
			"type",
			"review_id",
			"status",
		},
	)
}

func TestPrReviewDataFlowWithBotFiltering(t *testing.T) {
	var plugin impl.Github
	dataflowTester := e2ehelper.NewDataFlowTester(t, "github", plugin)

	// Set up bot filtering
	tasks.ResetExcludedUsernamesForTest()
	os.Setenv("GITHUB_PR_EXCLUDELIST", "renovate[bot]")
	defer os.Unsetenv("GITHUB_PR_EXCLUDELIST")

	taskData := &tasks.GithubTaskData{
		Options: &tasks.GithubOptions{
			ConnectionId: 1,
			Name:         "test/repo",
			GithubId:     123,
		},
	}

	// import raw data table with bot and human reviews
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_github_api_pr_reviews_bot_filter.csv", "_raw_github_api_pull_request_reviews")

	// verify review extraction filters bot reviews
	dataflowTester.FlushTabler(&models.GithubPrReview{})
	dataflowTester.FlushTabler(&models.GithubReviewer{})
	dataflowTester.FlushTabler(&models.GithubRepoAccount{})
	dataflowTester.Subtask(tasks.ExtractApiPullRequestReviewsMeta, taskData)

	// Verify only human review was extracted
	var reviews []models.GithubPrReview
	dataflowTester.Dal.All(&reviews, dal.Where("connection_id = ?", 1))

	if len(reviews) != 1 {
		t.Errorf("Expected 1 review (human), got %d", len(reviews))
	}
	if len(reviews) > 0 && reviews[0].GithubId != 5002 {
		t.Errorf("Expected review #5002 (human), got #%d", reviews[0].GithubId)
	}
}
