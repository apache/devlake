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

package services

import (
	"testing"

	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/impls/logruslog"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testGitUrl       = "https://git:secret-token@example.com/group/repo.git"
	testMaskedGitUrl = "https://git:************@example.com/group/repo.git"
)

func TestSanitizeBlueprintLeavesTheSourcePlanUnchanged(t *testing.T) {
	bp := &coreModels.Blueprint{
		Plan: coreModels.PipelinePlan{{
			{Plugin: "gitextractor", Options: map[string]interface{}{"url": testGitUrl, "name": "group/repo"}},
		}},
	}

	sanitized := *bp
	require.NoError(t, SanitizeBlueprint(&sanitized))

	assert.Equal(t, map[string]interface{}{"url": testMaskedGitUrl, "name": "group/repo"}, sanitized.Plan[0][0].Options)
	assert.Equal(t, map[string]interface{}{"url": testGitUrl, "name": "group/repo"}, bp.Plan[0][0].Options)
}

func TestPatchBlueprintKeepsTheScheduledPlanUnsanitized(t *testing.T) {
	withProjectTestValidator(t)
	testDB := mockdal.NewDal(t)
	withProjectTestDatabase(t, testDB)
	previousLogger, previousCron, previousCronIds := logger, cronManager, bpCronIdMap
	logger, cronManager, bpCronIdMap = logruslog.Global, cron.New(), map[uint64]cron.EntryID{}
	t.Cleanup(func() {
		logger, cronManager, bpCronIdMap = previousLogger, previousCron, previousCronIds
	})

	testDB.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		bp := args.Get(0).(*coreModels.Blueprint)
		bp.ID = 9
		bp.Name = "advanced"
		bp.Mode = coreModels.BLUEPRINT_MODE_ADVANCED
		bp.Enable = true
		bp.CronConfig = "0 0 * * *"
		bp.Plan = coreModels.PipelinePlan{{
			{Plugin: "gitextractor", Options: map[string]interface{}{"url": testGitUrl}},
		}}
	}).Return(nil).Once()
	testDB.On("Pluck", "name", mock.Anything, mock.Anything).Return(nil).Once()
	testDB.On("All", mock.Anything, mock.Anything).Return(nil).Once()
	testDB.On("Update", mock.Anything).Return(nil).Once()
	testDB.On("Delete", mock.Anything, mock.Anything).Return(nil).Times(3)

	patched, err := PatchBlueprint(9, map[string]interface{}{"cronConfig": "*/5 * * * *"})
	require.NoError(t, err)
	assert.Equal(t, testMaskedGitUrl, patched.Plan[0][0].Options["url"])

	// the cron job must still run the stored plan, not the one sanitized for the response
	entries := cronManager.Entries()
	require.Len(t, entries, 1)
	job, ok := entries[0].Job.(*BlueprintJob)
	require.True(t, ok)
	assert.Equal(t, "*/5 * * * *", job.Blueprint.CronConfig)
	assert.Equal(t, testGitUrl, job.Blueprint.Plan[0][0].Options["url"])
}

