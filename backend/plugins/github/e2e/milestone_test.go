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
	"github.com/apache/devlake/plugins/github/impl"
	"github.com/apache/devlake/plugins/github/models"
	"github.com/apache/devlake/plugins/github/tasks"
)

func TestMilestoneDataFlow(t *testing.T) {
	var plugin impl.Github
	dataflowTester := e2ehelper.NewDataFlowTester(t, "github", plugin)

	taskData := &tasks.GithubTaskData{
		Options: &tasks.GithubOptions{
			ConnectionId: 1,
			Name:         "panjf2000/ants",
			GithubId:     134018330,
			ScopeConfig: &models.GithubScopeConfig{
				PrType:               "type/(.*)$",
				PrComponent:          "component/(.*)$",
				PrBodyClosePattern:   "(?mi)(fix|close|resolve|fixes|closes|resolves|fixed|closed|resolved)[\\s]*.*(((and )?(#|https:\\/\\/github.com\\/%s\\/issues\\/)\\d+[ ]*)+)",
				IssueSeverity:        "severity/(.*)$",
				IssuePriority:        "^(highest|high|medium|low)$",
				IssueComponent:       "component/(.*)$",
				IssueTypeBug:         "^(bug|failure|error)$",
				IssueTypeIncident:    "",
				IssueTypeRequirement: "^(feat|feature|proposal|requirement)$",
			},
		},
	}

	dataflowTester.FlushTabler(&models.GithubMilestone{})
	dataflowTester.FlushTabler(&models.GithubIssue{})
	dataflowTester.FlushTabler(&models.GithubIssueLabel{})
	dataflowTester.FlushTabler(&models.GithubRepoAccount{})
	dataflowTester.FlushTabler(&models.GithubIssueAssignee{})

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_github_api_milestones.csv", "_raw_"+tasks.RAW_MILESTONE_TABLE)
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_github_api_issues.csv", "_raw_"+tasks.RAW_ISSUE_TABLE)

	dataflowTester.Subtask(tasks.ExtractApiIssuesMeta, taskData)
	dataflowTester.Subtask(tasks.ExtractMilestonesMeta, taskData)
	dataflowTester.VerifyTable(
		models.GithubMilestone{},
		"./snapshot_tables/_tool_github_milestones.csv",
		[]string{
			"connection_id",
			"milestone_id",
			"repo_id",
			"number",
			"url",
			"title",
			"open_issues",
			"closed_issues",
			"state",
			"closed_at",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)

	dataflowTester.FlushTabler(&ticket.Sprint{})
	dataflowTester.FlushTabler(&ticket.BoardSprint{})
	dataflowTester.FlushTabler(&ticket.SprintIssue{})

	dataflowTester.Subtask(tasks.ConvertMilestonesMeta, taskData)
	dataflowTester.VerifyTable(
		ticket.Sprint{},
		"./snapshot_tables/sprints.csv",
		[]string{
			"id",
			"name",
			"url",
			"status",
			"started_date",
			"ended_date",
			"completed_date",
			"original_board_id",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)

	dataflowTester.VerifyTable(
		ticket.BoardSprint{},
		"./snapshot_tables/board_sprint.csv",
		[]string{
			"board_id",
			"sprint_id",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)

	dataflowTester.VerifyTable(
		ticket.SprintIssue{},
		"./snapshot_tables/sprint_issue.csv",
		[]string{
			"issue_id",
			"sprint_id",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)
}
