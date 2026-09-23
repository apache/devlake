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
	"testing"
	"time"

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/youtrack/impl"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/apache/devlake/plugins/youtrack/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mappedConfig() *models.YoutrackScopeConfig {
	return &models.YoutrackScopeConfig{
		ScopeConfig: common.ScopeConfig{
			Entities: []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
		},
		TypeField:       "Client type",
		StoryPointField: "Days in stage",
		DueDateField:    "Deadline",
		TypeMappings: map[string]string{
			"Bug":   ticket.BUG,
			"Story": ticket.REQUIREMENT,
		},
		StatusMappings: map[string]string{
			// the fixture's only unresolved state; mapped to OTHER so the
			// assertion can't be satisfied by the isResolved default (TODO)
			"Backlog": ticket.OTHER,
		},
	}
}

// TestYoutrackIssueDataFlowWithMappings covers the configured run: the
// renamed type field resolves by configured name, mappings apply at
// extraction into the Std* columns, and the optional SP/DD slots populate
// when named. It then changes a mapping and re-extracts from the raw layer —
// no new YouTrack API calls (the raw import is untouched).
func TestYoutrackIssueDataFlowWithMappings(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.FlushTabler(&models.YoutrackIssue{})
	dataflowTester.FlushTabler(&models.YoutrackIssueLabel{})
	dataflowTester.FlushTabler(&models.YoutrackAccount{})

	for _, projectId := range []string{"0-1", "0-2"} {
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, newTaskData(projectId, mappedConfig()))
	}

	dataflowTester.VerifyTableWithOptions(models.YoutrackIssue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_youtrack_issues_mapped.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	count := func(where string, args ...interface{}) int64 {
		var n int64
		require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).
			Where("connection_id = 1").Where(where, args...).Count(&n).Error)
		return n
	}
	typedCount := count("type_name != ''")
	bugCount := count("std_type = ?", ticket.BUG)
	requirementCount := count("std_type = ?", ticket.REQUIREMENT)
	otherCount := count("std_status = ?", ticket.OTHER)
	spCount := count("story_point IS NOT NULL")
	ddCount := count("due_date IS NOT NULL")
	assert.Positive(t, typedCount, "the renamed `Client type` resolves by configured name")
	assert.Positive(t, bugCount, "configured type mapping applies at extraction")
	assert.Positive(t, requirementCount)
	assert.Positive(t, otherCount, "configured status mapping applies at extraction")
	assert.Positive(t, spCount, "named story-point slot populates")
	assert.Positive(t, ddCount, "named due-date slot populates")

	// convert with the mapped tool layer: the domain rows carry the mapped values
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_projects.csv", &models.YoutrackProject{})
	dataflowTester.FlushTabler(&ticket.Board{})
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	for _, projectId := range []string{"0-1", "0-2"} {
		taskData := newTaskData(projectId, mappedConfig())
		dataflowTester.Subtask(tasks.ConvertProjectsMeta, taskData)
		dataflowTester.Subtask(tasks.ConvertIssuesMeta, taskData)
	}
	dataflowTester.VerifyTableWithOptions(ticket.Issue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issues_mapped.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	var domainBug, domainOther int64
	require.NoError(t, dataflowTester.Db.Model(&ticket.Issue{}).Where("type = ?", ticket.BUG).Count(&domainBug).Error)
	require.NoError(t, dataflowTester.Db.Model(&ticket.Issue{}).Where("status = ?", ticket.OTHER).Count(&domainOther).Error)
	assert.Equal(t, bugCount, domainBug)
	assert.Equal(t, otherCount, domainOther)

	// change a mapping and re-extract from the raw layer — the collector is
	// not involved (the raw table is untouched), so no new YouTrack API calls
	remapped := mappedConfig()
	remapped.TypeMappings = map[string]string{"Story": ticket.TASK, "Bug": ticket.BUG}
	for _, projectId := range []string{"0-1", "0-2"} {
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, newTaskData(projectId, remapped))
	}
	var storyAsTask, storyAsRequirement int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).
		Where("connection_id = 1 AND type_name = 'Story' AND std_type = ?", ticket.TASK).Count(&storyAsTask).Error)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).
		Where("connection_id = 1 AND type_name = 'Story' AND std_type = ?", ticket.REQUIREMENT).Count(&storyAsRequirement).Error)
	assert.Positive(t, storyAsTask, "changed mapping re-applies from the raw layer")
	assert.Zero(t, storyAsRequirement, "no row keeps the stale mapping")
}

