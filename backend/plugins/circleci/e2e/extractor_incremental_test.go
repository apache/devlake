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
	"context"
	"encoding/json"
	"testing"
	"time"

	coremodels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/runner"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	contextimpl "github.com/apache/devlake/impls/context"
	"github.com/apache/devlake/plugins/circleci/impl"
	"github.com/apache/devlake/plugins/circleci/models"
	"github.com/apache/devlake/plugins/circleci/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCircleciJobExtractorIncremental checks that an incremental run of extractJobs
// reads only the raw rows added since the previous run (#9305).
func TestCircleciJobExtractorIncremental(t *testing.T) {
	var circleci impl.Circleci
	dataflowTester := e2ehelper.NewDataFlowTester(t, "circleci", circleci)
	taskData := &tasks.CircleciTaskData{
		Options: &tasks.CircleciOptions{
			ConnectionId: 1,
			ProjectSlug:  "github/coldgust/coldgust.github.io",
		},
	}
	beginStatefulScenario(dataflowTester)
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_circleci_api_jobs.csv", "_raw_circleci_api_jobs")
	dataflowTester.FlushTabler(&models.CircleciJob{})

	runSubtaskPreservingState(t, dataflowTester, tasks.ExtractJobsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.CircleciJob{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_circleci_jobs.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
		},
	)

	const removedJob = "7b96e45b-b10e-47a0-95d0-96580b88bdda"
	const updatedJob = "ab8c3282-0e74-4a41-834e-152a71280bed"
	require.NoError(t, dataflowTester.Db.Where("id = ?", removedJob).Delete(&models.CircleciJob{}).Error)
	appendUpdatedRawRow(t, dataflowTester, "_raw_circleci_api_jobs", 20, map[string]any{"status": "success"})

	runSubtaskPreservingState(t, dataflowTester, tasks.ExtractJobsMeta, taskData)
	statuses := jobStatuses(t, dataflowTester)
	assert.Len(t, statuses, 8)
	assert.NotContains(t, statuses, removedJob, "an incremental run must not read old raw rows again")
	assert.Equal(t, "success", statuses[updatedJob])

	dataflowTester.Subtask(tasks.ExtractJobsMeta, taskData)
	statuses = jobStatuses(t, dataflowTester)
	assert.Len(t, statuses, 9)
	assert.Contains(t, statuses, removedJob, "a full sync must read all raw rows")
	assert.Equal(t, "success", statuses[updatedJob], "the newest raw row must win")
}

// TestCircleciWorkflowExtractorIncremental checks that an incremental run of
// extractWorkflows reads only the raw rows added since the previous run (#9305).
func TestCircleciWorkflowExtractorIncremental(t *testing.T) {
	var circleci impl.Circleci
	dataflowTester := e2ehelper.NewDataFlowTester(t, "circleci", circleci)
	taskData := &tasks.CircleciTaskData{
		Options: &tasks.CircleciOptions{
			ConnectionId: 1,
			ProjectSlug:  "github/coldgust/coldgust.github.io",
		},
	}
	beginStatefulScenario(dataflowTester)
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_circleci_api_workflows.csv", "_raw_circleci_api_workflows")
	dataflowTester.FlushTabler(&models.CircleciWorkflow{})

	runSubtaskPreservingState(t, dataflowTester, tasks.ExtractWorkflowsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.CircleciWorkflow{},
		e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/_tool_circleci_workflows.csv",
			IgnoreTypes:  []interface{}{common.NoPKModel{}},
			IgnoreFields: []string{"started_at", "stopped_at"},
		},
	)

	const removedWorkflow = "89054eb2-8e85-4f5c-9a93-66d753a0e970"
	const updatedWorkflow = "6731159f-5275-4bfa-ba70-39d343d63814"
	require.NoError(t, dataflowTester.Db.Where("id = ?", removedWorkflow).Delete(&models.CircleciWorkflow{}).Error)
	appendUpdatedRawRow(t, dataflowTester, "_raw_circleci_api_workflows", 19, map[string]any{"status": "success"})

	runSubtaskPreservingState(t, dataflowTester, tasks.ExtractWorkflowsMeta, taskData)
	statuses := workflowStatuses(t, dataflowTester)
	assert.Len(t, statuses, 9)
	assert.NotContains(t, statuses, removedWorkflow, "an incremental run must not read old raw rows again")
	assert.Equal(t, "success", statuses[updatedWorkflow])

	dataflowTester.Subtask(tasks.ExtractWorkflowsMeta, taskData)
	statuses = workflowStatuses(t, dataflowTester)
	assert.Len(t, statuses, 10)
	assert.Contains(t, statuses, removedWorkflow, "a full sync must read all raw rows")
	assert.Equal(t, "success", statuses[updatedWorkflow], "the newest raw row must win")
}

// beginStatefulScenario removes the stored states, so that the first extractor run is a full sync.
func beginStatefulScenario(dataflowTester *e2ehelper.DataFlowTester) {
	dataflowTester.FlushTabler(&coremodels.SubtaskState{})
	dataflowTester.FlushTabler(&coremodels.CollectorLatestState{})
}

// runSubtaskPreservingState runs a subtask without the forced full sync of DataFlowTester.Subtask.
func runSubtaskPreservingState(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, subtaskMeta plugin.SubTaskMeta, taskData interface{}) {
	t.Helper()
	subtaskCtx := contextimpl.NewStandaloneSubTaskContext(
		context.Background(),
		runner.CreateBasicRes(dataflowTester.Cfg, dataflowTester.Log, dataflowTester.Db),
		dataflowTester.Name,
		taskData,
		subtaskMeta.Name,
		&coremodels.SyncPolicy{},
	)
	require.NoError(t, subtaskMeta.EntryPoint(subtaskCtx))
}

// appendUpdatedRawRow adds a new raw row, created now, with the data of an existing row and the given changes.
func appendUpdatedRawRow(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, table string, sourceId uint64, changes map[string]any) {
	t.Helper()
	row := &api.RawData{}
	require.NoError(t, dataflowTester.Db.Table(table).First(row, sourceId).Error)
	data := map[string]any{}
	require.NoError(t, json.Unmarshal(row.Data, &data))
	for k, v := range changes {
		data[k] = v
	}
	var err error
	row.Data, err = json.Marshal(data)
	require.NoError(t, err)
	var maxId uint64
	require.NoError(t, dataflowTester.Db.Table(table).Select("COALESCE(MAX(id), 0)").Row().Scan(&maxId))
	row.ID = maxId + 1
	row.CreatedAt = time.Now()
	require.NoError(t, dataflowTester.Db.Table(table).Create(row).Error)
}

func jobStatuses(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) map[string]string {
	t.Helper()
	var jobs []models.CircleciJob
	require.NoError(t, dataflowTester.Db.Where("connection_id = ?", 1).Find(&jobs).Error)
	statuses := make(map[string]string, len(jobs))
	for _, j := range jobs {
		statuses[j.Id] = j.Status
	}
	return statuses
}

func workflowStatuses(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) map[string]string {
	t.Helper()
	var workflows []models.CircleciWorkflow
	require.NoError(t, dataflowTester.Db.Where("connection_id = ?", 1).Find(&workflows).Error)
	statuses := make(map[string]string, len(workflows))
	for _, w := range workflows {
		statuses[w.Id] = w.Status
	}
	return statuses
}
