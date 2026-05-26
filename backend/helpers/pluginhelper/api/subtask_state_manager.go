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
	"reflect"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/utils"
)

// SubtaskCommonArgs is a struct that contains the common arguments for a subtask
type SubtaskCommonArgs struct {
	plugin.SubTaskContext
	Table         string // raw table name
	Params        any    // for filtering rows belonging to the scope (jira board, github repo) of the subtask
	SubtaskConfig any    // for determining whether the subtask should run in Incremental or Full-Sync mode by comparing with the previous config to see if it changed
	BatchSize     int    // batch size for saving data
}

func (args *SubtaskCommonArgs) GetRawDataTable() string {
	return fmt.Sprintf("_raw_%s", args.Table)
}

func (args *SubtaskCommonArgs) GetRawDataParams() string {
	if args.Params == nil || reflect.ValueOf(args.Params).IsZero() {
		panic(errors.Default.New("Params is nil"))
	}
	return utils.ToJsonString(args.Params)
}

func (args *SubtaskCommonArgs) GetSubtaskConfig() string {
	return utils.ToJsonString(args.SubtaskConfig)
}

func (args *SubtaskCommonArgs) GetBatchSize() int {
	if args.BatchSize == 0 {
		args.BatchSize = 500
	}
	return args.BatchSize
}

// SubtaskStateManager manages the state of a subtask. It is used to determine whether
// the subtask should run in incremental mode or full sync mode and what time range to collect.
type SubtaskStateManager struct {
	db            dal.Dal
	state         *models.SubtaskState
	syncPolicy    *models.SyncPolicy
	isIncremental bool       // tells if the subtask should run in incremental mode or full sync mode
	since         *time.Time // the start time of the time range to work on
	until         *time.Time // the end time of the time range to work on
	config        string     // current configuration of the subtask for determining if the subtask should run in incremental or full sync mode
}

// NewSubtaskStateManager create a new SubtaskStateManager
func NewSubtaskStateManager(args *SubtaskCommonArgs) (stateManager *SubtaskStateManager, err errors.Error) {
	db := args.GetDal()
	// load sync policy and make sure it is not nil
	syncPolicy := args.SubTaskContext.TaskContext().SyncPolicy()
	if syncPolicy == nil {
		syncPolicy = &models.SyncPolicy{}
	}

	plugin := args.SubTaskContext.TaskContext().GetName()
	subtask := args.SubTaskContext.GetName()
	params := args.GetRawDataParams()
	preState, err := loadPreviousState(db, plugin, subtask, params)
	if err != nil {
		return
	}
	preState, err = bootstrapStateFromCollectorStateIfNeeded(db, preState, args)
	if err != nil {
		return
	}

	isIncremental, since := calculateStateManagerIncrementalMode(syncPolicy, preState, utils.ToJsonString(args.SubtaskConfig))

	now := time.Now()
	stateManager = &SubtaskStateManager{
		db:            db,
		state:         preState,
		syncPolicy:    syncPolicy,
		isIncremental: isIncremental,
		since:         since,
		until:         &now,
		config:        utils.ToJsonString(args.SubtaskConfig),
	}
	// fallback to the previous timeAfter if no new value
	if stateManager.since == nil {
		stateManager.since = preState.TimeAfter
	}
	return
}

func loadPreviousState(db dal.Dal, plugin, subtask, params string) (*models.SubtaskState, errors.Error) {
	// load the previous state from the database
	preState := &models.SubtaskState{}
	err := db.First(preState, dal.Where(`plugin = ? AND subtask =? AND params = ?`, plugin, subtask, params))
	if err != nil {
		if db.IsErrorNotFound(err) {
			preState = &models.SubtaskState{
				Plugin:  plugin,
				Subtask: subtask,
				Params:  params,
			}
		} else {
			return nil, errors.Default.Wrap(err, "failed to load the previous subtask state")
		}
	}

	return preState, nil
}