// TestYoutrackMappingReplayPreservesState covers the transition the
// DataFlowTester.Subtask reruns above cannot: Subtask deletes subtask state
// and forces FullSync, so it can never verify that a config change
// invalidates PERSISTED incremental state. This scenario keeps the state:
//
//	run 1  initial extraction (full sync: no prior state)
//	run 2  incremental update — one issue re-collected without its tag
//	run 3  mapping change — full re-extraction replayed from the raw layer
//
// asserting (a) the incremental run processes only the new raw version,
// (b) the replay never re-hits the API and never resurrects the removed
// tag from the retained old version, and (c) the new mapping applies.
func TestYoutrackMappingReplayPreservesState(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)
	beginStatefulScenario(t, dataflowTester)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.FlushTabler(&models.YoutrackIssue{})
	dataflowTester.FlushTabler(&models.YoutrackIssueLabel{})
	dataflowTester.FlushTabler(&models.YoutrackAccount{})

	// run 1: initial full extraction
	for _, projectId := range []string{"0-1", "0-2"} {
		runSubtaskPreservingState(t, dataflowTester, tasks.ExtractIssuesMeta, newTaskData(projectId, mappedConfig()))
	}

	// the fixture's tagged issue in project 0-1 (PROJ1-1, tag "Долго в работе")
	const taggedIssue = "2-1"
	countLabels := func(issueId string) int64 {
		var n int64
		require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueLabel{}).
			Where("connection_id = 1 AND issue_id = ?", issueId).Count(&n).Error)
		return n
	}
	var totalLabels int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueLabel{}).
		Where("connection_id = 1").Count(&totalLabels).Error)
	require.Equal(t, int64(1), countLabels(taggedIssue), "run 1 extracts the fixture's tag")
	require.Positive(t, totalLabels)

	// the incremental update: the same issue re-collected tagless, updated later
	var rawCountBefore int64
	require.NoError(t, dataflowTester.Db.Table("_raw_youtrack_issues").Count(&rawCountBefore).Error)
	type rawIssueRow struct {
		Params string
		Data   []byte
		Url    string
	}
	var rows []rawIssueRow
	require.NoError(t, dataflowTester.Db.Table("_raw_youtrack_issues").
		Where("params LIKE ?", `%0-1%`).Find(&rows).Error)
	var src rawIssueRow
	var v2data []byte
	for _, row := range rows {
		var issue map[string]interface{}
		require.NoError(t, json.Unmarshal(row.Data, &issue))
		if issue["id"] != taggedIssue {
			continue
		}
		issue["tags"] = []interface{}{}
		issue["updated"] = issue["updated"].(float64) + 60000
		v2, err := json.Marshal(issue)
		require.NoError(t, err)
		v2data = v2
		src = row
		break
	}
	require.NotNil(t, v2data, "the fixture must contain issue %s in project 0-1", taggedIssue)
	require.NoError(t, dataflowTester.Db.Table("_raw_youtrack_issues").Create(map[string]interface{}{
		"params":     src.Params,
		"data":       v2data,
		"url":        src.Url,
		"created_at": time.Now(), // inside run 2's incremental window
	}).Error)

	// run 2: same config, state preserved -> incremental — only the new
	// version is processed, the tag disappears
	for _, projectId := range []string{"0-1", "0-2"} {
		runSubtaskPreservingState(t, dataflowTester, tasks.ExtractIssuesMeta, newTaskData(projectId, mappedConfig()))
	}
	assert.Zero(t, countLabels(taggedIssue), "incremental re-extraction replaces the issue's labels")
	assert.Equal(t, totalLabels-1, func() int64 {
		var n int64
		require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueLabel{}).
			Where("connection_id = 1").Count(&n).Error)
		return n
	}(), "every other issue's labels are untouched")

	// run 3: a mapping change must invalidate the persisted incremental
	// state -> full re-extraction replayed from the raw layer. The retained
	// v1 still carries the tag; only latest-version expansion keeps it dead.
	remapped := mappedConfig()
	remapped.TypeMappings = map[string]string{"Story": ticket.TASK, "Bug": ticket.BUG}
	for _, projectId := range []string{"0-1", "0-2"} {
		runSubtaskPreservingState(t, dataflowTester, tasks.ExtractIssuesMeta, newTaskData(projectId, remapped))
	}
	assert.Zero(t, countLabels(taggedIssue),
		"mapping replay must not resurrect the tag from the retained old raw version")

	var storyAsTask, storyAsRequirement int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).
		Where("connection_id = 1 AND type_name = 'Story' AND std_type = ?", ticket.TASK).Count(&storyAsTask).Error)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).
		Where("connection_id = 1 AND type_name = 'Story' AND std_type = ?", ticket.REQUIREMENT).Count(&storyAsRequirement).Error)
	assert.Positive(t, storyAsTask, "the changed mapping re-applies from the raw layer")
	assert.Zero(t, storyAsRequirement, "no row keeps the stale mapping")

	var rawCountAfter int64
	require.NoError(t, dataflowTester.Db.Table("_raw_youtrack_issues").Count(&rawCountAfter).Error)
	assert.Equal(t, rawCountBefore+1, rawCountAfter,
		"the replay read the raw layer only — no collection call (the task data carries no API client)")
}

