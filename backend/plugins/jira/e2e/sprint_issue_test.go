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
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/jira/impl"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks"
	"testing"
)

func TestSprintIssueDataFlow(t *testing.T) {
	var plugin impl.Jira
	dataflowTester := e2ehelper.NewDataFlowTester(t, "jira", plugin)

	taskData := &tasks.JiraTaskData{
		Options: &tasks.JiraOptions{
			ConnectionId: 2,
			BoardId:      8,
		},
	}
	// sprint_issues is built from the board's Sprint Reports. The fixture also
	// holds rows for another board on the same connection and for the same
	// board id on another connection; neither may be converted for board 8.
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_jira_sprint_reports_for_sprint_issues.csv", models.JiraSprintReport{})
	// Pre-existing links: one from board 9, which must survive a board 8 run,
	// and a stale one from board 8, which must be replaced.
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/sprint_issues_other_board.csv", ticket.SprintIssue{})
	dataflowTester.Subtask(tasks.ConvertSprintIssuesMeta, taskData)

	dataflowTester.VerifyTable(
		ticket.SprintIssue{},
		"./snapshot_tables/sprint_issues.csv",
		[]string{
			"sprint_id",
			"issue_id",
		},
	)
}
