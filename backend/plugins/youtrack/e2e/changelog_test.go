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
	"github.com/apache/devlake/plugins/youtrack/impl"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/apache/devlake/plugins/youtrack/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestYoutrackIssueChangelogDataFlow covers the changelog pipeline end to
// end: raw activity items extract into _tool_youtrack_issue_changelogs
// (multi-value payloads JSON-encoded, authors derived into accounts) and
// convert into domain issue_changelogs — state changes carrying std
// From/ToValue (mapping-aware, isResolved fallback via workflow_states),
// assignee changes carrying didgen account ids, everything else passing
// through. The fixture is the anonymised live capture (covering:
// a scalar State change, a multi-value Tags change, an assignee change, an
// IssueCreatedCategory item, multiple authors). Issues extract first
// because the changelog convertor scopes rows to the project via their
// parent issue, exactly like the pipeline's SubTaskMetas order.
func TestYoutrackIssueChangelogDataFlow(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issue_changelogs.csv", "_raw_youtrack_issue_changelogs")
	dataflowTester.FlushTabler(&models.YoutrackIssue{})
	dataflowTester.FlushTabler(&models.YoutrackIssueChangelog{})
	dataflowTester.FlushTabler(&models.YoutrackAccount{})

	for _, projectId := range []string{"0-1", "0-2"} {
		taskData := newTaskData(projectId, zeroConfig())
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)
		dataflowTester.Subtask(tasks.ExtractIssueChangelogsMeta, taskData)
	}

	dataflowTester.VerifyTableWithOptions(models.YoutrackIssueChangelog{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_youtrack_issue_changelogs.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// explicit invariants, so the auto-generated snapshot is never the only oracle
	countWhere := func(model interface{}, where string, args ...interface{}) int64 {
		var n int64
		require.NoError(t, dataflowTester.Db.Model(model).
			Where("connection_id = 1").Where(where, args...).Count(&n).Error)
		return n
	}
	toolCount := countWhere(&models.YoutrackIssueChangelog{}, "1 = 1")
	assert.Equal(t, int64(29), toolCount, "every captured activity item lands in the tool layer")

	// all five collected categories convert; the domain table has no
	// category column
	assert.Equal(t, int64(20), countWhere(&models.YoutrackIssueChangelog{}, "category = 'CustomFieldCategory'"))
	assert.Equal(t, int64(2), countWhere(&models.YoutrackIssueChangelog{}, "category = 'IssueCreatedCategory'"))
	assert.Equal(t, int64(2), countWhere(&models.YoutrackIssueChangelog{}, "category = 'IssueResolvedCategory'"))
	assert.Equal(t, int64(2), countWhere(&models.YoutrackIssueChangelog{}, "category = 'SummaryCategory'"))
	assert.Equal(t, int64(3), countWhere(&models.YoutrackIssueChangelog{}, "category = 'TagsCategory'"))

	// the captured shapes, at the table seam
	assert.Positive(t, countWhere(&models.YoutrackIssueChangelog{},
		"field_name = 'State' AND from_value != '' AND from_value NOT LIKE '[%'"),
		"a scalar State change: value ids scalar, display names alongside")
	assert.Positive(t, countWhere(&models.YoutrackIssueChangelog{},
		"field_name = 'State' AND original_from_value != '' AND original_from_value NOT LIKE '[%'"),
		"a scalar State change carries the original display names")
	assert.Equal(t, int64(4), countWhere(&models.YoutrackIssueChangelog{},
		`from_value LIKE '["%' OR to_value LIKE '["%' OR original_from_value LIKE '["%' OR original_to_value LIKE '["%'`),
		"multi-value changes round-trip as JSON in the tool layer")
	assert.Positive(t, countWhere(&models.YoutrackIssueChangelog{}, "field_name = 'Assignee' AND to_value != ''"),
		"an assignee change carries user value ids")
	assert.Equal(t, int64(2), countWhere(&models.YoutrackIssueChangelog{},
		"category = 'IssueCreatedCategory'"), "IssueCreatedCategory items land")

	// multiple authors; activity authors (and the users inside assignee
	// changes) derive into accounts — every referenced author id has an
	// account row
	var authorCount int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueChangelog{}).
		Where("connection_id = 1 AND author_id != ''").Distinct("author_id").Count(&authorCount).Error)
	assert.Greater(t, authorCount, int64(1), "items are authored by different users")
	var missingAuthors int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueChangelog{}).
		Where("connection_id = 1 AND author_id != '' AND author_id NOT IN "+
			"(SELECT id FROM _tool_youtrack_accounts WHERE connection_id = 1)").
		Count(&missingAuthors).Error)
	assert.Zero(t, missingAuthors, "activity authors must derive into _tool_youtrack_accounts")

	// convert: workflow_states seeds the isResolved lookup, exactly like the
	// pipeline, where the workflow-state subtasks run first
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_projects.csv", &models.YoutrackProject{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_workflow_states.csv", &models.YoutrackWorkflowState{})
	dataflowTester.FlushTabler(&ticket.IssueChangelogs{})
	for _, projectId := range []string{"0-1", "0-2"} {
		dataflowTester.Subtask(tasks.ConvertIssueChangelogsMeta, newTaskData(projectId, zeroConfig()))
	}

	dataflowTester.VerifyTableWithOptions(ticket.IssueChangelogs{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issue_changelogs.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	var domainCount int64
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueChangelogs{}).Count(&domainCount).Error)
	assert.Equal(t, toolCount, domainCount, "every collected activity item converts")

	domainCountWhere := func(where string, args ...interface{}) int64 {
		var n int64
		require.NoError(t, dataflowTester.Db.Model(&ticket.IssueChangelogs{}).Where(where, args...).Count(&n).Error)
		return n
	}

	// state changes carry std From/ToValue (zero-config: isResolved -> DONE,
	// else TODO) plus the original display names
	assert.Equal(t, countWhere(&models.YoutrackIssueChangelog{}, "field_name = 'State'"),
		domainCountWhere("field_name = 'State'"), "every state change converts")
	assert.Zero(t, domainCountWhere(
		"field_name = 'State' AND from_value NOT IN ('', 'TODO', 'DONE')"),
		"zero-config state FromValue is std (isResolved fallback via workflow_states)")
	assert.Zero(t, domainCountWhere(
		"field_name = 'State' AND to_value NOT IN ('', 'TODO', 'DONE')"),
		"zero-config state ToValue is std (isResolved fallback via workflow_states)")
	assert.Positive(t, domainCountWhere("field_name = 'State' AND to_value = 'DONE'"),
		"a transition into a resolved state reads DONE")
	assert.Positive(t, domainCountWhere("field_name = 'State' AND to_value = 'TODO'"),
		"a transition into an unresolved state reads TODO")
	assert.Positive(t, domainCountWhere("field_name = 'State' AND original_to_value NOT IN ('TODO', 'DONE')"),
		"Original* keep the stored display names")

	// assignee changes carry didgen account ids in From/ToValue and full
	// names in Original* — scalar and multi-value JSON shapes alike
	assert.Zero(t, domainCountWhere(
		`field_name = 'Assignee' AND to_value != '' AND to_value NOT LIKE '["%' AND to_value NOT LIKE 'youtrack:YoutrackAccount:1:%'`),
		"assignee ToValue is a didgen account id")
	assert.Zero(t, domainCountWhere(
		`field_name = 'Assignee' AND from_value != '' AND from_value NOT LIKE '["%' AND from_value NOT LIKE 'youtrack:YoutrackAccount:1:%'`),
		"assignee FromValue is a didgen account id")
	assert.Zero(t, domainCountWhere(
		`field_name = 'Assignee' AND to_value LIKE '["%' AND to_value NOT LIKE '[%"youtrack:YoutrackAccount:1:%'`),
		"multi-assignee ToValue is a JSON array of didgen account ids")
	assert.Zero(t, domainCountWhere(
		`field_name = 'Assignee' AND from_value LIKE '["%' AND from_value NOT LIKE '[%"youtrack:YoutrackAccount:1:%'`),
		"multi-assignee FromValue is a JSON array of didgen account ids")
	assert.Zero(t, domainCountWhere(
		"field_name = 'Assignee' AND (original_to_value LIKE 'youtrack:%' OR original_from_value LIKE 'youtrack:%')"),
		"assignee Original* keep the full names, never didgen ids")

	// pass-through categories convert without corruption, multi-value JSON
	// included
	assert.Equal(t, int64(4), domainCountWhere(
		`from_value LIKE '["%' OR to_value LIKE '["%' OR original_from_value LIKE '["%' OR original_to_value LIKE '["%'`),
		"multi-value changes convert without corruption")
	assert.Positive(t, domainCountWhere("field_name = 'tags'"), "tags changes convert")

	// ids, authorship and dates at the domain seam
	var ids []string
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueChangelogs{}).Limit(20).Pluck("id", &ids).Error)
	for _, id := range ids {
		assert.Regexp(t, `^youtrack:YoutrackIssueChangelog:1:.+$`, id)
	}
	var issueIds []string
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueChangelogs{}).Limit(20).Pluck("issue_id", &issueIds).Error)
	for _, id := range issueIds {
		assert.Regexp(t, `^youtrack:YoutrackIssue:1:\d+-\d+$`, id)
	}
	assert.Zero(t, domainCountWhere("field_id != ''"), "FieldId stays empty")
	assert.Zero(t, domainCountWhere("created_date IS NULL OR created_date = '0001-01-01 00:00:00'"),
		"CreatedDate is always set")
	assert.Zero(t, domainCountWhere("author_id LIKE '%-%' AND author_id NOT LIKE 'youtrack:YoutrackAccount:1:%'"),
		"AuthorId is a didgen account id when set")
	assert.Positive(t, domainCountWhere("author_name != ''"), "AuthorName carries the display name")

	// re-walks are idempotent via upsert by activity id: the
	// config change flips the extractor to full sync, so every
	// raw row is reprocessed — the counts must not move
	reextract := zeroConfig()
	reextract.StoryPointField = "Days in stage"
	for _, projectId := range []string{"0-1", "0-2"} {
		taskData := newTaskData(projectId, reextract)
		dataflowTester.Subtask(tasks.ExtractIssueChangelogsMeta, taskData)
		dataflowTester.Subtask(tasks.ConvertIssueChangelogsMeta, taskData)
	}
	assert.Equal(t, toolCount, countWhere(&models.YoutrackIssueChangelog{}, "1 = 1"),
		"a full re-extraction upserts by activity id — no duplicate tool rows")
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueChangelogs{}).Count(&domainCount).Error)
	assert.Equal(t, toolCount, domainCount, "re-conversion upserts by id — no duplicate domain rows")
}

