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
	gocontext "context"
	"time"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
)

// CollectorStateManager manages the state of the collector. It is used to determine whether
// the collector should run in incremental mode or full sync mode and what time range to collect.
type CollectorStateManager struct {
	db         dal.Dal
	state      *models.CollectorLatestState
	syncPolicy *models.SyncPolicy
	// IsIncremental indicates whether the collector should run in incremental mode or full sync mode
	isIncremental bool
	// Since is the start time of the time range to collect
	since *time.Time
	// Until is the end time of the time range to collect
	until *time.Time
}

// NewCollectorStateManager create a new CollectorStateManager
func NewCollectorStateManager(basicRes context.BasicRes, syncPolicy *models.SyncPolicy, rawTable, rawParams string) (stateManager *CollectorStateManager, err errors.Error) {
	// load sync policy and make sure it is not nil
	if syncPolicy == nil {
		syncPolicy = &models.SyncPolicy{}
	}

	// load the previous state from the database
	db := basicRes.GetDal()
	state := &models.CollectorLatestState{}
	err = db.First(state, dal.Where(`raw_data_table = ? AND raw_data_params = ?`, rawTable, rawParams))
	if err != nil {
		if db.IsErrorNotFound(err) {
			state = &models.CollectorLatestState{
				RawDataTable:  rawTable,
				RawDataParams: rawParams,
			}
			err = nil
		} else {
			err = errors.Default.Wrap(err, "failed to load the previous collector state")
			return
		}
	}

	// fullsync by default
	now := time.Now()
	if exec, ok := basicRes.(interface{ GetContext() gocontext.Context }); ok {
		if started, found := plugin.TaskStartedAt(exec.GetContext()); found {
			now = started
		}
	}
	stateManager = &CollectorStateManager{
		db:            db,
		state:         state,
		syncPolicy:    syncPolicy,
		isIncremental: false,
		since:         syncPolicy.TimeAfter,
		until:         &now,
	}
	// fallback to the previous timeAfter if no new value
	if stateManager.since == nil {
		stateManager.since = state.TimeAfter
	}

	// if fullsync is set or no previous success start time, we are in the full sync mode
	if syncPolicy.FullSync || state.LatestSuccessStart == nil {
		return
	}

	// if timeAfter is not set or NOT before the previous value, we are in the incremental mode
	if syncPolicy.TimeAfter == nil || state.TimeAfter == nil || !syncPolicy.TimeAfter.Before(*state.TimeAfter) {
		stateManager.isIncremental = true
		stateManager.since = state.LatestSuccessStart
	}

	return
}

func (c *CollectorStateManager) IsIncremental() bool {
	return c.isIncremental
}

func (c *CollectorStateManager) GetSince() *time.Time {
	return c.since
}

func (c *CollectorStateManager) GetUntil() *time.Time {
	return c.until
}

func (c *CollectorStateManager) Close() errors.Error {
	// update timeAfter in the database only for fullsync mode
	if !c.isIncremental {
		// prefer non-nil value
		if c.syncPolicy.TimeAfter != nil {
			c.state.TimeAfter = c.syncPolicy.TimeAfter
		}
	}
	// always update the latest success start time
	c.state.LatestSuccessStart = c.until
	return c.db.Update(c.state)
}
