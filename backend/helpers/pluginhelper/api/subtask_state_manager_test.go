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

package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
	mockdal "github.com/apache/incubator-devlake/mocks/core/dal"
	mockplugin "github.com/apache/incubator-devlake/mocks/core/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testGithubScopeParams = `{"ConnectionId":1,"Name":"test/repo"}`

func TestSubtaskStateManager(t *testing.T) {
	time0 := errors.Must1(time.Parse(time.RFC3339, "2020-01-01T00:00:00Z"))
	time1 := errors.Must1(time.Parse(time.RFC3339, "2021-01-01T00:00:00Z"))
	time2 := errors.Must1(time.Parse(time.RFC3339, "2022-01-01T00:00:00Z"))
	for _, tc := range []struct {
		name                      string
		state                     *models.SubtaskState
		syncPolicy                *models.SyncPolicy
		config                    string
		expectedIsIncremental     bool
		expectedSince             *time.Time
		expectedNewStateTimeAfter *time.Time
	}{
		{
			name:                      "syncPolicy has no timeAfter - First run",
			state:                     &models.SubtaskState{PrevStartedAt: nil},
			syncPolicy:                &models.SyncPolicy{TimeAfter: nil},
			expectedIsIncremental:     false,
			expectedSince:             nil,
			expectedNewStateTimeAfter: nil,
		},
		{
			name:                      "syncPolicy has no timeAfter - Second run",
			state:                     &models.SubtaskState{PrevStartedAt: &time1},
			syncPolicy:                &models.SyncPolicy{TimeAfter: nil},
			expectedIsIncremental:     true,
			expectedSince:             &time1,
			expectedNewStateTimeAfter: nil,
		},
		{
			name:                      "syncPolicy has no timeAfter - Third run with timeAfter specified",
			state:                     &models.SubtaskState{PrevStartedAt: &time1},
			syncPolicy:                &models.SyncPolicy{TimeAfter: &time1},
			expectedIsIncremental:     true,
			expectedSince:             &time1,
			expectedNewStateTimeAfter: nil,
		},
		{
			name:                      "syncPolicy has timeAfter - First run",
			state:                     &models.SubtaskState{PrevStartedAt: nil},
			syncPolicy:                &models.SyncPolicy{TimeAfter: &time1},
			expectedIsIncremental:     false,
			expectedSince:             &time1,
			expectedNewStateTimeAfter: &time1,
		},
		{
			name:                      "syncPolicy has timeAfter - Second run with a later timeAfter",
			state:                     &models.SubtaskState{TimeAfter: &time1, PrevStartedAt: &time2},
			syncPolicy:                &models.SyncPolicy{TimeAfter: &time2},
			expectedIsIncremental:     true,
			expectedSince:             &time2,
			expectedNewStateTimeAfter: &time1,
		},
		{
			name:                      "syncPolicy has timeAfter - Third run with a earlier timeAfter",
			state:                     &models.SubtaskState{TimeAfter: &time1, PrevStartedAt: &time1},
			syncPolicy:                &models.SyncPolicy{TimeAfter: &time0},
			expectedIsIncremental:     false,
			expectedSince:             &time0,
			expectedNewStateTimeAfter: &time0,
		},
		{
			name:                      "syncPolicy has timeAfter - Fourth run with a same timeAfter",
			state:                     &models.SubtaskState{TimeAfter: &time1, PrevStartedAt: &time2},
			syncPolicy:                &models.SyncPolicy{TimeAfter: &time1},
			expectedIsIncremental:     true,
			expectedSince:             &time2,
			expectedNewStateTimeAfter: &time1,
		},
		{
			name:                      "config no changed",
			state:                     &models.SubtaskState{TimeAfter: &time1, PrevStartedAt: &time2, PrevConfig: `"hello"`},
			syncPolicy:                &models.SyncPolicy{TimeAfter: &time1},
			config:                    "hello",
			expectedIsIncremental:     true,
			expectedSince:             &time2,
			expectedNewStateTimeAfter: &time1,
		},
		{
			name:                      "Full sync - with timeAfter",
			state:                     &models.SubtaskState{TimeAfter: &time1, PrevStartedAt: &time1},
			syncPolicy:                &models.SyncPolicy{TriggerSyncPolicy: models.TriggerSyncPolicy{FullSync: true}},
			expectedIsIncremental:     false,
			expectedSince:             &time1,
			expectedNewStateTimeAfter: &time1,
		},
		{
			name:                      "Full sync - with newer timeAfter",
			state:                     &models.SubtaskState{TimeAfter: &time1, PrevStartedAt: &time1},
			syncPolicy:                &models.SyncPolicy{TimeAfter: &time2, TriggerSyncPolicy: models.TriggerSyncPolicy{FullSync: true}},
			expectedIsIncremental:     false,
			expectedSince:             &time2,
			expectedNewStateTimeAfter: &time2,
		},
		{
			name:                      "Full sync - with older timeAfter",
			state:                     &models.SubtaskState{TimeAfter: &time1, PrevStartedAt: &time1},
			syncPolicy:                &models.SyncPolicy{TimeAfter: &time0, TriggerSyncPolicy: models.TriggerSyncPolicy{FullSync: true}},
			expectedIsIncremental:     false,
			expectedSince:             &time0,
			expectedNewStateTimeAfter: &time0,
		},
		{
			name:                      "Full sync - without timeAfter",
			state:                     &models.SubtaskState{TimeAfter: nil, PrevStartedAt: &time1},
			syncPolicy:                &models.SyncPolicy{TriggerSyncPolicy: models.TriggerSyncPolicy{FullSync: true}},
			expectedIsIncremental:     false,
			expectedSince:             nil,
			expectedNewStateTimeAfter: nil,
		},
		{
			name:                      "Full sync - config changed",
			state:                     &models.SubtaskState{PrevStartedAt: &time1, PrevConfig: "hello"},
			syncPolicy:                &models.SyncPolicy{},
			config:                    "world",
			expectedIsIncremental:     false,
			expectedSince:             nil,
			expectedNewStateTimeAfter: nil,
		},
	} {
		started := time.Now()
		t.Run(tc.name, func(t *testing.T) {
			// mockBasicRes := unithelper.DummyBasicRes(func(mockDal *mockdal.Dal) {
			mockDal := new(mockdal.Dal)
			mockDal.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				dst := args.Get(0).(*models.SubtaskState)
				*dst = *tc.state
			}).Return(nil).Once()
			mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil).Once()
			// })

			// mockBasicRes, tc.syncPolicy, "table", "params"
			mockTaskCtx := new(mockplugin.TaskContext)
			mockTaskCtx.On("SyncPolicy").Return(tc.syncPolicy)
			mockTaskCtx.On("GetName").Return("test-plugin")
			mockSubtaskCtx := new(mockplugin.SubTaskContext)
			mockSubtaskCtx.On("TaskContext").Return(mockTaskCtx)
			mockSubtaskCtx.On("GetName").Return("test-subtask")
			mockSubtaskCtx.On("GetDal").Return(mockDal)

			stateManager, err := NewSubtaskStateManager(&SubtaskCommonArgs{
				SubTaskContext: mockSubtaskCtx,
				SubtaskConfig:  fmt.Sprintf("%v", tc.config),
				Params:         "whatever",
			})
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedSince, stateManager.since)
			assert.Equal(t, tc.expectedIsIncremental, stateManager.isIncremental)
			assert.Nil(t, stateManager.Close())
			assert.Equal(t, tc.expectedNewStateTimeAfter, stateManager.state.TimeAfter)
			// PrevStartedAt should be updated
			assert.GreaterOrEqual(t, stateManager.state.PrevStartedAt.Unix(), started.Unix())
			// First and update should both be called once
			mockDal.AssertExpectations(t)
		})
	}
}