func TestReloadBlueprintByIdSchedulesTheStoredBlueprint(t *testing.T) {
	testDB := mockdal.NewDal(t)
	withProjectTestDatabase(t, testDB)
	previousLogger, previousCron, previousCronIds := logger, cronManager, bpCronIdMap
	logger, cronManager, bpCronIdMap = logruslog.Global, cron.New(), map[uint64]cron.EntryID{}
	t.Cleanup(func() {
		logger, cronManager, bpCronIdMap = previousLogger, previousCron, previousCronIds
	})

	// the job scheduled before a webhook connection was attached to the blueprint
	stale := &coreModels.Blueprint{Enable: true, CronConfig: "0 0 * * *"}
	stale.ID = 9
	require.NoError(t, reloadBlueprint(stale))

	testDB.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		bp := args.Get(0).(*coreModels.Blueprint)
		bp.ID = 9
		bp.Enable = true
		bp.CronConfig = "0 0 * * *"
	}).Return(nil).Once()
	testDB.On("Pluck", "name", mock.Anything, mock.Anything).Return(nil).Once()
	testDB.On("All", mock.AnythingOfType("*[]*models.BlueprintConnection"), mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*[]*coreModels.BlueprintConnection) = []*coreModels.BlueprintConnection{
			{BlueprintId: 9, PluginName: "webhook", ConnectionId: 7},
		}
	}).Return(nil).Once()
	testDB.On("All", mock.AnythingOfType("*[]*models.BlueprintScope"), mock.Anything).Return(nil).Once()

	require.NoError(t, ReloadBlueprintById(9))

	// the stale job is replaced by one that holds the blueprint as it is stored now
	entries := cronManager.Entries()
	require.Len(t, entries, 1)
	job, ok := entries[0].Job.(*BlueprintJob)
	require.True(t, ok)
	require.Len(t, job.Blueprint.Connections, 1)
	assert.Equal(t, "webhook", job.Blueprint.Connections[0].PluginName)
	assert.Equal(t, uint64(7), job.Blueprint.Connections[0].ConnectionId)
}

func TestParallelizePipelineTasks(t *testing.T) {
	plan1 := coreModels.PipelinePlan{
		{
			{Plugin: "github"},
			{Plugin: "gitlab"},
		},
		{
			{Plugin: "gitextractor1"},
			{Plugin: "gitextractor2"},
		},
	}

	plan2 := coreModels.PipelinePlan{
		{
			{Plugin: "jira"},
		},
	}

	plan3 := coreModels.PipelinePlan{
		{
			{Plugin: "jenkins"},
		},
		{
			{Plugin: "jenkins"},
		},
		{
			{Plugin: "jenkins"},
		},
	}

	assert.Equal(t, plan1, ParallelizePipelinePlans(plan1))
	assert.Equal(t, plan2, ParallelizePipelinePlans(plan2))
	assert.Equal(
		t,
		coreModels.PipelinePlan{
			{
				{Plugin: "github"},
				{Plugin: "gitlab"},
				{Plugin: "jira"},
			},
			{
				{Plugin: "gitextractor1"},
				{Plugin: "gitextractor2"},
			},
		},
		ParallelizePipelinePlans(plan1, plan2),
	)
	assert.Equal(
		t,
		coreModels.PipelinePlan{
			{
				{Plugin: "github"},
				{Plugin: "gitlab"},
				{Plugin: "jira"},
				{Plugin: "jenkins"},
			},
			{
				{Plugin: "gitextractor1"},
				{Plugin: "gitextractor2"},
				{Plugin: "jenkins"},
			},
			{
				{Plugin: "jenkins"},
			},
		},
		ParallelizePipelinePlans(plan1, plan2, plan3),
	)
}

func TestRemoveCollectorTasks(t *testing.T) {
	plan1 := coreModels.PipelinePlan{
		{
			{
				Plugin: "github",
				Subtasks: []string{
					"CollectApiPipelines",
					"ExtractApiPipelines",
					"collectApiPipelineDetails",
					"extractApiPipelineDetails",
					"collectApiJobs",
					"extractApiJobs",
					"collectAccounts",
					"extractAccounts",
					"ConvertAccounts",
					"convertApiProject",
					"convertPipelines",
					"convertPipelineCommits",
					"convertJobs",
				},
			},
		},
		{
			{
				Plugin:   "starrocks",
				Subtasks: []string{},
			},
		},
	}
	assert.Equal(t, coreModels.PipelinePlan{
		{
			{
				Plugin: "github",
				Subtasks: []string{
					"ExtractApiPipelines",
					"extractApiPipelineDetails",
					"extractApiJobs",
					"extractAccounts",
					"ConvertAccounts",
					"convertApiProject",
					"convertPipelines",
					"convertPipelineCommits",
					"convertJobs",
				},
			},
		},
		{
			{
				Plugin:   "starrocks",
				Subtasks: []string{},
			},
		},
	}, removeCollectorTasks(plan1))
}
