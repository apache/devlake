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

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/tapd/impl"
	"github.com/apache/devlake/plugins/tapd/models"
	"github.com/apache/devlake/plugins/tapd/tasks"
)

func TestTapdBugDataFlow(t *testing.T) {

	var tapd impl.Tapd
	dataflowTester := e2ehelper.NewDataFlowTester(t, "tapd", tapd)

	taskData := &tasks.TapdTaskData{
		Options: &tasks.TapdOptions{
			ConnectionId: 1,
			WorkspaceId:  991,
			ScopeConfig: &models.TapdScopeConfig{
				TypeMappings: map[string]string{
					"BUG":  "缺陷",
					"TASK": "任务",
				},
				StatusMappings: map[string]string{
					"已关闭":   "完成",
					"接受/处理": "处理中",
				},
			},
		},
	}

	// bug status
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_tapd_bug_statuses.csv", &models.TapdBugStatus{})

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_tapd_api_bugs.csv", "_raw_tapd_api_bugs")

	// verify extraction
	dataflowTester.FlushTabler(&models.TapdBug{})
	dataflowTester.FlushTabler(&models.TapdWorkSpaceBug{})
	dataflowTester.FlushTabler(&models.TapdIterationBug{})
	dataflowTester.FlushTabler(&models.TapdBugLabel{})
	dataflowTester.Subtask(tasks.ExtractBugMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&models.TapdBug{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_tapd_bugs.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTable(
		models.TapdWorkSpaceBug{},
		"./snapshot_tables/_tool_tapd_workspace_bugs.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"workspace_id",
			"bug_id",
		),
	)
	dataflowTester.VerifyTable(
		models.TapdIterationBug{},
		"./snapshot_tables/_tool_tapd_iteration_bugs.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"workspace_id",
			"iteration_id",
			"bug_id",
			"resolution_date",
			"bug_created_date",
		),
	)
	dataflowTester.VerifyTable(
		models.TapdBugLabel{},
		"./snapshot_tables/_tool_tapd_bug_labels.csv",
		e2ehelper.ColumnWithRawData(
			"label_name",
			"bug_id",
		),
	)

	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.SprintIssue{})
	dataflowTester.FlushTabler(&ticket.IssueLabel{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.Subtask(tasks.ConvertBugMeta, taskData)
	dataflowTester.VerifyTable(
		ticket.Issue{},
		"./snapshot_tables/issues_bug.csv",
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
			"due_date",
		),
	)
	dataflowTester.VerifyTable(
		ticket.BoardIssue{},
		"./snapshot_tables/board_issues_bug.csv",
		e2ehelper.ColumnWithRawData(
			"board_id",
			"issue_id",
		),
	)
	dataflowTester.VerifyTable(
		ticket.SprintIssue{},
		"./snapshot_tables/sprint_issues_bug.csv",
		e2ehelper.ColumnWithRawData(
			"issue_id",
			"sprint_id",
		),
	)
	dataflowTester.VerifyTableWithOptions(ticket.IssueAssignee{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/bug_issue_assignees.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.Subtask(tasks.ConvertBugLabelsMeta, taskData)
	dataflowTester.VerifyTable(
		ticket.IssueLabel{},
		"./snapshot_tables/issue_labels_bug.csv",
		e2ehelper.ColumnWithRawData(
			"issue_id",
			"label_name",
		),
	)
}

func TestTapdBugCustomizeDueDate(t *testing.T) {
	var tapd impl.Tapd
	dataflowTester := e2ehelper.NewDataFlowTester(t, "tapd", tapd)

	taskData := &tasks.TapdTaskData{
		Options: &tasks.TapdOptions{
			ConnectionId: 1,
			WorkspaceId:  991,
			ScopeConfig: &models.TapdScopeConfig{
				TypeMappings: map[string]string{
					"BUG":  "缺陷",
					"TASK": "任务",
				},
				StatusMappings: map[string]string{
					"已关闭":   "完成",
					"接受/处理": "处理中",
				},
				BugDueDateField: "custom_field_four",
			},
		},
	}

	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_tapd_bug_statuses.csv", &models.TapdBugStatus{})
	dataflowTester.FlushTabler(&models.TapdBug{})
	dataflowTester.FlushTabler(&models.TapdWorkSpaceBug{})
	dataflowTester.FlushTabler(&models.TapdIterationBug{})
	dataflowTester.FlushTabler(&models.TapdBugLabel{})
	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_tapd_api_bugs_for_due_date.csv", "_raw_tapd_api_bugs")
	dataflowTester.Subtask(tasks.ExtractBugMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&models.TapdBug{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_tapd_bugs_for_due_date.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.SprintIssue{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.Subtask(tasks.ConvertBugMeta, taskData)
	dataflowTester.VerifyTable(
		ticket.Issue{},
		"./snapshot_tables/issues_bug_for_due_date.csv",
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
			"due_date",
		),
	)
}
