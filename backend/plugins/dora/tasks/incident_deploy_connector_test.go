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
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/impls/dalgorm"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestConnectIncidentInScopeProjectFilter reproduces the bug behind "the board
// still contains incidents that relate to Jira issues that shouldn't be there".
//
// ConnectIncidentToDeployment builds project_incident_deployment_relationships
// (the table the Change Failure Rate metric reads). It used to select incidents
// by project_mapping on scope_id (= board) ONLY, so on a board shared by several
// projects, every incident on the board was attached to every project -- the
// incident objects leaked across projects even though the per-issue slice was
// correct.
//
// Shared board (board1), mapped to both ProjectA and ProjectB. Incidents carry
// scope_id = board1 (that is all an incident knows). The per-project slice lives
// on board_issues.project_name:
//
//	incident/issue 1001  board_issues.project_name = ProjectA
//	incident/issue 1002  board_issues.project_name = ProjectB
//	incident/issue 1003  board_issues.project_name = ''        (legacy)
//
// Expected selection:
//	ProjectA -> {1001, 1003}  (never 1002)
//	ProjectB -> {1002, 1003}  (never 1001)
//
// On the OLD board-only query, ProjectA would also select 1002 -> the leak.
func TestConnectIncidentInScopeProjectFilter(t *testing.T) {
	db := newDoraIncidentDB(t)

	const board = "board1"
	seedIncident(t, db, "1001", board)
	seedIncident(t, db, "1002", board)
	seedIncident(t, db, "1003", board)

	seedDoraBoardIssue(t, db, board, "1001", "ProjectA")
	seedDoraBoardIssue(t, db, board, "1002", "ProjectB")
	// 1003 is an unstamped (empty) row on a board that DOES have real per-project
	// stamps. On such a board the empty/NULL rows are the ambiguous "unfiltered
	// pool" and must NOT be attributed to any project (that was the leak).
	seedDoraBoardIssue(t, db, board, "1003", "")

	seedDoraProjectMapping(t, db, "ProjectA", "boards", board)
	seedDoraProjectMapping(t, db, "ProjectB", "boards", board)

	projectA := queryConnectIncidentIds(t, db, "ProjectA")
	projectB := queryConnectIncidentIds(t, db, "ProjectB")

	require.Equal(t, []string{"1001"}, projectA)
	require.NotContains(t, projectA, "1002", "ProjectA must not attach ProjectB's incident on a shared board")
	require.NotContains(t, projectA, "1003", "unstamped rows on a stamped board must be excluded")

	require.Equal(t, []string{"1002"}, projectB)
	require.NotContains(t, projectB, "1001")
	require.NotContains(t, projectB, "1003")
}

// TestConnectIncidentInScopeExcludesNullPool reproduces the real staging state:
// ProjectA was collected (its ExtraJQL slice stamped with "ProjectA"), while the
// rest of the board's incidents are left over from an earlier unfiltered
// collection and carry a NULL project_name. ProjectA's Change Failure Rate must
// include ONLY its stamped slice -- the NULL "unfiltered pool" must be excluded,
// not re-admitted.
//
// This is the case the previous fix got wrong: a NULL/empty fallback re-added
// every NULL row to every project, so ProjectA showed the whole board.
func TestConnectIncidentInScopeExcludesNullPool(t *testing.T) {
	db := newDoraIncidentDB(t)

	const board = "board1"
	// ProjectA's real slice
	seedIncident(t, db, "A1", board)
	seedIncident(t, db, "A2", board)
	seedDoraBoardIssue(t, db, board, "A1", "ProjectA")
	seedDoraBoardIssue(t, db, board, "A2", "ProjectA")
	// leftover unfiltered pool from an earlier collection (NULL project_name)
	seedIncident(t, db, "N1", board)
	seedIncident(t, db, "N2", board)
	seedIncident(t, db, "N3", board)
	seedDoraBoardIssueNull(t, db, board, "N1")
	seedDoraBoardIssueNull(t, db, board, "N2")
	seedDoraBoardIssueNull(t, db, board, "N3")

	seedDoraProjectMapping(t, db, "ProjectA", "boards", board)

	ids := queryConnectIncidentIds(t, db, "ProjectA")
	require.Equal(t, []string{"A1", "A2"}, ids, "only ProjectA's stamped slice may count; the NULL pool must be excluded")
	require.NotContains(t, ids, "N1")
	require.NotContains(t, ids, "N2")
	require.NotContains(t, ids, "N3")
}

// TestConnectIncidentInScopeNoShare is the control: a non-shared board with no
// per-project slice still attaches all of its incidents (no regression).
func TestConnectIncidentInScopeNoShare(t *testing.T) {
	db := newDoraIncidentDB(t)

	const board = "boardSolo"
	seedIncident(t, db, "2001", board)
	seedIncident(t, db, "2002", board)
	seedDoraBoardIssue(t, db, board, "2001", "")
	seedDoraBoardIssue(t, db, board, "2002", "")
	seedDoraProjectMapping(t, db, "SoloProject", "boards", board)

	ids := queryConnectIncidentIds(t, db, "SoloProject")
	require.Equal(t, []string{"2001", "2002"}, ids)
}

// --- helpers ---------------------------------------------------------------

func newDoraIncidentDB(t *testing.T) dal.Dal {
	gormDb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDb.AutoMigrate(&ticket.Incident{}, &ticket.BoardIssue{}, &crossdomain.ProjectMapping{}))
	require.NoError(t, gormDb.Exec("DELETE FROM incidents").Error)
	require.NoError(t, gormDb.Exec("DELETE FROM board_issues").Error)
	require.NoError(t, gormDb.Exec("DELETE FROM project_mapping").Error)
	return dalgorm.NewDalgorm(gormDb)
}

func seedIncident(t *testing.T, db dal.Dal, id, boardId string) {
	require.NoError(t, db.Create(&ticket.Incident{
		DomainEntity: domainlayer.DomainEntity{Id: id},
		ScopeId:      boardId,
		Table:        "boards",
	}))
}

func seedDoraBoardIssue(t *testing.T, db dal.Dal, boardId, issueId, projectName string) {
	require.NoError(t, db.Create(&ticket.BoardIssue{
		BoardId:     boardId,
		IssueId:     issueId,
		ProjectName: projectName,
	}))
}

func seedDoraBoardIssueNull(t *testing.T, db dal.Dal, boardId, issueId string) {
	// Insert a row whose project_name is SQL NULL (the leftover unfiltered pool),
	// which is distinct from the empty-string case.
	require.NoError(t, db.Exec(
		"INSERT INTO board_issues (board_id, issue_id, project_name) VALUES (?, ?, NULL)",
		boardId, issueId,
	))
}

func seedDoraProjectMapping(t *testing.T, db dal.Dal, projectName, table, rowId string) {
	require.NoError(t, db.Create(&crossdomain.ProjectMapping{
		ProjectName: projectName,
		Table:       table,
		RowId:       rowId,
	}))
}

func queryConnectIncidentIds(t *testing.T, db dal.Dal, projectName string) []string {
	cursor, err := db.Cursor(connectIncidentInScopeClauses(db, projectName)...)
	require.NoError(t, err)
	defer cursor.Close()

	var ids []string
	for cursor.Next() {
		row := &ticket.Incident{}
		require.NoError(t, db.Fetch(cursor, row))
		ids = append(ids, row.Id)
	}
	sort.Strings(ids)
	return ids
}
