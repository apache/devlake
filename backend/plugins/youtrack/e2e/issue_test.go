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
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/youtrack/impl"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/apache/devlake/plugins/youtrack/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTaskData builds the per-project task data for an e2e run: the REST
// client stays nil (raw CSVs are imported instead of collecting), and the
// connection carries only the endpoint the convertors compute URLs from.
// Project mirrors what PrepareTaskData would have loaded from the scope
// table (see snapshot_tables/_tool_youtrack_projects.csv).
func newTaskData(projectId string, scopeConfig *models.YoutrackScopeConfig) *tasks.YoutrackTaskData {
	connection := &models.YoutrackConnection{}
	connection.Endpoint = "https://youtrack.example.com/youtrack/api"
	shortNames := map[string]string{"0-1": "PROJ1", "0-2": "PROJ2"}
	return &tasks.YoutrackTaskData{
		Options:    &tasks.YoutrackOptions{ConnectionId: 1, ProjectId: projectId},
		Connection: connection,
		Project: &models.YoutrackProject{
			Id:        projectId,
			ShortName: shortNames[projectId],
		},
		ScopeConfig: scopeConfig,
	}
}

func zeroConfig() *models.YoutrackScopeConfig {
	return &models.YoutrackScopeConfig{
		ScopeConfig: common.ScopeConfig{
			Entities: []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
		},
	}
}

