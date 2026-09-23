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
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/youtrack/impl"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/apache/devlake/plugins/youtrack/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYoutrackWorkflowStateDataFlow(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	taskData := &tasks.YoutrackTaskData{
		Options:     &tasks.YoutrackOptions{ConnectionId: 1, ProjectId: "0-1"},
		ScopeConfig: &models.YoutrackScopeConfig{},
	}

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_workflow_states.csv", "_raw_youtrack_workflow_states")
	dataflowTester.FlushTabler(&models.YoutrackWorkflowState{})

	// extract both projects' raw rows (the raw table holds one row per
	// project, filtered by params)
	for _, projectId := range []string{"0-1", "0-2"} {
		taskData.Options.ProjectId = projectId
		dataflowTester.Subtask(tasks.ExtractWorkflowStatesMeta, taskData)
	}

	dataflowTester.VerifyTableWithOptions(models.YoutrackWorkflowState{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_youtrack_workflow_states.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// the fixture's traps, asserted explicitly so an auto-generated snapshot
	// is never the only oracle:
	var resolved, unresolved, archived int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackWorkflowState{}).
		Where("connection_id = 1 AND is_resolved").Count(&resolved).Error)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackWorkflowState{}).
		Where("connection_id = 1 AND NOT is_resolved").Count(&unresolved).Error)
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackWorkflowState{}).
		Where("connection_id = 1 AND archived").Count(&archived).Error)
	assert.Positive(t, resolved, "fixture must carry resolved states (zero-config DONE signal)")
	assert.Positive(t, unresolved, "fixture must carry unresolved states")
	assert.Positive(t, archived, "fixture must carry archived bundle values")

	// mixed-language workflow vocabulary survives extraction verbatim
	var cyrillic int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackWorkflowState{}).
		Where("connection_id = 1 AND name REGEXP '[^ -~]'").Count(&cyrillic).Error)
	assert.Greater(t, cyrillic, int64(0), "mixed-language state names (e.g. Новый) must be preserved")
}

// TestYoutrackWorkflowStateFullRefresh covers the "full refresh each
// run": a state renamed or removed upstream must not linger in the table.
func TestYoutrackWorkflowStateFullRefresh(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)
	taskData := &tasks.YoutrackTaskData{
		Options:     &tasks.YoutrackOptions{ConnectionId: 1, ProjectId: "0-1"},
		ScopeConfig: &models.YoutrackScopeConfig{},
	}

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_youtrack_workflow_states.csv", "_raw_youtrack_workflow_states")
	dataflowTester.FlushTabler(&models.YoutrackWorkflowState{})
	dataflowTester.Subtask(tasks.ExtractWorkflowStatesMeta, taskData)
	otherScope := &tasks.YoutrackTaskData{
		Options:     &tasks.YoutrackOptions{ConnectionId: 1, ProjectId: "0-2"},
		ScopeConfig: &models.YoutrackScopeConfig{},
	}
	dataflowTester.Subtask(tasks.ExtractWorkflowStatesMeta, otherScope)

	var otherScopeRowsBefore int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackWorkflowState{}).
		Where("connection_id = 1 AND project_id = '0-2'").Count(&otherScopeRowsBefore).Error)
	require.Positive(t, otherScopeRowsBefore)

	// a stale row from an earlier run (state since renamed upstream)
	stale := &models.YoutrackWorkflowState{
		ConnectionId: 1, ProjectId: "0-1", Id: "94-99999", Name: "Old Name",
	}
	require.NoError(t, dataflowTester.Db.Create(stale).Error)
	dataflowTester.Subtask(tasks.ExtractWorkflowStatesMeta, taskData)

	var staleCount int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackWorkflowState{}).
		Where("connection_id = 1 AND project_id = '0-1' AND id = ?", stale.Id).Count(&staleCount).Error)
	assert.Zero(t, staleCount, "full refresh replaces the scope's rows")

	var otherScopeRowsAfter int64
	require.NoError(t, dataflowTester.Db.Model(&models.YoutrackWorkflowState{}).
		Where("connection_id = 1 AND project_id = '0-2'").Count(&otherScopeRowsAfter).Error)
	assert.Equal(t, otherScopeRowsBefore, otherScopeRowsAfter, "the refresh is per-scope")
}
