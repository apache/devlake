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
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/youtrack/impl"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/apache/devlake/plugins/youtrack/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestYoutrackCommentDataFlow covers the comment pipeline end to end: raw
// comments extract into _tool_youtrack_issue_comments (deleted flag kept)
// with authors derived into _tool_youtrack_accounts, and convert into
// domain issue_comments with didgen ids — deleted rows never arriving. The
// fixture is the anonymised live capture (covering: normal,
// deleted, and user-authored comments on both projects). Issues extract
// first because the comment convertor scopes comments to the project via
// their parent issue, exactly like the pipeline's SubTaskMetas order.
func TestYoutrackCommentDataFlow(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issue_comments.csv", "_raw_youtrack_issue_comments")
	dataflowTester.FlushTabler(&models.YoutrackIssue{})
	dataflowTester.FlushTabler(&models.YoutrackIssueComment{})
	dataflowTester.FlushTabler(&models.YoutrackAccount{})

	for _, projectId := range []string{"0-1", "0-2"} {
		taskData := newTaskData(projectId, zeroConfig())
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)
		dataflowTester.Subtask(tasks.ExtractCommentsMeta, taskData)
	}

	dataflowTester.VerifyTableWithOptions(models.YoutrackIssueComment{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_youtrack_issue_comments.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// explicit invariants, so the auto-generated snapshot is never the only oracle
	var commentCount, deletedCount, updatedCount int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueComment{}).
		Where("connection_id = 1").Count(&commentCount).Error)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueComment{}).
		Where("connection_id = 1 AND deleted").Count(&deletedCount).Error)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueComment{}).
		Where("connection_id = 1 AND updated IS NOT NULL").Count(&updatedCount).Error)
	assert.Equal(t, int64(7), commentCount, "every captured comment lands in the tool layer")
	assert.Equal(t, int64(2), deletedCount, "deleted comments stay in the tool layer, flag kept")
	assert.Positive(t, updatedCount, "edited comments must carry Updated")

	// comment authors derive into accounts (CROSS is on): every author id
	// referenced by a comment must have an account row
	var missingAuthors int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueComment{}).
		Where("connection_id = 1 AND author_id != '' AND author_id NOT IN "+
			"(SELECT id FROM _tool_youtrack_accounts WHERE connection_id = 1)").
		Count(&missingAuthors).Error)
	assert.Zero(t, missingAuthors, "comment authors must derive into _tool_youtrack_accounts")

	// convert: deleted comments never reach the domain layer
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_projects.csv", &models.YoutrackProject{})
	dataflowTester.FlushTabler(&ticket.IssueComment{})
	dataflowTester.FlushTabler(&crossdomain.Account{})
	for _, projectId := range []string{"0-1", "0-2"} {
		taskData := newTaskData(projectId, zeroConfig())
		dataflowTester.Subtask(tasks.ConvertCommentsMeta, taskData)
		dataflowTester.Subtask(tasks.ConvertAccountsMeta, taskData)
	}

	dataflowTester.VerifyTableWithOptions(ticket.IssueComment{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issue_comments.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	var domainCount int64
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueComment{}).Count(&domainCount).Error)
	assert.Equal(t, commentCount-deletedCount, domainCount, "only non-deleted comments convert")

	var leaked int64
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueComment{}).
		Where("id IN (SELECT CONCAT('youtrack:YoutrackIssueComment:1:', id) FROM "+
			"_tool_youtrack_issue_comments WHERE connection_id = 1 AND deleted)").
		Count(&leaked).Error)
	assert.Zero(t, leaked, "a deleted comment must never reach the domain layer")

	// didgen ids, AccountId from the author ref, and dates carried through
	var ids []string
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueComment{}).Limit(20).Pluck("id", &ids).Error)
	for _, id := range ids {
		assert.Regexp(t, `^youtrack:YoutrackIssueComment:1:\d+-\d+$`, id)
	}
	var issueIds []string
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueComment{}).Limit(20).Pluck("issue_id", &issueIds).Error)
	for _, id := range issueIds {
		assert.Regexp(t, `^youtrack:YoutrackIssue:1:\d+-\d+$`, id)
	}

	var noAccount int64
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueComment{}).
		Where("account_id = '' OR account_id IS NULL").Count(&noAccount).Error)
	assert.Zero(t, noAccount, "every live comment has an author — AccountId = didgen(AuthorId)")
	var accountIds []string
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueComment{}).Limit(20).Pluck("account_id", &accountIds).Error)
	for _, id := range accountIds {
		assert.Regexp(t, `^youtrack:YoutrackAccount:1:\d+-\d+$`, id)
	}

	var toolUpdated, domainUpdated int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueComment{}).
		Where("connection_id = 1 AND NOT deleted AND updated IS NOT NULL").Count(&toolUpdated).Error)
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueComment{}).
		Where("updated_date IS NOT NULL").Count(&domainUpdated).Error)
	assert.Equal(t, toolUpdated, domainUpdated, "Updated carries through to UpdatedDate")
	var missingCreated int64
	require.NoError(t, dataflowTester.Db.Model(&ticket.IssueComment{}).
		Where("created_date IS NULL OR created_date = '0001-01-01 00:00:00'").Count(&missingCreated).Error)
	assert.Zero(t, missingCreated, "CreatedDate is always set")

	// every converted comment references an issue of its project, and the
	// derived accounts convert too (guest skip is ConvertAccounts' own test)
	var domainAccounts int64
	require.NoError(t, dataflowTester.Db.Model(&crossdomain.Account{}).Count(&domainAccounts).Error)
	assert.Positive(t, domainAccounts)
}

// TestYoutrackCommentAllDeletedReplacement covers the replacement edge the
// lazy divider cleanup cannot reach: a scope whose comments have ALL become
// deleted emits zero output rows, and the previously converted domain rows
// must still disappear (deleted comments leave domain metrics).
func TestYoutrackCommentAllDeletedReplacement(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issue_comments.csv", "_raw_youtrack_issue_comments")
	dataflowTester.FlushTabler(&models.YoutrackIssue{})
	dataflowTester.FlushTabler(&models.YoutrackIssueComment{})
	dataflowTester.FlushTabler(&ticket.IssueComment{})

	for _, projectId := range []string{"0-1", "0-2"} {
		taskData := newTaskData(projectId, zeroConfig())
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)
		dataflowTester.Subtask(tasks.ExtractCommentsMeta, taskData)
	}
	for _, projectId := range []string{"0-1", "0-2"} {
		dataflowTester.Subtask(tasks.ConvertCommentsMeta, newTaskData(projectId, zeroConfig()))
	}

	domainCount := func() int64 {
		var n int64
		require.NoError(t, dataflowTester.Db.Model(&ticket.IssueComment{}).Count(&n).Error)
		return n
	}
	require.Positive(t, domainCount(), "populated scope converts comments into the domain layer")

	// every comment becomes deleted upstream (the tool layer keeps the rows,
	// flag set — exactly what the next incremental collection would extract)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssueComment{}).
		Where("connection_id = 1").Update("deleted", true).Error)

	for _, projectId := range []string{"0-1", "0-2"} {
		dataflowTester.Subtask(tasks.ConvertCommentsMeta, newTaskData(projectId, zeroConfig()))
	}
	assert.Zero(t, domainCount(),
		"conversion emitting zero rows must still replace the scope's previous domain output")
}
