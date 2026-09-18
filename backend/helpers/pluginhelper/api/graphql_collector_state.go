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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
)

// GraphqlCollectorState tracks a collector run (empty InputHash) or an input.
// Completed input rows survive until the entire collector finishes. The run row
// survives completion until the next task, closing the crash window between
// finishing collection and the runner recording subtask success.
type GraphqlCollectorState = models.GraphqlCollectorState

func graphqlCollectorInputHash(inputJSON string) string {
	hash := sha256.Sum256([]byte(inputJSON))
	return hex.EncodeToString(hash[:])
}

func loadGraphqlCollectorState(db dal.Dal, scopeHash, inputHash string) (*GraphqlCollectorState, errors.Error) {
	state := &GraphqlCollectorState{}
	err := db.First(state, dal.Where("scope_hash = ? AND input_hash = ?", scopeHash, inputHash))
	if err != nil {
		if db.IsErrorNotFound(err) {
			return nil, nil
		}
		return nil, errors.Default.Wrap(err, "failed to load graphql collector state")
	}
	return state, nil
}

func saveGraphqlCollectorState(db dal.Dal, state *GraphqlCollectorState) errors.Error {
	return db.CreateOrUpdate(state)
}

func (collector *GraphqlCollector) checkpoint(inputHash string) *GraphqlCollectorState {
	return &GraphqlCollectorState{
		ScopeHash: collector.scopeHash, InputHash: inputHash, TaskID: collector.taskID,
		RawDataTable: collector.table, RawDataParams: collector.params,
		Contract: collector.args.checkpointContract, InputDigest: collector.args.inputDigest,
		RawCount: collector.expectedRawCount,
	}
}

// Shared by every collector writing the same raw scope, including nested ones.
func (collector *GraphqlCollector) rawCheckpoint() *GraphqlCollectorState {
	state := collector.checkpoint("__raw__")
	identity, _ := json.Marshal([]string{collector.table, collector.params})
	state.ScopeHash = graphqlCollectorInputHash(string(identity))
	return state
}

// prepareCheckpoint atomically starts a new run (including a full-sync flush)
// or restores the same persisted task. Task-less callers keep legacy behavior.
func (collector *GraphqlCollector) prepareCheckpoint() (bool, errors.Error) {
	db := collector.args.Ctx.GetDal()
	if collector.taskID == 0 {
		if !collector.args.Incremental {
			return false, db.Delete(&RawData{}, dal.From(collector.table), dal.Where("params = ?", collector.params))
		}
		return false, nil
	}
	identity, err := json.Marshal([]interface{}{collector.table, collector.params, collector.args.Ctx.GetName(), collector.args.checkpointIndex})
	if err != nil {
		return false, errors.Convert(err)
	}
	collector.scopeHash = graphqlCollectorInputHash(string(identity))
	if err := db.AutoMigrate(&GraphqlCollectorState{}); err != nil {
		return false, err
	}
	state, stateErr := loadGraphqlCollectorState(db, collector.scopeHash, "")
	if stateErr != nil {
		return false, stateErr
	}
	var rawState *GraphqlCollectorState
	if collector.args.checkpointContract != "" {
		key := collector.rawCheckpoint()
		var err errors.Error
		rawState, err = loadGraphqlCollectorState(db, key.ScopeHash, key.InputHash)
		if err != nil {
			return false, err
		}
		if rawState != nil && rawState.TaskID > collector.taskID {
			return false, errors.BadInput.New("GraphQL task was superseded by a newer task")
		}
	}
	if state != nil && state.TaskID == collector.taskID {
		if collector.args.checkpointContract != "" {
			if state.Contract != collector.args.checkpointContract {
				return false, errors.BadInput.New("GraphQL collector version/config changed; start a new task")
			}
			if !state.Completed && state.InputDigest != collector.args.inputDigest {
				return false, errors.BadInput.New("GraphQL input changed since interruption; start a new task")
			}
			count, err := db.Count(dal.From(collector.table), dal.Where("params = ?", collector.params))
			if err != nil {
				return false, err
			}
			if rawState == nil || rawState.TaskID != collector.taskID || rawState.RawCount == nil || count != *rawState.RawCount {
				return false, errors.BadInput.New("GraphQL raw data no longer matches checkpoint; start a new task")
			}
			collector.expectedRawCount = rawState.RawCount
		}
		return state.Completed, nil
	}
	tx := db.Begin()
	defer tx.Rollback()
	if !collector.args.Incremental {
		if err := tx.Delete(&RawData{}, dal.From(collector.table), dal.Where("params = ?", collector.params)); err != nil {
			return false, err
		}
	}
	if err := tx.Delete(&GraphqlCollectorState{}, dal.Where("scope_hash = ?", collector.scopeHash)); err != nil {
		return false, err
	}
	if collector.args.checkpointContract != "" {
		count, err := tx.Count(dal.From(collector.table), dal.Where("params = ?", collector.params))
		if err != nil {
			return false, err
		}
		collector.expectedRawCount = &count
	}
	if err := saveGraphqlCollectorState(tx, collector.checkpoint("")); err != nil {
		return false, err
	}
	if collector.args.checkpointContract != "" {
		if err := saveGraphqlCollectorState(tx, collector.rawCheckpoint()); err != nil {
			return false, err
		}
	}
	return false, tx.Commit()
}

func (collector *GraphqlCollector) completeCheckpoint() errors.Error {
	if collector.taskID == 0 {
		return nil
	}
	tx := collector.args.Ctx.GetDal().Begin()
	defer tx.Rollback()
	state := collector.checkpoint("")
	state.Completed = true
	if err := saveGraphqlCollectorState(tx, state); err != nil {
		return err
	}
	if err := tx.Delete(&GraphqlCollectorState{}, dal.Where("scope_hash = ? AND input_hash <> ?", collector.scopeHash, "")); err != nil {
		return err
	}
	return tx.Commit()
}
