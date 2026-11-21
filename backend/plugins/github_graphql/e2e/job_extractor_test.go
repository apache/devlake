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

	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/runner"
	"github.com/apache/devlake/helpers/e2ehelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	contextimpl "github.com/apache/devlake/impls/context"
	"github.com/apache/devlake/plugins/github/models"
	"github.com/apache/devlake/plugins/github/tasks"
	"github.com/apache/devlake/plugins/github_graphql/impl"
	githubGraphQLTasks "github.com/apache/devlake/plugins/github_graphql/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rawGraphqlJobsTable = "_raw_github_graphql_jobs"

// TestGithubGraphqlJobExtractorIncremental checks that an incremental run of "Extract Jobs"
// reads only the raw rows added since the previous run and leaves the other jobs alone.
// "Convert Jobs" selects the jobs to convert by `updated_at`, so a job that is written again
// without a change is also converted again (#9421).
func TestGithubGraphqlJobExtractorIncremental(t *testing.T) {
	var githubGraphql impl.GithubGraphql
	dataflowTester := e2ehelper.NewDataFlowTester(t, "github_graphql", githubGraphql)
	regexEnricher := helper.NewRegexEnricher()
	require.NoError(t, regexEnricher.TryAdd(devops.DEPLOYMENT, "Build and Push"))
	taskData := &tasks.GithubTaskData{
		Options: &tasks.GithubOptions{
			ConnectionId: 1,
			Name:         "apache/devlake",
			GithubId:     384111310,
		},
		RegexEnricher: regexEnricher,
	}
	const (
		unchangedJob = 135513855832 // "lint for yamls", finished before the first run
		updatedJob   = 135513855476 // "e2e-mysql", still running at the first run
	)

	// no stored state, so the first run extracts every raw row
	dataflowTester.FlushTabler(&coreModels.SubtaskState{})
	dataflowTester.FlushTabler(&coreModels.CollectorLatestState{})
	dataflowTester.FlushTabler(&models.GithubJob{})
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_github_graphql_jobs.csv", rawGraphqlJobsTable)

	runExtractJobs(t, dataflowTester, taskData, false)
	dataflowTester.VerifyTableWithOptions(&models.GithubJob{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_github_jobs.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	firstRun := githubJobs(t, dataflowTester)

	// the collector adds a raw row for the job that finished after the first run
	appendFinishedRawJob(t, dataflowTester, updatedJob)
	runExtractJobs(t, dataflowTester, taskData, false)
	incrementalRun := githubJobs(t, dataflowTester)
	require.Len(t, incrementalRun, len(firstRun))
	for id, job := range incrementalRun {
		if id == updatedJob {
			assert.Equal(t, "COMPLETED", job.Status)
			assert.Equal(t, "SUCCESS", job.Conclusion)
			assert.NotNil(t, job.CompletedAt)
			continue
		}
		assert.Equal(t, firstRun[id].UpdatedAt, job.UpdatedAt, "an incremental run must not write job %d again", id)
	}

	// Type and Environment depend on the scope config patterns: after a change every raw row is extracted again
	assert.Equal(t, devops.PRODUCTION, incrementalRun[unchangedJob].Environment)
	require.NoError(t, regexEnricher.TryAdd(devops.PRODUCTION, "Build and Push"))
	runExtractJobs(t, dataflowTester, taskData, false)
	reconfiguredRun := githubJobs(t, dataflowTester)
	require.Len(t, reconfiguredRun, len(firstRun))
	assert.Empty(t, reconfiguredRun[unchangedJob].Environment)
	assert.Equal(t, "COMPLETED", reconfiguredRun[updatedJob].Status, "the newest raw row must win")

	// a job that is missing in the tool layer comes back with a full sync, not with an incremental run
	require.NoError(t, dataflowTester.Db.Where("id = ?", unchangedJob).Delete(&models.GithubJob{}).Error)
	runExtractJobs(t, dataflowTester, taskData, false)
	_, extractedAgain := githubJobs(t, dataflowTester)[unchangedJob]
	assert.False(t, extractedAgain, "an incremental run must not read old raw rows again")
	runExtractJobs(t, dataflowTester, taskData, true)
	fullSyncRun := githubJobs(t, dataflowTester)
	require.Len(t, fullSyncRun, len(firstRun))
	assert.Equal(t, "COMPLETED", fullSyncRun[updatedJob].Status, "the newest raw row must win")
}

// runExtractJobs runs "Extract Jobs" and keeps the subtask state, which DataFlowTester.Subtask
// does not: it removes the state and forces a full sync on every call.
func runExtractJobs(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, taskData *tasks.GithubTaskData, fullSync bool) {
	t.Helper()
	subtaskCtx := contextimpl.NewStandaloneSubTaskContext(
		context.Background(),
		runner.CreateBasicRes(dataflowTester.Cfg, dataflowTester.Log, dataflowTester.Db),
		githubGraphQLTasks.ExtractJobsMeta.Name,
		taskData,
		dataflowTester.Name,
		&coreModels.SyncPolicy{TriggerSyncPolicy: coreModels.TriggerSyncPolicy{FullSync: fullSync}},
	)
	require.NoError(t, githubGraphQLTasks.ExtractJobsMeta.EntryPoint(subtaskCtx))
}

// appendFinishedRawJob adds a raw row, created now, with the finished state of a job that
// already has a raw row. This is what the collector does for a run that was updated.
func appendFinishedRawJob(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, jobId int) {
	t.Helper()
	var rows []helper.RawData
	require.NoError(t, dataflowTester.Db.Table(rawGraphqlJobsTable).Order("id ASC").Find(&rows).Error)
	for _, row := range rows {
		checkRun := &githubGraphQLTasks.DbCheckRun{}
		require.NoError(t, json.Unmarshal(row.Data, checkRun))
		if checkRun.DatabaseId != jobId {
			continue
		}
		completedAt := checkRun.StartedAt.Add(5 * time.Minute)
		checkRun.Status = "COMPLETED"
		checkRun.Conclusion = "SUCCESS"
		checkRun.CompletedAt = &completedAt
		data, err := json.Marshal(checkRun)
		require.NoError(t, err)
		row.ID = rows[len(rows)-1].ID + 1
		row.Data = data
		row.CreatedAt = time.Now()
		require.NoError(t, dataflowTester.Db.Table(rawGraphqlJobsTable).Create(&row).Error)
		return
	}
	require.Fail(t, "raw row of the job not found", "job %d", jobId)
}

func githubJobs(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) map[int]models.GithubJob {
	t.Helper()
	var jobs []models.GithubJob
	require.NoError(t, dataflowTester.Db.Find(&jobs).Error)
	jobsById := make(map[int]models.GithubJob, len(jobs))
	for _, job := range jobs {
		jobsById[job.ID] = job
	}
	return jobsById
}
