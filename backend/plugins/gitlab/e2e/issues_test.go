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

	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/gitlab/impl"
	"github.com/apache/devlake/plugins/gitlab/models"
	"github.com/apache/devlake/plugins/gitlab/tasks"
)

func TestGitlabIssueDataFlow(t *testing.T) {

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
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_gitlab_api_issues.csv",
		"_raw_gitlab_api_issues")

	// verify extraction
	dataflowTester.FlushTabler(&models.GitlabIssue{})
	dataflowTester.FlushTabler(&models.GitlabAccount{})
	dataflowTester.FlushTabler(&models.GitlabIssueLabel{})
	dataflowTester.FlushTabler(&models.GitlabIssueAssignee{})
	dataflowTester.Subtask(tasks.ExtractApiIssuesMeta, taskData)
	dataflowTester.VerifyTable(
		models.GitlabIssue{},
		"./snapshot_tables/_tool_gitlab_issues.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"gitlab_id",
			"project_id",
			"number",
			"state",
			"title",
			"body",
			"priority",
			"type",
			"status",
			"assignee_id",
			"assignee_name",
			"creator_id",
			"creator_name",
			"lead_time_minutes",
			"url",
			"closed_at",
			"gitlab_created_at",
			"gitlab_updated_at",
			"severity",
			"component",
			"time_estimate",
			"total_time_spent",
		),
	)

	dataflowTester.VerifyTable(
		models.GitlabIssueLabel{},
		"./snapshot_tables/_tool_gitlab_issue_labels.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"issue_id",
			"label_name",
		),
	)

	// verify conversion
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.IssueLabel{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.Subtask(tasks.ConvertIssuesMeta, taskData)
	dataflowTester.VerifyTable(
		ticket.Issue{},
		"./snapshot_tables/issues.csv",
		e2ehelper.ColumnWithRawData(
			"id",
			"url",
			"issue_key",
			"title",
			"description",
			"epic_key",
			"type",
			"status",
			"original_status",
			"story_point",
			"resolution_date",
			"created_date",
			"updated_date",
			"lead_time_minutes",
			"parent_issue_id",
			"priority",
			"original_estimate_minutes",
			"time_spent_minutes",
			"time_remaining_minutes",
			"creator_id",
			"assignee_id",
			"assignee_name",
			"severity",
			"component",
			"icon_url",
			"creator_name",
		),
	)

	dataflowTester.VerifyTable(
		&ticket.BoardIssue{},
		"./snapshot_tables/board_issues.csv",
		e2ehelper.ColumnWithRawData(
			"board_id",
			"issue_id",
		),
	)
	// verify conversion
	dataflowTester.FlushTabler(&ticket.IssueLabel{})
	dataflowTester.Subtask(tasks.ConvertIssueLabelsMeta, taskData)
	dataflowTester.VerifyTable(
		ticket.IssueLabel{},
		"./snapshot_tables/issue_labels.csv",
		e2ehelper.ColumnWithRawData(
			"issue_id",
			"label_name",
		),
	)
}
