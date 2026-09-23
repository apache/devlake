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
	"github.com/apache/devlake/core/utils"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/youtrack/impl"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/apache/devlake/plugins/youtrack/tasks"
	"github.com/stretchr/testify/require"
)

// The divider's lazy cleanup fires only when a converter emits an output row,
// so the domain tables below must be replaced explicitly: once every source
// row disappears, the scope's previous domain output must disappear too.

func countRows(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, model interface{}) int64 {
	var n int64
	require.NoError(t, dataflowTester.Db.Model(model).Count(&n).Error)
	return n
}

// TestYoutrackIssueLabelsAllRemovedReplacement: the project's last tag is
// removed — the extractor wipes the tool labels, the converter emits nothing.
func TestYoutrackIssueLabelsAllRemovedReplacement(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.FlushTabler(&models.YoutrackIssue{})
	dataflowTester.FlushTabler(&models.YoutrackIssueLabel{})
	dataflowTester.FlushTabler(&ticket.IssueLabel{})

	projectIds := []string{"0-1", "0-2"}
	for _, projectId := range projectIds {
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, newTaskData(projectId, zeroConfig()))
	}
	for _, projectId := range projectIds {
		dataflowTester.Subtask(tasks.ConvertIssueLabelsMeta, newTaskData(projectId, zeroConfig()))
	}
	require.Positive(t, countRows(t, dataflowTester, &ticket.IssueLabel{}), "populated scope converts labels into the domain layer")

	require.NoError(t, dataflowTester.Db.Where("connection_id = 1").Delete(&models.YoutrackIssueLabel{}).Error)

	for _, projectId := range projectIds {
		dataflowTester.Subtask(tasks.ConvertIssueLabelsMeta, newTaskData(projectId, zeroConfig()))
	}
	require.Zero(t, countRows(t, dataflowTester, &ticket.IssueLabel{}),
		"conversion emitting zero labels must still replace the scope's previous domain labels")
}

// TestYoutrackIssueAssigneesAllRemovedReplacement: every issue of the project
// becomes unassigned — issues still convert, but no assignee row is emitted.
func TestYoutrackIssueAssigneesAllRemovedReplacement(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_projects.csv", &models.YoutrackProject{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_issues_assignee_seed.csv", &models.YoutrackIssue{})
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	// the seed carries no raw origin; stamp the one the issue extractor would
	// have set, since the converter copies it onto every domain row
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).Where("connection_id = 1").Updates(map[string]interface{}{
		"_raw_data_table":  "_raw_youtrack_issues",
		"_raw_data_params": utils.ToJsonString(models.YoutrackApiParams{ConnectionId: 1, ProjectId: "0-1"}),
	}).Error)

	dataflowTester.Subtask(tasks.ConvertIssuesMeta, newTaskData("0-1", zeroConfig()))
	require.Positive(t, countRows(t, dataflowTester, &ticket.IssueAssignee{}), "an assigned issue converts into issue_assignees")

	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).
		Where("connection_id = 1").Updates(map[string]interface{}{"assignee_id": "", "assignee_name": ""}).Error)

	dataflowTester.Subtask(tasks.ConvertIssuesMeta, newTaskData("0-1", zeroConfig()))
	require.Positive(t, countRows(t, dataflowTester, &ticket.Issue{}), "the issue itself still converts")
	require.Zero(t, countRows(t, dataflowTester, &ticket.IssueAssignee{}),
		"conversion emitting zero assignees must still replace the scope's previous domain assignees")
}
