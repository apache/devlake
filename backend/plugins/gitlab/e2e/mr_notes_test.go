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
	"testing"

	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/gitlab/impl"
	"github.com/apache/devlake/plugins/gitlab/models"
	"github.com/apache/devlake/plugins/gitlab/tasks"
)

func TestGitlabMrNoteDataFlow(t *testing.T) {

	var gitlab impl.Gitlab
	dataflowTester := e2ehelper.NewDataFlowTester(t, "gitlab", gitlab)

	taskData := &tasks.GitlabTaskData{
		Options: &tasks.GitlabOptions{
			ConnectionId: 1,
			ProjectId:    12345678,
			ScopeConfig:  new(models.GitlabScopeConfig),
		},
	}
	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_gitlab_api_merge_requests_for_mr_notes_test.csv",
		"_raw_gitlab_api_merge_requests")

	// verify extraction
	dataflowTester.FlushTabler(&models.GitlabMergeRequest{})
	dataflowTester.FlushTabler(&models.GitlabMrLabel{})
	dataflowTester.FlushTabler(&models.GitlabReviewer{})
	dataflowTester.FlushTabler(&models.GitlabAssignee{})
	dataflowTester.Subtask(tasks.ExtractApiMergeRequestsMeta, taskData)
	dataflowTester.VerifyTable(
		models.GitlabMergeRequest{},
		"./snapshot_tables/_tool_gitlab_merge_requests_for_mr_notes_test.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"gitlab_id",
			"iid",
			"project_id",
			"source_project_id",
			"target_project_id",
			"state",
			"title",
			"web_url",
			"user_notes_count",
			"work_in_progress",
			"source_branch",
			"target_branch",
			"merge_commit_sha",
			"merged_at",
			"gitlab_created_at",
			"closed_at",
			"merged_by_username",
			"description",
			"author_username",
			"author_user_id",
			"component",
			"first_comment_time",
			"review_rounds",
		),
	)
	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_gitlab_api_merge_request_notes.csv",
		"_raw_gitlab_api_merge_request_notes")

	// verify extraction
	dataflowTester.FlushTabler(&models.GitlabMrNote{})
	dataflowTester.FlushTabler(&models.GitlabMrComment{})
	dataflowTester.Subtask(tasks.ExtractApiMrNotesMeta, taskData)
	dataflowTester.VerifyTable(
		models.GitlabMrNote{},
		"./snapshot_tables/_tool_gitlab_mr_notes.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"gitlab_id",
			"merge_request_id",
			"merge_request_iid",
			"noteable_type",
			"author_username",
			"body",
			"gitlab_created_at",
			"confidential",
			"resolvable",
			"is_system",
		),
	)
	dataflowTester.VerifyTable(
		models.GitlabMrComment{},
		"./snapshot_tables/_tool_gitlab_mr_comments.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"gitlab_id",
			"merge_request_id",
			"merge_request_iid",
			"body",
			"author_username",
			"author_user_id",
			"gitlab_created_at",
			"resolvable",
		),
	)

	// verify conversion
	dataflowTester.FlushTabler(&code.PullRequestComment{})
	dataflowTester.Subtask(tasks.ConvertMrCommentMeta, taskData)
	dataflowTester.VerifyTable(
		code.PullRequestComment{},
		"./snapshot_tables/pull_request_comments.csv",
		e2ehelper.ColumnWithRawData(
			"id",
			"pull_request_id",
			"body",
			"account_id",
			"created_date",
			"commit_sha",
		),
	)
}
