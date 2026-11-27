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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira/impl"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks"
	"github.com/stretchr/testify/assert"
)

// An epic of the board is collected by collectIssues and, while board issues refer to it, by collectEpics too.
// Both extractors run in full-sync mode here, so extractEpics replays every response of the epics raw table
// after extractIssues: it must not replace an issue with a response older than the one already extracted.
func TestEpicExtractorKeepsNewerIssue(t *testing.T) {
	var plugin impl.Jira
	dataflowTester := e2ehelper.NewDataFlowTester(t, "jira", plugin)
	const connectionId, boardId = 1, 8
	taskData := &tasks.JiraTaskData{
		Options: &tasks.JiraOptions{ConnectionId: connectionId, BoardId: boardId, ScopeConfig: &models.JiraScopeConfig{}},
	}
	const rawIssues, rawEpics = "_raw_jira_api_issues", "_raw_jira_api_epics"
	dataflowTester.FlushRawTable(rawIssues)
	dataflowTester.FlushRawTable(rawEpics)
	dataflowTester.FlushTabler(&models.JiraIssue{})
	dataflowTester.FlushTabler(&models.JiraBoardIssue{})
	dataflowTester.FlushTabler(&models.JiraSprintIssue{})
	dataflowTester.FlushTabler(&models.JiraIssueChangelogs{})
	dataflowTester.FlushTabler(&models.JiraIssueChangelogItems{})
	dataflowTester.FlushTabler(&models.JiraWorklog{})
	dataflowTester.FlushTabler(&models.JiraAccount{})
	dataflowTester.FlushTabler(&models.JiraIssueType{})
	dataflowTester.FlushTabler(&models.JiraIssueComment{})
	dataflowTester.FlushTabler(&models.JiraIssueLabel{})
	dataflowTester.FlushTabler(&models.JiraIssueField{})
	dataflowTester.FlushTabler(&models.JiraIssueRelationship{})

	day := func(n int) time.Time { return time.Date(2026, 1, n, 10, 0, 0, 0, time.UTC) }
	changelogId := 0
	// addResponse stores what Jira returned for the issue on the given day: its status and one changelog history
	addResponse := func(rawTable string, issueId uint64, updated time.Time, status string) {
		changelogId++
		response := fmt.Sprintf(`{
			"id": "%d", "key": "EPIC-%d", "self": "https://example.atlassian.net/rest/api/3/issue/%d",
			"fields": {
				"summary": "epic", "created": "2026-01-01T00:00:00.000+0000", "updated": "%s",
				"issuetype": {"id": "10000", "name": "Epic"}, "status": {"name": "%s"}
			},
			"changelog": {"total": 1, "histories": [{"id": "%d", "created": "%s"}]}
		}`, issueId, issueId, issueId, updated.Format("2006-01-02T15:04:05.000-0700"), status, changelogId, updated.Format("2006-01-02T15:04:05.000-0700"))
		assert.Nil(t, dataflowTester.Dal.Create(&api.RawData{
			Params:    fmt.Sprintf(`{"ConnectionId":%d,"BoardId":%d}`, connectionId, boardId),
			Data:      []byte(response),
			Input:     json.RawMessage(`{}`),
			CreatedAt: updated,
		}, dal.From(rawTable)))
	}
	// epic 1 belongs to the board; collectEpics stopped collecting it after day 2, collectIssues did not
	addResponse(rawEpics, 1, day(1), "To Do")
	addResponse(rawEpics, 1, day(2), "In Progress")
	addResponse(rawIssues, 1, day(3), "Done")
	// epic 2 was collected by collectEpics after its last collection by collectIssues
	addResponse(rawIssues, 2, day(1), "To Do")
	addResponse(rawEpics, 2, day(2), "In Progress")
	// epic 3 is outside the board, only collectEpics collects it
	addResponse(rawEpics, 3, day(1), "To Do")
	addResponse(rawEpics, 3, day(2), "Done")

	ctx := dataflowTester.SubtaskContext(taskData)
	assert.Nil(t, tasks.ExtractIssuesMeta.EntryPoint(ctx))
	assert.Nil(t, tasks.ExtractEpicsMeta.EntryPoint(ctx))

	var issues []models.JiraIssue
	assert.Nil(t, dataflowTester.Dal.All(&issues, dal.Where("connection_id = ?", connectionId), dal.Orderby("issue_id")))
	type issueState struct {
		issueId      uint64
		status       string
		updated      time.Time
		rawDataTable string
	}
	actual := make([]issueState, 0, len(issues))
	for _, issue := range issues {
		actual = append(actual, issueState{issue.IssueId, issue.StatusName, issue.Updated.UTC(), issue.RawDataTable})
	}
	assert.Equal(t, []issueState{
		{1, "Done", day(3), rawIssues},       // the older responses of the epics raw table are skipped
		{2, "In Progress", day(2), rawEpics}, // a newer response of the epics raw table still wins
		{3, "Done", day(2), rawEpics},        // the last response of the epics raw table wins
	}, actual)

	// the history carried only by the older responses is still extracted
	changelogs, err := dataflowTester.Dal.Count(dal.From(&models.JiraIssueChangelogs{}), dal.Where("connection_id = ? AND issue_id = ?", connectionId, 1))
	assert.Nil(t, err)
	assert.Equal(t, int64(3), changelogs)
}
