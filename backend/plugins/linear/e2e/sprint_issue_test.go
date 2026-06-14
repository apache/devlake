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

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/linear/impl"
	"github.com/apache/incubator-devlake/plugins/linear/models"
	"github.com/apache/incubator-devlake/plugins/linear/tasks"
)

// TestLinearSprintIssueStaleCycle guards against stale sprint_issues rows when
// an issue is moved out of a cycle. Sprint membership is derived from the
// issue's cycle_id; once that empties on re-collection, the issue must no
// longer appear in sprint_issues from a previous run.
func TestLinearSprintIssueStaleCycle(t *testing.T) {
	var linear impl.Linear
	dataflowTester := e2ehelper.NewDataFlowTester(t, "linear", linear)

	taskData := &tasks.LinearTaskData{
		Options: &tasks.LinearOptions{
			ConnectionId: 1,
			TeamId:       "team-1",
		},
	}

	// seed issues: issue-1 and issue-4 both belong to cycle-1
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_linear_issues.csv", "_raw_linear_issues")
	dataflowTester.FlushTabler(&models.LinearIssue{})
	dataflowTester.FlushTabler(&models.LinearIssueLabel{})
	dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)

	// first conversion: both issues land in the sprint
	dataflowTester.FlushTabler(&ticket.SprintIssue{})
	dataflowTester.Subtask(tasks.ConvertSprintIssuesMeta, taskData)

	// every issue is moved out of its cycle; on re-collection cycle_id empties.
	// The second conversion then produces zero sprint issues, which is the case
	// the batch divider's lazy delete fails to cover.
	if err := dataflowTester.Dal.Exec(
		"UPDATE _tool_linear_issues SET cycle_id = '' WHERE connection_id = ? AND team_id = ?", 1, "team-1",
	); err != nil {
		t.Fatal(err)
	}

	// second conversion (no flush) must drop ALL stale sprint_issues rows
	dataflowTester.Subtask(tasks.ConvertSprintIssuesMeta, taskData)
	dataflowTester.VerifyTableWithOptions(ticket.SprintIssue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/sprint_issues_after_leaving_cycle.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
