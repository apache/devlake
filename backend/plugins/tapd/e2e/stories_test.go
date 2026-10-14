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

func TestTapdStoryDataFlow(t *testing.T) {

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
					"需求":   "故事需求",
					"技术债":  "技术需求债务",
					"长篇故事": "Epic需求",
				},
			},
		},
	}

	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_tapd_workitem_types.csv", &models.TapdWorkitemType{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_tapd_story_statuses.csv", &models.TapdStoryStatus{})

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_tapd_api_stories.csv",
		"_raw_tapd_api_stories")

	// verify extraction
	dataflowTester.FlushTabler(&models.TapdStory{})
	dataflowTester.FlushTabler(&models.TapdWorkSpaceStory{})
	dataflowTester.FlushTabler(&models.TapdIterationStory{})
	dataflowTester.FlushTabler(&models.TapdStoryLabel{})
	dataflowTester.Subtask(tasks.ExtractStoryMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&models.TapdStory{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_tapd_stories.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	dataflowTester.VerifyTable(
		models.TapdWorkSpaceStory{},
		"./snapshot_tables/_tool_tapd_workspace_stories.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"workspace_id",
			"story_id",
		),
	)
	dataflowTester.VerifyTable(
		models.TapdIterationStory{},
		"./snapshot_tables/_tool_tapd_iteration_stories.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"workspace_id",
			"iteration_id",
			"story_id",
			"resolution_date",
			"story_created_date",
		),
	)
	dataflowTester.VerifyTable(
		models.TapdStoryLabel{},
		"./snapshot_tables/_tool_tapd_story_labels.csv",
		e2ehelper.ColumnWithRawData(
			"label_name",
			"story_id",
		),
	)

	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.SprintIssue{})
	dataflowTester.FlushTabler(&ticket.IssueLabel{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.Subtask(tasks.ConvertStoryMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&ticket.Issue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issues_story.csv",
		IgnoreTypes: []interface{}{common.Model{}},
	})

	dataflowTester.VerifyTable(
		ticket.BoardIssue{},
		"./snapshot_tables/board_issues_story.csv",
		e2ehelper.ColumnWithRawData(
			"board_id",
			"issue_id",
		),
	)
	dataflowTester.VerifyTable(
		ticket.SprintIssue{},
		"./snapshot_tables/sprint_issues_story.csv",
		e2ehelper.ColumnWithRawData(
			"issue_id",
			"sprint_id",
		),
	)
	dataflowTester.VerifyTableWithOptions(ticket.IssueAssignee{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/story_issue_assignees.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	dataflowTester.Subtask(tasks.ConvertStoryLabelsMeta, taskData)
	dataflowTester.VerifyTable(
		ticket.IssueLabel{},
		"./snapshot_tables/issue_labels_story.csv",
		e2ehelper.ColumnWithRawData(
			"issue_id",
			"label_name",
		),
	)
}

func TestTapdStoryCustomizeDueDate(t *testing.T) {
	var tapd impl.Tapd
	dataflowTester := e2ehelper.NewDataFlowTester(t, "tapd", tapd)
	taskData := &tasks.TapdTaskData{
		Options: &tasks.TapdOptions{
			ConnectionId: 1,
			WorkspaceId:  991,
			ScopeConfig: &models.TapdScopeConfig{
				StoryDueDateField: "custom_field_three",
				TypeMappings: map[string]string{
					"BUG":  "缺陷",
					"TASK": "任务",
					"需求":   "故事需求",
					"技术债":  "技术需求债务",
					"长篇故事": "Epic需求",
				},
			},
		},
	}
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_tapd_workitem_types.csv", &models.TapdWorkitemType{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_tapd_story_statuses.csv", &models.TapdStoryStatus{})
	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_tapd_api_stories_for_due_date.csv",
		"_raw_tapd_api_stories")
	dataflowTester.FlushTabler(&models.TapdStory{})
	dataflowTester.FlushTabler(&models.TapdWorkSpaceStory{})
	dataflowTester.FlushTabler(&models.TapdIterationStory{})
	dataflowTester.FlushTabler(&models.TapdStoryLabel{})
	dataflowTester.Subtask(tasks.ExtractStoryMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&models.TapdStory{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_tapd_stories_for_due_date.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.SprintIssue{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.Subtask(tasks.ConvertStoryMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&ticket.Issue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issues_story_for_due_date.csv",
		IgnoreTypes: []interface{}{common.Model{}},
	})
}
