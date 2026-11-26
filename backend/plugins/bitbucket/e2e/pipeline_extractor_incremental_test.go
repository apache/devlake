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
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/runner"
	"github.com/apache/devlake/helpers/e2ehelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	contextimpl "github.com/apache/devlake/impls/context"
	"github.com/apache/devlake/plugins/bitbucket/impl"
	"github.com/apache/devlake/plugins/bitbucket/models"
	"github.com/apache/devlake/plugins/bitbucket/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rawPipelineTable = "_raw_bitbucket_api_pipelines"

// TestBitbucketPipelineExtractorIncremental checks that an incremental run of
// Extract Pipelines reads only the raw rows added since the previous run.
// Bitbucket ignores the updated_on filter on /pipelines, so every collection
// appends a full copy of all pipelines; re-reading every copy made the subtask
// grow without bound.
func TestBitbucketPipelineExtractorIncremental(t *testing.T) {
	var bitbucket impl.Bitbucket
	dataflowTester := e2ehelper.NewDataFlowTester(t, "bitbucket", bitbucket)
	taskData := pipelineTaskData("pipeline")

	dataflowTester.FlushTabler(&coremodels.SubtaskState{})
	dataflowTester.FlushTabler(&coremodels.CollectorLatestState{})
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_bitbucket_api_pipelines.csv", rawPipelineTable)
	dataflowTester.FlushTabler(&models.BitbucketPipeline{})

	runPipelineExtractor(t, dataflowTester, taskData)
	statuses := pipelineStatuses(t, dataflowTester)
	total := len(statuses)
	require.NotZero(t, total)

	const removedPipeline = "{01fad871-cd21-4a80-bd59-74148dd5be8e}" // raw row 1
	const updatedPipeline = "{ee91b6d3-7736-482c-892f-ad1e923d145e}" // raw row 6
	require.Contains(t, statuses, removedPipeline)
	require.NoError(t, dataflowTester.Db.Where("bitbucket_id = ?", removedPipeline).Delete(&models.BitbucketPipeline{}).Error)
	appendUpdatedPipelineRawRow(t, dataflowTester, 6, map[string]any{"state": map[string]any{"name": "IN_PROGRESS"}})

	runPipelineExtractor(t, dataflowTester, taskData)
	statuses = pipelineStatuses(t, dataflowTester)
	assert.Len(t, statuses, total-1)
	assert.NotContains(t, statuses, removedPipeline, "an incremental run must not read old raw rows again")
	assert.Equal(t, "IN_PROGRESS", statuses[updatedPipeline])

	// A changed production pattern changes the type/environment of old rows, so it must trigger a full re-extract.
	runPipelineExtractor(t, dataflowTester, pipelineTaskData("main"))
	statuses = pipelineStatuses(t, dataflowTester)
	assert.Len(t, statuses, total)
	assert.Contains(t, statuses, removedPipeline, "a changed pattern must read all raw rows")
	assert.Equal(t, "IN_PROGRESS", statuses[updatedPipeline], "the newest raw row must win")
}

func pipelineTaskData(productionPattern string) *tasks.BitbucketTaskData {
	regexEnricher := helper.NewRegexEnricher()
	_ = regexEnricher.TryAdd(devops.DEPLOYMENT, "main")
	_ = regexEnricher.TryAdd(devops.PRODUCTION, productionPattern)
	return &tasks.BitbucketTaskData{
		Options: &tasks.BitbucketOptions{
			ConnectionId: 1,
			FullName:     "likyh/likyhphp",
		},
		RegexEnricher: regexEnricher,
	}
}

// runPipelineExtractor runs the subtask without the forced full sync of DataFlowTester.Subtask.
func runPipelineExtractor(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, taskData *tasks.BitbucketTaskData) {
	t.Helper()
	runSubtaskWithSyncPolicy(t, dataflowTester, tasks.ExtractApiPipelinesMeta, taskData)
}

func runSubtaskWithSyncPolicy(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, subtaskMeta plugin.SubTaskMeta, taskData interface{}) {
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

// appendUpdatedPipelineRawRow adds a new raw row, created now, with the data of an existing row and the given changes.
func appendUpdatedPipelineRawRow(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, sourceId uint64, changes map[string]any) {
	t.Helper()
	row := &helper.RawData{}
	require.NoError(t, dataflowTester.Db.Table(rawPipelineTable).First(row, sourceId).Error)
	data := map[string]any{}
	require.NoError(t, json.Unmarshal(row.Data, &data))
	for k, v := range changes {
		data[k] = v
	}
	var err error
	row.Data, err = json.Marshal(data)
	require.NoError(t, err)
	var maxId uint64
	require.NoError(t, dataflowTester.Db.Table(rawPipelineTable).Select("COALESCE(MAX(id), 0)").Row().Scan(&maxId))
	row.ID = maxId + 1
	row.CreatedAt = time.Now()
	require.NoError(t, dataflowTester.Db.Table(rawPipelineTable).Create(row).Error)
}

func pipelineStatuses(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) map[string]string {
	t.Helper()
	var pipelines []models.BitbucketPipeline
	require.NoError(t, dataflowTester.Db.Where("connection_id = ?", 1).Find(&pipelines).Error)
	statuses := make(map[string]string, len(pipelines))
	for _, p := range pipelines {
		statuses[p.BitbucketId] = p.Status
	}
	return statuses
}