// TestYoutrackIssueDataFlowMissingField covers the renamed-field trap: a
// configured-but-absent field warns per scope and leaves the dedicated
// columns empty; the run completes (warn, never fail).
func TestYoutrackIssueDataFlowMissingField(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.FlushTabler(&models.YoutrackIssue{})
	dataflowTester.FlushTabler(&models.YoutrackIssueLabel{})
	dataflowTester.FlushTabler(&models.YoutrackAccount{})

	missing := zeroConfig()
	missing.TypeField = "No Such Field"
	missing.StateField = "Also Not There"
	for _, projectId := range []string{"0-1", "0-2"} {
		// panics on error: the run completing at all is half the assertion
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, newTaskData(projectId, missing))
	}

	var issueCount, typedCount, statedCount int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).Where("connection_id = 1").Count(&issueCount).Error)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).Where("connection_id = 1 AND type_name != ''").Count(&typedCount).Error)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).Where("connection_id = 1 AND state_name != ''").Count(&statedCount).Error)
	assert.Equal(t, int64(15), issueCount, "a missing field never kills extraction")
	assert.Zero(t, typedCount, "configured-but-absent type field leaves the column empty")
	assert.Zero(t, statedCount, "configured-but-absent state field leaves the column empty")
}

// TestYoutrackAccountsOnlyWithCross covers the entity gating: accounts
// derive inline only when CROSS is among the scope config's entities.
func TestYoutrackAccountsOnlyWithCross(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.FlushTabler(&models.YoutrackAccount{})

	ticketOnly := zeroConfig()
	ticketOnly.Entities = []string{plugin.DOMAIN_TYPE_TICKET}
	for _, projectId := range []string{"0-1", "0-2"} {
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, newTaskData(projectId, ticketOnly))
	}
	var accountCount int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackAccount{}).Where("connection_id = 1").Count(&accountCount).Error)
	assert.Zero(t, accountCount, "no accounts without CROSS in entities")

	for _, projectId := range []string{"0-1", "0-2"} {
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, newTaskData(projectId, zeroConfig()))
	}
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackAccount{}).Where("connection_id = 1").Count(&accountCount).Error)
	assert.Positive(t, accountCount, "accounts derive inline when CROSS is collected")
}
