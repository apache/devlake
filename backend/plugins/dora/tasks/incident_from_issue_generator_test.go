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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestInScopeIncidentIssueProjectFilter is a fast, infrastructure-free check of
// the behavior behind the "DORA Details - Change Failure Rate" panel: when a
// per-project Extra JQL is set on a Jira board that is SHARED by several DevLake
// projects, each project's DORA metrics must count only that project's slice of
// issues, not the whole board.
//
// It runs the REAL production query (inScopeIncidentIssueClauses, used by
// ConvertIssuesToIncidents) against an in-memory SQLite database wrapped in
// DevLake's Dal, so there is no MySQL/Docker dependency and the test completes
// in milliseconds.
//
// Scenario - ProjectA and ProjectB both map to the same board (board1):
//
//	issue-A1  INCIDENT  bi.project_name=ProjectA
//	issue-A2  INCIDENT  bi.project_name=ProjectA
//	issue-B1  INCIDENT  bi.project_name=ProjectB
//	issue-N1  INCIDENT  bi.project_name=''        (legacy row, pre-feature)
//	issue-A3  BUG       bi.project_name=ProjectA  (not an INCIDENT)
//
// Change Failure Rate counts distinct INCIDENT issues in project scope:
//
//	ProjectA -> {A1, A2} + legacy N1
//	ProjectB -> {B1}     + legacy N1
//
// The decisive isolation guarantee is the cross-project EXCLUSION: ProjectA must
// never see issue-B1 and ProjectB must never see issue-A1/A2, even though both
// projects share board1. Before this feature both projects mapped to board1 and
// the query returned the entire board, so ProjectA would have wrongly counted
// issue-B1.
func TestInScopeIncidentIssueProjectFilter(t *testing.T) {
	db := newInMemoryDB(t)

	seedIssue(t, db, "issue-A1", "INCIDENT")
	seedIssue(t, db, "issue-A2", "INCIDENT")
	seedIssue(t, db, "issue-B1", "INCIDENT")
	seedIssue(t, db, "issue-N1", "INCIDENT")
	seedIssue(t, db, "issue-A3", "BUG")

	seedBoardIssue(t, db, "board1", "issue-A1", "ProjectA")
	seedBoardIssue(t, db, "board1", "issue-A2", "ProjectA")
	seedBoardIssue(t, db, "board1", "issue-B1", "ProjectB")
	seedBoardIssue(t, db, "board1", "issue-N1", "") // ambiguous pool row on a stamped board
	seedBoardIssue(t, db, "board1", "issue-A3", "ProjectA")

	// Both projects map to the SAME board - this is the shared-board setup.
	seedProjectMapping(t, db, "ProjectA", "boards", "board1")
	seedProjectMapping(t, db, "ProjectB", "boards", "board1")

	projectA := queryInScopeIncidentIssueIds(t, db, "ProjectA")
	projectB := queryInScopeIncidentIssueIds(t, db, "ProjectB")

	// board1 HAS real per-project stamps, so each project sees ONLY its own
	// stamped slice. The unstamped issue-N1 (the ambiguous pool) is excluded,
	// and ProjectB's issue never leaks into ProjectA.
	assert.Equal(t, []string{"issue-A1", "issue-A2"}, projectA)
	assert.NotContains(t, projectA, "issue-B1", "ProjectA must not see ProjectB's issue on the shared board")
	assert.NotContains(t, projectA, "issue-N1", "unstamped pool rows are excluded on a stamped board")
	assert.NotContains(t, projectA, "issue-A3", "non-INCIDENT issues never become incidents")

	// ProjectB: only its own stamped incident.
	assert.Equal(t, []string{"issue-B1"}, projectB)
	assert.NotContains(t, projectB, "issue-A1")
	assert.NotContains(t, projectB, "issue-A2")
	assert.NotContains(t, projectB, "issue-N1")

	// The two projects share a board yet get different Change-Failure-Rate
	// numerators - that is the whole point of per-project isolation.
	assert.NotEqual(t, projectA, projectB)
}

// TestInScopeIncidentIssueNoExtraJql is the control: when a board is NOT shared
// and no per-project slice is recorded (bi.project_name empty), the project
// still sees all of its board's incidents. This guarantees the feature does not
// regress the ordinary single-project case.
func TestInScopeIncidentIssueNoExtraJql(t *testing.T) {
	db := newInMemoryDB(t)

	seedIssue(t, db, "issue-1", "INCIDENT")
	seedIssue(t, db, "issue-2", "INCIDENT")
	seedBoardIssue(t, db, "boardX", "issue-1", "")
	seedBoardIssue(t, db, "boardX", "issue-2", "")
	seedProjectMapping(t, db, "SoloProject", "boards", "boardX")

	ids := queryInScopeIncidentIssueIds(t, db, "SoloProject")
	assert.Equal(t, []string{"issue-1", "issue-2"}, ids)
}

// --- helpers ---------------------------------------------------------------

func newInMemoryDB(t *testing.T) dal.Dal {
	gormDb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDb.AutoMigrate(&ticket.Issue{}, &ticket.BoardIssue{}, &crossdomain.ProjectMapping{}))
	return dalgorm.NewDalgorm(gormDb)
}

func seedIssue(t *testing.T, db dal.Dal, id, issueType string) {
	require.NoError(t, db.Create(&ticket.Issue{
		DomainEntity: domainlayer.DomainEntity{Id: id},
		Type:         issueType,
	}))
}

func seedBoardIssue(t *testing.T, db dal.Dal, boardId, issueId, projectName string) {
	require.NoError(t, db.Create(&ticket.BoardIssue{
		BoardId:     boardId,
		IssueId:     issueId,
		ProjectName: projectName,
	}))
}

func seedProjectMapping(t *testing.T, db dal.Dal, projectName, table, rowId string) {
	require.NoError(t, db.Create(&crossdomain.ProjectMapping{
		ProjectName: projectName,
		Table:       table,
		RowId:       rowId,
	}))
}

// queryInScopeIncidentIssueIds runs the real production selection used by
// ConvertIssuesToIncidents and returns the matched issue ids (sorted).
func queryInScopeIncidentIssueIds(t *testing.T, db dal.Dal, projectName string) []string {
	cursor, err := db.Cursor(inScopeIncidentIssueClauses(db, projectName)...)
	require.NoError(t, err)
	defer cursor.Close()

	var ids []string
	for cursor.Next() {
		row := &issueWithBoardId{}
		require.NoError(t, db.Fetch(cursor, row))
		ids = append(ids, row.Id)
	}
	sort.Strings(ids)
	return ids
}
