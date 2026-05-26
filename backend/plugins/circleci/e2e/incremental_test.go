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
	"sort"
	"testing"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/circleci/impl"
	"github.com/apache/incubator-devlake/plugins/circleci/models"
	"github.com/apache/incubator-devlake/plugins/circleci/tasks"
	"github.com/stretchr/testify/assert"
)

// TestCircleciWorkflowIncremental verifies that the workflow collector's incremental
// logic correctly filters pipelines by created_date. This is a regression test for
// the created_at -> created_date column rename.
func TestCircleciWorkflowIncremental(t *testing.T) {
	var circleci impl.Circleci

	dataflowTester := e2ehelper.NewDataFlowTester(t, "circleci", circleci)
	taskData := &tasks.CircleciTaskData{
		Options: &tasks.CircleciOptions{
			ConnectionId: 1,
			ProjectSlug:  "github/coldgust/coldgust.github.io",
		},
		RegexEnricher: api.NewRegexEnricher(),
	}

	// seed pipelines table via extraction
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_circleci_api_pipelines.csv", "_raw_circleci_api_pipelines")
	dataflowTester.FlushTabler(&models.CircleciPipeline{})
	dataflowTester.Subtask(tasks.ExtractPipelinesMeta, taskData)

	// Part 1: verify the SQL query used by BuildInputIterator works with created_date.
	// Pipelines #4-10 have created_date > 17:45 — assert the exact IDs returned.
	createdAfter := time.Date(2023, 3, 25, 17, 45, 0, 0, time.UTC)
	var pipelines []models.CircleciPipeline
	assert.Nil(t, dataflowTester.Dal.All(&pipelines,
		dal.Where("connection_id = ? AND project_slug = ? AND created_date > ?",
			1, "github/coldgust/coldgust.github.io", createdAfter),
	))
	pipelineIds := make([]string, len(pipelines))
	for i, p := range pipelines {
		pipelineIds[i] = p.Id
	}
	sort.Strings(pipelineIds)
	assert.Equal(t, []string{
		"23622ee4-e150-4920-9d66-81533fa765a4", // pipeline #5
		"2c45280f-7fb3-4025-b703-a547c4a94916", // pipeline #4
		"70f3eb15-3b94-4f80-b65e-f23f4b74c33a", // pipeline #6
		"7fcc1623-edcc-4a76-ad20-cd81aa83519f", // pipeline #9
		"866e967d-f826-4470-aed6-fc0c92e98703", // pipeline #7
		"afe0cabe-e7ee-4eb7-bf13-bb6170d139f0", // pipeline #8
		"d323f088-02fa-4ed5-9696-fc2f89a27150", // pipeline #10
	}, pipelineIds)

	// Part 2: verify extraction with only the incrementally-collected workflow raw data
	// (workflows for pipelines #4-9 only).
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_circleci_api_workflows_incremental.csv", "_raw_circleci_api_workflows")
	dataflowTester.FlushTabler(&models.CircleciWorkflow{})
	dataflowTester.Subtask(tasks.ExtractWorkflowsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.CircleciWorkflow{},
		e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/_tool_circleci_workflows_incremental.csv",
			IgnoreTypes:  []interface{}{common.NoPKModel{}},
			IgnoreFields: []string{"started_at", "stopped_at"},
		},
	)
}

// TestCircleciJobIncremental verifies that the job collector's incremental logic
// correctly filters workflows by created_date. Regression test for the column rename.
func TestCircleciJobIncremental(t *testing.T) {
	var circleci impl.Circleci

	dataflowTester := e2ehelper.NewDataFlowTester(t, "circleci", circleci)
	taskData := &tasks.CircleciTaskData{
		Options: &tasks.CircleciOptions{
			ConnectionId: 1,
			ProjectSlug:  "github/coldgust/coldgust.github.io",
		},
		RegexEnricher: api.NewRegexEnricher(),
		Project: &models.CircleciProject{
			Id: "abcd",
		},
	}

	// seed workflows table via extraction (all 10 workflows including b3b77371 with null created_date)
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_circleci_api_workflows.csv", "_raw_circleci_api_workflows")
	dataflowTester.FlushTabler(&models.CircleciWorkflow{})
	dataflowTester.Subtask(tasks.ExtractWorkflowsMeta, taskData)

	// Part 1: verify the SQL query used by BuildInputIterator works with created_date.
	// Workflows for pipelines #4-9 have created_date > 17:45 — assert the exact IDs returned.
	// Workflow b3b77371 (null created_date) is excluded by the > comparison.
	createdAfter := time.Date(2023, 3, 25, 17, 45, 0, 0, time.UTC)
	var workflows []models.CircleciWorkflow
	assert.Nil(t, dataflowTester.Dal.All(&workflows,
		dal.Where("connection_id = ? AND project_slug = ? AND created_date > ?",
			1, "github/coldgust/coldgust.github.io", createdAfter),
	))
	workflowIds := make([]string, len(workflows))
	for i, w := range workflows {
		workflowIds[i] = w.Id
	}
	sort.Strings(workflowIds)
	assert.Equal(t, []string{
		"6731159f-5275-4bfa-ba70-39d343d63814", // pipeline #5
		"7370985a-9de3-4a47-acbc-e6a1fe8e5812", // pipeline #7
		"b9ab7bbe-2f30-4c59-b4e2-eb2005bffb14", // pipeline #6
		"c7df82a6-0d2b-4e19-a36a-3f3aa9fd3943", // pipeline #4
		"fc76deef-bcdd-4856-8e96-a8e2d1c5a85f", // pipeline #8
		"fd0bd4f5-264f-4e3c-a151-06153c018f78", // pipeline #9
	}, workflowIds)

	// Part 2: verify extraction with only the incrementally-collected job raw data
	// (jobs for workflows from pipelines #4-9 only).
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_circleci_api_projects.csv", "_raw_circleci_api_projects")
	dataflowTester.FlushTabler(&models.CircleciProject{})
	dataflowTester.Subtask(tasks.ExtractProjectsMeta, taskData)

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_circleci_api_jobs_incremental.csv", "_raw_circleci_api_jobs")
	dataflowTester.FlushTabler(&models.CircleciJob{})
	dataflowTester.Subtask(tasks.ExtractJobsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.CircleciJob{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_circleci_jobs_incremental.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
		},
	)
}
