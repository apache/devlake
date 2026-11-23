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
	"sort"
	"testing"
	"time"

	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/jira/impl"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks"
	"github.com/stretchr/testify/assert"
)

func TestChangelogCollectorClauses(t *testing.T) {
	var plugin impl.Jira
	dataflowTester := e2ehelper.NewDataFlowTester(t, "jira", plugin)
	dataflowTester.FlushTabler(&models.JiraBoardIssue{})
	dataflowTester.FlushTabler(&models.JiraIssue{})
	dataflowTester.FlushTabler(&models.JiraIssueChangelogs{})

	const connectionId, boardId, otherBoardId = 1, 10, 20
	since := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	before := since.AddDate(0, 0, -9)
	after := since.AddDate(0, 0, 10)

	issues := []struct {
		id      uint64
		board   uint64
		stdType string
		updated time.Time
		total   int
		stored  int
	}{
		{1, boardId, "REQUIREMENT", after, 50, 50},       // updated since the last sync
		{2, boardId, "REQUIREMENT", before, 50, 40},      // not updated, changelog truncated: must be repaired
		{3, boardId, "REQUIREMENT", before, 50, 50},      // not updated, changelog complete
		{4, boardId, "REQUIREMENT", after, 30, 30},       // fully embedded in the search API response
		{5, boardId, "Epic", after, 50, 40},              // epics are skipped
		{6, otherBoardId, "REQUIREMENT", before, 50, 40}, // another board
	}
	changelogId := uint64(0)
	for _, issue := range issues {
		assert.Nil(t, dataflowTester.Dal.Create(&models.JiraBoardIssue{ConnectionId: connectionId, BoardId: issue.board, IssueId: issue.id}))
		assert.Nil(t, dataflowTester.Dal.Create(&models.JiraIssue{
			ConnectionId: connectionId, IssueId: issue.id, StdType: issue.stdType, Created: before, Updated: issue.updated,
			ChangelogTotal: issue.total,
		}))
		for i := 0; i < issue.stored; i++ {
			changelogId++
			assert.Nil(t, dataflowTester.Dal.Create(&models.JiraIssueChangelogs{
				ConnectionId: connectionId, ChangelogId: changelogId, IssueId: issue.id, Created: before,
			}))
		}
	}

	selected := func(since *time.Time) []uint64 {
		var rows []struct {
			IssueId    uint64
			UpdateTime time.Time
		}
		assert.Nil(t, dataflowTester.Dal.All(&rows, tasks.BuildChangelogCollectorClauses(connectionId, boardId, since)...))
		ids := make([]uint64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.IssueId)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return ids
	}

	// incremental sync: issues updated since the last sync, plus issues whose stored changelog is incomplete
	assert.Equal(t, []uint64{1, 2}, selected(&since))
	// full sync: every non-epic issue of the board with more histories than the search API embeds
	assert.Equal(t, []uint64{1, 2, 3}, selected(nil))
}