// TestYoutrackIssueDataFlowZeroConfig covers the zero-config run: no field
// slots configured (defaults apply), no type/status mappings. The fixture
// encodes the live instance's traps: the type field is renamed (`Client
// type`, invisible to the default `Type`), the assignee field is multi-value
// (ineligible), and states are resolved via isResolved alone.
func TestYoutrackIssueDataFlowZeroConfig(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_issues.csv", "_raw_youtrack_issues")
	dataflowTester.FlushTabler(&models.YoutrackIssue{})
	dataflowTester.FlushTabler(&models.YoutrackIssueLabel{})
	dataflowTester.FlushTabler(&models.YoutrackAccount{})

	for _, projectId := range []string{"0-1", "0-2"} {
		dataflowTester.Subtask(tasks.ExtractIssuesMeta, newTaskData(projectId, zeroConfig()))
	}

	dataflowTester.VerifyTableWithOptions(models.YoutrackIssue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_youtrack_issues_zero_config.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTableWithOptions(models.YoutrackIssueLabel{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_youtrack_issue_labels.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTableWithOptions(models.YoutrackAccount{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_youtrack_accounts.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// explicit invariants, so the auto-generated snapshot is never the only oracle
	var doneCount, todoCount int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).
		Where("connection_id = 1 AND std_status = ?", ticket.DONE).Count(&doneCount).Error)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackIssue{}).
		Where("connection_id = 1 AND std_status = ?", ticket.TODO).Count(&todoCount).Error)
	assert.Positive(t, doneCount, "zero-config: resolved states must land as DONE")
	assert.Positive(t, todoCount, "zero-config: unresolved states must land as TODO")

	var typedCount, assigneeCount, leadTimeCount int64
	dataflowTester.Db.Model(&models.YoutrackIssue{}).Where("connection_id = 1 AND type_name != ''").Count(&typedCount)
	dataflowTester.Db.Model(&models.YoutrackIssue{}).Where("connection_id = 1 AND assignee_id != ''").Count(&assigneeCount)
	dataflowTester.Db.Model(&models.YoutrackIssue{}).Where("connection_id = 1 AND lead_time_minutes IS NOT NULL").Count(&leadTimeCount)
	assert.Zero(t, typedCount, "zero-config: the instance renamed Type to `Client type`, so the default slot finds nothing")
	assert.Zero(t, assigneeCount, "zero-config: the instance's Assignee is user[*] — multi-value fields are ineligible")
	assert.Positive(t, leadTimeCount, "resolved issues must carry lead time computed at extraction")

	// accounts derived inline from reporter/updater refs (CROSS is on)
	var accountCount int64
	dataflowTester.Db.Model(&models.YoutrackAccount{}).Where("connection_id = 1").Count(&accountCount)
	assert.Positive(t, accountCount)

	// multi-tag issues produce one label row per tag
	var labelCount, multiTagIssues int64
	dataflowTester.Db.Model(&models.YoutrackIssueLabel{}).Where("connection_id = 1").Count(&labelCount)
	dataflowTester.Db.Model(&models.YoutrackIssueLabel{}).Where("connection_id = 1").
		Select("issue_id").Group("issue_id").Having("COUNT(*) > 1").Count(&multiTagIssues)
	assert.Positive(t, labelCount)
	assert.Positive(t, multiTagIssues, "the fixture's multi-tag issues must yield multiple label rows")

	// seed the scope rows (created by attaching scopes in production), then convert
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_projects.csv", &models.YoutrackProject{})
	dataflowTester.FlushTabler(&ticket.Board{})
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.FlushTabler(&ticket.IssueLabel{})
	dataflowTester.FlushTabler(&crossdomain.Account{})
	for _, projectId := range []string{"0-1", "0-2"} {
		taskData := newTaskData(projectId, zeroConfig())
		dataflowTester.Subtask(tasks.ConvertProjectsMeta, taskData)
		dataflowTester.Subtask(tasks.ConvertIssuesMeta, taskData)
		dataflowTester.Subtask(tasks.ConvertIssueLabelsMeta, taskData)
		dataflowTester.Subtask(tasks.ConvertAccountsMeta, taskData)
	}

	dataflowTester.VerifyTableWithOptions(ticket.Board{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/boards.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTableWithOptions(ticket.Issue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issues_zero_config.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTableWithOptions(ticket.BoardIssue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/board_issues.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTableWithOptions(ticket.IssueLabel{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issue_labels.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTableWithOptions(ticket.IssueAssignee{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issue_assignees.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTableWithOptions(crossdomain.Account{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/accounts.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	var issueCount, boardIssueCount, domainAccountCount int64
	dataflowTester.Db.Model(&ticket.Issue{}).Count(&issueCount)
	dataflowTester.Db.Model(&ticket.BoardIssue{}).Count(&boardIssueCount)
	dataflowTester.Db.Model(&crossdomain.Account{}).Count(&domainAccountCount)
	assert.Equal(t, int64(15), issueCount, "9 issues in project 0-1 + 6 in project 0-2")
	assert.Equal(t, issueCount, boardIssueCount, "every issue joins its board")
	assert.Equal(t, accountCount, domainAccountCount)

	var originalProjects []string
	dataflowTester.Db.Model(&ticket.Issue{}).Distinct().Pluck("original_project", &originalProjects)
	assert.ElementsMatch(t, []string{"PROJ1", "PROJ2"}, originalProjects, "OriginalProject comes from the scope's shortName")

	var issueKeys []string
	dataflowTester.Db.Model(&ticket.Issue{}).Limit(5).Pluck("issue_key", &issueKeys)
	for _, key := range issueKeys {
		assert.Regexp(t, `^PROJ\d-\d+$`, key, "IssueKey is the idReadable")
	}
}

// TestYoutrackConvertIssuesEmitsIssueAssignees covers the issue_assignees
// emission branch (AssigneeId != ""). The live fixture can't reach it — the
// instance's Assignee field is multi-value, which the extractor rejects by
// design — so the tool table is seeded directly.
func TestYoutrackConvertIssuesEmitsIssueAssignees(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_projects.csv", &models.YoutrackProject{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_issues_assignee_seed.csv", &models.YoutrackIssue{})
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.Subtask(tasks.ConvertIssuesMeta, newTaskData("0-1", zeroConfig()))

	dataflowTester.VerifyTableWithOptions(ticket.IssueAssignee{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issue_assignees_from_assignee.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	var assignee ticket.IssueAssignee
	require.NoError(t, dataflowTester.Db.First(&assignee).Error)
	assert.Equal(t, "youtrack:YoutrackIssue:1:2-999", assignee.IssueId)
	assert.Equal(t, "youtrack:YoutrackAccount:1:1-100", assignee.AssigneeId)
	assert.Equal(t, "User 100", assignee.AssigneeName)
}