func TestBootstrapStateFromCollectorStateIfNeeded(t *testing.T) {
	latest := errors.Must1(time.Parse(time.RFC3339, "2026-04-27T18:05:31Z"))
	timeAfter := errors.Must1(time.Parse(time.RFC3339, "2024-12-20T00:00:00Z"))

	t.Run("bootstrap prev_started_at and time_after from collector state", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("HasTable", mock.Anything).Return(true).Once()
		mockDal.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*models.CollectorLatestState)
			dst.LatestSuccessStart = &latest
			dst.TimeAfter = &timeAfter
		}).Return(nil).Once()

		state := &models.SubtaskState{
			Plugin:  "github",
			Subtask: "Convert Jobs",
			Params:  testGithubScopeParams,
		}
		args := &SubtaskCommonArgs{Table: "github_api_jobs"}

		bootstrapped, err := bootstrapStateFromCollectorStateIfNeeded(mockDal, state, args)
		assert.Nil(t, err)
		if assert.NotNil(t, bootstrapped.PrevStartedAt) {
			assert.True(t, bootstrapped.PrevStartedAt.Equal(latest))
		}
		if assert.NotNil(t, bootstrapped.TimeAfter) {
			assert.True(t, bootstrapped.TimeAfter.Equal(timeAfter))
		}
		mockDal.AssertExpectations(t)
	})

	t.Run("do not overwrite existing prev_started_at", func(t *testing.T) {
		mockDal := new(mockdal.Dal)

		existing := errors.Must1(time.Parse(time.RFC3339, "2026-04-27T18:06:37Z"))
		state := &models.SubtaskState{
			Plugin:        "github",
			Subtask:       "Convert Jobs",
			Params:        testGithubScopeParams,
			PrevStartedAt: &existing,
		}
		args := &SubtaskCommonArgs{Table: "github_api_jobs"}

		bootstrapped, err := bootstrapStateFromCollectorStateIfNeeded(mockDal, state, args)
		assert.Nil(t, err)
		if assert.NotNil(t, bootstrapped.PrevStartedAt) {
			assert.True(t, bootstrapped.PrevStartedAt.Equal(existing))
		}
		mockDal.AssertNotCalled(t, "HasTable", mock.Anything)
		mockDal.AssertNotCalled(t, "First", mock.Anything, mock.Anything)
	})

	t.Run("ignore not found collector state", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		notFoundErr := errors.Default.New("record not found")
		mockDal.On("HasTable", mock.Anything).Return(true).Once()
		mockDal.On("First", mock.Anything, mock.Anything).Return(notFoundErr).Once()
		mockDal.On("IsErrorNotFound", notFoundErr).Return(true).Once()

		state := &models.SubtaskState{
			Plugin:  "github",
			Subtask: "Convert Jobs",
			Params:  testGithubScopeParams,
		}
		args := &SubtaskCommonArgs{Table: "github_api_jobs"}

		bootstrapped, err := bootstrapStateFromCollectorStateIfNeeded(mockDal, state, args)
		assert.Nil(t, err)
		assert.Nil(t, bootstrapped.PrevStartedAt)
		assert.Nil(t, bootstrapped.TimeAfter)
		mockDal.AssertExpectations(t)
	})

	t.Run("return error when collector state query fails", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		dbErr := errors.Default.New("db unavailable")
		mockDal.On("HasTable", mock.Anything).Return(true).Once()
		mockDal.On("First", mock.Anything, mock.Anything).Return(dbErr).Once()
		mockDal.On("IsErrorNotFound", dbErr).Return(false).Once()

		state := &models.SubtaskState{
			Plugin:  "github",
			Subtask: "Convert Jobs",
			Params:  testGithubScopeParams,
		}
		args := &SubtaskCommonArgs{Table: "github_api_jobs"}

		bootstrapped, err := bootstrapStateFromCollectorStateIfNeeded(mockDal, state, args)
		assert.Nil(t, bootstrapped)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "failed to load collector state for subtask bootstrap")
		mockDal.AssertExpectations(t)
	})

	t.Run("ignore missing collector table errors", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("HasTable", mock.Anything).Return(true).Once()
		tableErr := errors.Default.New("Error 1146 (42S02): Table 'lake._devlake_collector_latest_state' doesn't exist")
		mockDal.On("First", mock.Anything, mock.Anything).Return(tableErr).Once()
		mockDal.On("IsErrorNotFound", tableErr).Return(false).Once()

		state := &models.SubtaskState{
			Plugin:  "github",
			Subtask: "Convert Jobs",
			Params:  testGithubScopeParams,
		}
		args := &SubtaskCommonArgs{Table: "github_api_jobs"}

		bootstrapped, err := bootstrapStateFromCollectorStateIfNeeded(mockDal, state, args)
		assert.Nil(t, err)
		assert.NotNil(t, bootstrapped)
		assert.Nil(t, bootstrapped.PrevStartedAt)
		assert.Nil(t, bootstrapped.TimeAfter)
		mockDal.AssertExpectations(t)
	})

	t.Run("do not ignore missing unrelated table errors", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("HasTable", mock.Anything).Return(true).Once()
		tableErr := errors.Default.New("Error 1146 (42S02): Table 'lake._tool_github_issues' doesn't exist")
		mockDal.On("First", mock.Anything, mock.Anything).Return(tableErr).Once()
		mockDal.On("IsErrorNotFound", tableErr).Return(false).Once()

		state := &models.SubtaskState{
			Plugin:  "github",
			Subtask: "Convert Jobs",
			Params:  testGithubScopeParams,
		}
		args := &SubtaskCommonArgs{Table: "github_api_jobs"}

		bootstrapped, err := bootstrapStateFromCollectorStateIfNeeded(mockDal, state, args)
		assert.Nil(t, bootstrapped)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "failed to load collector state for subtask bootstrap")
		mockDal.AssertExpectations(t)
	})

	t.Run("skip collector lookup when collector state table does not exist", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("HasTable", mock.Anything).Return(false).Once()

		state := &models.SubtaskState{
			Plugin:  "github",
			Subtask: "Convert Jobs",
			Params:  testGithubScopeParams,
		}
		args := &SubtaskCommonArgs{Table: "github_api_jobs"}

		bootstrapped, err := bootstrapStateFromCollectorStateIfNeeded(mockDal, state, args)
		assert.Nil(t, err)
		assert.NotNil(t, bootstrapped)
		assert.Nil(t, bootstrapped.PrevStartedAt)
		assert.Nil(t, bootstrapped.TimeAfter)
		mockDal.AssertNotCalled(t, "First", mock.Anything, mock.Anything)
		mockDal.AssertExpectations(t)
	})
}

func TestIsStateTableNotReadyError(t *testing.T) {
	assert.False(t, isStateTableNotReadyError(nil))
	assert.True(t, isStateTableNotReadyError(errors.Default.New("Error 1146 (42S02): Table 'lake._devlake_collector_latest_state' doesn't exist")))
	assert.True(t, isStateTableNotReadyError(errors.Default.New("pq: relation \"_devlake_collector_latest_state\" does not exist")))
	assert.True(t, isStateTableNotReadyError(errors.Default.New("no such table: _devlake_collector_latest_state")))
	assert.False(t, isStateTableNotReadyError(errors.Default.New("Error 1146 (42S02): Table 'lake._tool_github_issues' doesn't exist")))
	assert.False(t, isStateTableNotReadyError(errors.Default.New("db unavailable")))
}