// TestYoutrackIssueChangelogMappedStatuses covers the mapping-aware state
// conversion: a configured status mapping beats the isResolved fallback,
// from the same raw capture.
func TestYoutrackIssueChangelogMappedStatuses(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issue_changelogs.csv", "_raw_youtrack_issue_changelogs")
	dataflowTester.FlushTabler(&models.YoutrackIssue{})
	dataflowTester.FlushTabler(&models.YoutrackIssueLabel{})
	dataflowTester.FlushTabler(&models.YoutrackIssueChangelog{})

	for _, projectId := range []string{"0-1", "0-2"} {
		taskData := newTaskData(projectId, mappedConfig())
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)
		dataflowTester.Subtask(tasks.ExtractIssueChangelogsMeta, taskData)
	}

	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_projects.csv", &models.YoutrackProject{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_workflow_states.csv", &models.YoutrackWorkflowState{})
	dataflowTester.FlushTabler(&ticket.IssueChangelogs{})
	for _, projectId := range []string{"0-1", "0-2"} {
		dataflowTester.Subtask(tasks.ConvertIssueChangelogsMeta, newTaskData(projectId, mappedConfig()))
	}

	// the fixture's Backlog transitions: the configured OTHER mapping must
	// beat the isResolved fallback (TODO) — from and to both
	var backlogToOther int64
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueChangelogs{}).
		Where("field_name = 'State' AND original_to_value = 'Backlog' AND to_value = ?", ticket.OTHER).
		Count(&backlogToOther).Error)
	assert.Positive(t, backlogToOther, "a configured status mapping applies to state changelogs")
	var backlogToTodo int64
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueChangelogs{}).
		Where("field_name = 'State' AND original_to_value = 'Backlog' AND to_value = ?", ticket.TODO).
		Count(&backlogToTodo).Error)
	assert.Zero(t, backlogToTodo, "the isResolved fallback must not override the mapping")
}
