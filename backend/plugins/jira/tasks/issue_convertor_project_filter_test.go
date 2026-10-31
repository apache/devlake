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

package tasks

import (
	"sort"
	"testing"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/impls/dalgorm"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestConvertIssuesInputProjectFilter reproduces (and then guards against) the
// root cause of "DORA metrics show the whole shared board instead of just the
// project's slice".
//
// The domain board_issues.project_name stamp is written by the convertor. The
// convertor reads its input from _tool_jira_board_issues. BEFORE the fix, that
// input query filtered only by connection_id + board_id -- so when ProjectA and
// ProjectB share a board, ProjectA's convertor run read BOTH projects' tool
// rows and stamped them all "ProjectA", leaking ProjectB's issues into
// ProjectA (and vice versa). The project-level DORA filter downstream was then
// meaningless because every issue carried the wrong project_name.
//
// This test seeds a shared board (board 10) whose issues were collected per
// project into _tool_jira_board_issues (A-slice tagged ProjectA, B-slice tagged
// ProjectB, plus a legacy row with empty project_name), then runs the REAL
// convertor input query (convertIssuesInputClauses) for each project and
// asserts it returns only that project's slice (+ the legacy row). On the old
// unfiltered query this test fails (ProjectA would also see issue 1002).
func TestConvertIssuesInputProjectFilter(t *testing.T) {
	db := newJiraInMemoryDB(t)

	const connectionId = uint64(1)
	const sharedBoard = uint64(10)

	// tool issues (the board-level identity does not distinguish project)
	seedJiraIssue(t, db, connectionId, 1001)
	seedJiraIssue(t, db, connectionId, 1002)
	seedJiraIssue(t, db, connectionId, 1003)

	// per-project tool board-issue rows on the SAME board
	seedJiraBoardIssue(t, db, connectionId, sharedBoard, 1001, "ProjectA")
	seedJiraBoardIssue(t, db, connectionId, sharedBoard, 1002, "ProjectB")
	// 1003 is unstamped (empty) on a board that already HAS real per-project
	// stamps -> it is the ambiguous leftover pool and must NOT be attributed to
	// any project (attributing it was the cross-project leak).
	seedJiraBoardIssue(t, db, connectionId, sharedBoard, 1003, "")

	projectA := queryConvertIssueIds(t, db, connectionId, sharedBoard, "ProjectA")
	projectB := queryConvertIssueIds(t, db, connectionId, sharedBoard, "ProjectB")

	// ProjectA converts ONLY its own slice, never ProjectB's, never the pool.
	require.Equal(t, []uint64{1001}, projectA)
	require.NotContains(t, projectA, uint64(1002), "ProjectA convertor must not read ProjectB's issue on a shared board")
	require.NotContains(t, projectA, uint64(1003), "unstamped rows on a stamped board must be excluded")

	// ProjectB converts ONLY its own slice.
	require.Equal(t, []uint64{1002}, projectB)
	require.NotContains(t, projectB, uint64(1001))
	require.NotContains(t, projectB, uint64(1003))
}

// TestConvertIssuesInputExcludesNullPool mirrors the real staging data: ProjectA
// was collected (stamped) while an earlier unfiltered collection left tool rows
// with NULL project_name on the same board. ProjectA's convertor must read only
// its stamped slice; the NULL pool must be excluded (previously it leaked back
// in via the empty/NULL fallback).
func TestConvertIssuesInputExcludesNullPool(t *testing.T) {
	db := newJiraInMemoryDB(t)
	const connectionId = uint64(1)
	const sharedBoard = uint64(10)

	seedJiraIssue(t, db, connectionId, 1)
	seedJiraIssue(t, db, connectionId, 2)
	seedJiraIssue(t, db, connectionId, 3)
	seedJiraBoardIssue(t, db, connectionId, sharedBoard, 1, "ProjectA")
	// leftover unfiltered pool with NULL project_name
	seedJiraBoardIssueNull(t, db, connectionId, sharedBoard, 2)
	seedJiraBoardIssueNull(t, db, connectionId, sharedBoard, 3)

	ids := queryConvertIssueIds(t, db, connectionId, sharedBoard, "ProjectA")
	require.Equal(t, []uint64{1}, ids, "only ProjectA's stamped slice may convert; the NULL pool is excluded")
}

// TestConvertIssuesInputNoProject is the control: a non-shared board with no
// project context (empty project_name) still converts all of its issues, so the
// fix does not regress the ordinary single-project case.
func TestConvertIssuesInputNoProject(t *testing.T) {
	db := newJiraInMemoryDB(t)
	const connectionId = uint64(1)
	const board = uint64(20)

	seedJiraIssue(t, db, connectionId, 2001)
	seedJiraIssue(t, db, connectionId, 2002)
	seedJiraBoardIssue(t, db, connectionId, board, 2001, "")
	seedJiraBoardIssue(t, db, connectionId, board, 2002, "")

	ids := queryConvertIssueIds(t, db, connectionId, board, "")
	require.Equal(t, []uint64{2001, 2002}, ids)
}

// --- helpers ---------------------------------------------------------------

func newJiraInMemoryDB(t *testing.T) dal.Dal {
	gormDb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDb.AutoMigrate(&models.JiraIssue{}, &models.JiraBoardIssue{}))
	// start from a clean slate (shared in-memory cache can persist across tests)
	require.NoError(t, gormDb.Exec("DELETE FROM _tool_jira_issues").Error)
	require.NoError(t, gormDb.Exec("DELETE FROM _tool_jira_board_issues").Error)
	return dalgorm.NewDalgorm(gormDb)
}

func seedJiraIssue(t *testing.T, db dal.Dal, connectionId, issueId uint64) {
	require.NoError(t, db.Create(&models.JiraIssue{
		ConnectionId: connectionId,
		IssueId:      issueId,
	}))
}

func seedJiraBoardIssue(t *testing.T, db dal.Dal, connectionId, boardId, issueId uint64, projectName string) {
	require.NoError(t, db.Create(&models.JiraBoardIssue{
		ConnectionId: connectionId,
		BoardId:      boardId,
		IssueId:      issueId,
		ProjectName:  projectName,
	}))
}

func seedJiraBoardIssueNull(t *testing.T, db dal.Dal, connectionId, boardId, issueId uint64) {
	// leftover unfiltered pool: project_name is SQL NULL (distinct from empty).
	require.NoError(t, db.Exec(
		"INSERT INTO _tool_jira_board_issues (connection_id, board_id, issue_id, project_name) VALUES (?, ?, ?, NULL)",
		connectionId, boardId, issueId,
	))
}

func queryConvertIssueIds(t *testing.T, db dal.Dal, connectionId, boardId uint64, projectName string) []uint64 {
	cursor, err := db.Cursor(convertIssuesInputClauses(connectionId, boardId, projectName)...)
	require.NoError(t, err)
	defer cursor.Close()

	var ids []uint64
	for cursor.Next() {
		row := &models.JiraIssue{}
		require.NoError(t, db.Fetch(cursor, row))
		ids = append(ids, row.IssueId)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