func bootstrapStateFromCollectorStateIfNeeded(db dal.Dal, preState *models.SubtaskState, args *SubtaskCommonArgs) (*models.SubtaskState, errors.Error) {
	if preState == nil || preState.PrevStartedAt != nil {
		return preState, nil
	}
	if args == nil || args.Table == "" {
		return preState, nil
	}
	if !db.HasTable(&models.CollectorLatestState{}) {
		return preState, nil
	}

	rawTable := args.GetRawDataTable()
	if rawTable == "" {
		return preState, nil
	}

	collectorState := &models.CollectorLatestState{}
	err := db.First(
		collectorState,
		dal.Where("raw_data_table = ? AND raw_data_params = ?", rawTable, preState.Params),
	)
	if err != nil {
		if db.IsErrorNotFound(err) || isStateTableNotReadyError(err) {
			return preState, nil
		}
		return nil, errors.Default.Wrap(err, "failed to load collector state for subtask bootstrap")
	}

	if collectorState.LatestSuccessStart != nil {
		preState.PrevStartedAt = collectorState.LatestSuccessStart
	}
	if preState.TimeAfter == nil && collectorState.TimeAfter != nil {
		preState.TimeAfter = collectorState.TimeAfter
	}

	return preState, nil
}

func isStateTableNotReadyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "_devlake_collector_latest_state") &&
		(strings.Contains(msg, "doesn't exist") ||
			strings.Contains(msg, "does not exist") ||
			strings.Contains(msg, "unknown table") ||
			strings.Contains(msg, "no such table"))
}

// calculateStateManagerIncrementalMode tries to calculate whether state manager should run in incremental mode and returns the state manager's 'since' time.
func calculateStateManagerIncrementalMode(syncPolicy *models.SyncPolicy, preState *models.SubtaskState, newSubtaskConfig string) (bool, *time.Time) {
	if preState == nil || syncPolicy == nil {
		panic("preState or syncPolicy is nil")
	}

	// User click 'Collect Data in Full Refresh Mode'
	// No matter whether there is a successful pipeline.
	if syncPolicy.FullSync {
		return false, syncPolicy.TimeAfter
	}
	// No previous success state means this pipeline has never been executed.
	if preState.PrevStartedAt == nil {
		return false, syncPolicy.TimeAfter
	}
	// When subtask config has changed, state manager should NOT in incremental mode.
	if subTaskConfigHasChanged(preState, newSubtaskConfig) {
		return false, syncPolicy.TimeAfter
	}
	// There is a sync policy and sync policy is earlier than latest successful pipeline's timeAfter
	if syncPolicy.TimeAfter != nil && preState.TimeAfter != nil && syncPolicy.TimeAfter.Before(*preState.TimeAfter) {
		return false, syncPolicy.TimeAfter
	}

	// No need to do a full refresh, run task incrementally.
	// New state manager's start time is previous state's finished time.
	// But there is no such field, so use previous state's PrevStartedAt time.
	return true, preState.PrevStartedAt
}

// subTaskConfigHasChanged checks whether the previous sub-task config is the same as the current sub-task config
// When plugin's scope config changes, Subtask's config may change.
func subTaskConfigHasChanged(preState *models.SubtaskState, newSubtaskConfig string) bool {
	if preState == nil {
		return true
	}
	preConfig := preState.PrevConfig
	return preConfig != "" && preConfig != newSubtaskConfig
}

func (c *SubtaskStateManager) IsIncremental() bool {
	return c.isIncremental
}

func (c *SubtaskStateManager) GetSince() *time.Time {
	return c.since
}

func (c *SubtaskStateManager) GetUntil() *time.Time {
	return c.until
}

func (c *SubtaskStateManager) Close() errors.Error {
	// update timeAfter in the database only for fullsync mode
	if !c.isIncremental {
		// prefer non-nil value
		if c.syncPolicy.TimeAfter != nil {
			c.state.TimeAfter = c.syncPolicy.TimeAfter
		}
	}
	// always update the latest success start time
	c.state.PrevStartedAt = c.until
	c.state.PrevConfig = c.config
	return c.db.CreateOrUpdate(c.state)
}
