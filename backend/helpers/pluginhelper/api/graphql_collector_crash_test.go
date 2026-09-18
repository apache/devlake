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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	deverrors "github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/unithelper"
	"github.com/apache/devlake/impls/dalgorm"
	"github.com/merico-ai/graphql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type crashInputs struct{ values []string }

func openGraphqlCrashDB(dsn string) (*gorm.DB, error) {
	if os.Getenv("DEVLAKE_CRASH_DRIVER") == "postgres" {
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	}
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}

type graphqlCrashRequest struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	Scenario     string
	Phase        string
	InputID      string
	CursorValue  string
	InputMembers string
}

func (graphqlCrashRequest) TableName() string { return "crash_requests" }

func (i *crashInputs) HasNext() bool { return len(i.values) > 0 }
func (i *crashInputs) Fetch() (interface{}, deverrors.Error) {
	value := i.values[0]
	i.values = i.values[1:]
	return value, nil
}
func (i *crashInputs) Close() deverrors.Error { return nil }

// Real rollback and retry assertions complement the mocked commit-error tests.
func TestGraphqlCollectorMySQLLifecycleRecovery(t *testing.T) {
	dsn := os.Getenv("DEVLAKE_CRASH_DSN")
	if dsn == "" {
		t.Skip("dedicated crash-test database not configured")
	}
	for _, operation := range []string{"prepare", "complete"} {
		for _, stage := range []string{"delete", "save"} {
			t.Run(operation+"/"+stage, func(t *testing.T) {
				db, err := openGraphqlCrashDB(dsn)
				require.NoError(t, err)
				sqlDB, err := db.DB()
				require.NoError(t, err)
				defer sqlDB.Close()
				ctx := unithelper.DummySubTaskContext(dalgorm.NewDalgorm(db))
				args := RawDataSubTaskArgs{Ctx: ctx, Table: "lifecycle_" + operation + "_" + stage, Params: "repo"}
				raw, lakeErr := NewRawDataSubTask(args)
				require.NoError(t, lakeErr)
				c := &GraphqlCollector{RawDataSubTask: raw, args: &GraphqlCollectorArgs{RawDataSubTaskArgs: args}, taskID: 1}
				require.NoError(t, db.Table(c.table).AutoMigrate(&RawData{}))
				_, lakeErr = c.prepareCheckpoint()
				require.NoError(t, lakeErr)
				require.NoError(t, db.Table(c.table).Create(&RawData{Params: c.params, Data: json.RawMessage(`{"ID":99}`)}).Error)
				input := c.checkpoint("input")
				input.Completed = true
				require.NoError(t, db.Create(input).Error)
				if operation == "prepare" {
					c.taskID = 2
				}
				injected := errors.New("injected lifecycle SQL failure")
				if stage == "save" {
					require.NoError(t, db.Callback().Create().After("gorm:create").Register("fail_lifecycle", func(tx *gorm.DB) {
						if _, ok := tx.Statement.Dest.(*GraphqlCollectorState); ok {
							tx.AddError(injected)
						}
					}))
				} else {
					require.NoError(t, db.Callback().Delete().After("gorm:delete").Register("fail_lifecycle", func(tx *gorm.DB) {
						if _, ok := tx.Statement.Dest.(*GraphqlCollectorState); ok {
							tx.AddError(injected)
						}
					}))
				}
				run := func() error {
					if operation == "prepare" {
						_, err := c.prepareCheckpoint()
						return err
					}
					return c.completeCheckpoint()
				}
				require.Error(t, run())
				var marker GraphqlCollectorState
				require.NoError(t, db.Where("scope_hash = ? AND input_hash = ?", c.scopeHash, "").First(&marker).Error)
				require.EqualValues(t, 1, marker.TaskID)
				require.False(t, marker.Completed)
				var count int64
				require.NoError(t, db.Table(c.table).Count(&count).Error)
				require.EqualValues(t, 1, count, "failed initialization must not lose raw data")
				require.NoError(t, db.Model(&GraphqlCollectorState{}).Where("scope_hash = ?", c.scopeHash).Count(&count).Error)
				require.EqualValues(t, 2, count, "failed completion must retain per-input progress")
				if stage == "save" {
					require.NoError(t, db.Callback().Create().Remove("fail_lifecycle"))
				} else {
					require.NoError(t, db.Callback().Delete().Remove("fail_lifecycle"))
				}
				require.NoError(t, run(), "retry must recover")
				marker = GraphqlCollectorState{}
				require.NoError(t, db.Where("scope_hash = ? AND input_hash = ?", c.scopeHash, "").First(&marker).Error)
				require.Equal(t, c.taskID, marker.TaskID)
				require.Equal(t, operation == "complete", marker.Completed)
				require.NoError(t, db.Model(&GraphqlCollectorState{}).Where("scope_hash = ?", c.scopeHash).Count(&count).Error)
				require.EqualValues(t, 1, count)
				require.NoError(t, db.Table(c.table).Count(&count).Error)
				if operation == "prepare" {
					require.Zero(t, count)
				} else {
					require.EqualValues(t, 1, count)
				}
			})
		}
	}
}

func TestGraphqlCollectorMySQLScopeIsolation(t *testing.T) {
	dsn := os.Getenv("DEVLAKE_CRASH_DSN")
	if dsn == "" {
		t.Skip("dedicated crash-test database not configured")
	}
	db, err := openGraphqlCrashDB(dsn)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	var collectors []*GraphqlCollector
	for _, dimension := range []string{"base", "table", "params", "subtask", "index"} {
		var ctx plugin.SubTaskContext = unithelper.DummySubTaskContext(dalgorm.NewDalgorm(db))
		args := GraphqlCollectorArgs{RawDataSubTaskArgs: RawDataSubTaskArgs{Ctx: ctx, Table: "scope_isolation", Params: "repo"}, Incremental: true}
		switch dimension {
		case "table":
			args.Table += "_other"
		case "params":
			args.Params = "other-repository"
		case "subtask":
			args.Ctx = namedCheckpointContext{ctx, "other-subtask"}
		case "index":
			args.checkpointIndex = 1
		}
		raw, err := NewRawDataSubTask(args.RawDataSubTaskArgs)
		require.NoError(t, err)
		c := &GraphqlCollector{RawDataSubTask: raw, args: &args, taskID: 1}
		completed, err := c.prepareCheckpoint()
		require.NoError(t, err)
		require.False(t, completed)
		input := c.checkpoint("same-input")
		input.SkipCursor = dimension
		require.NoError(t, saveGraphqlCollectorState(ctx.GetDal(), input))
		collectors = append(collectors, c)
	}
	// Completing and starting a new run for one scope must not clear any other.
	require.NoError(t, collectors[0].completeCheckpoint())
	collectors[0].taskID = 2
	_, lakeErr := collectors[0].prepareCheckpoint()
	require.NoError(t, lakeErr)
	for i, dimension := range []string{"table", "params", "subtask", "index"} {
		c := collectors[i+1]
		state, err := loadGraphqlCollectorState(c.args.Ctx.GetDal(), c.scopeHash, "same-input")
		require.NoError(t, err)
		require.NotNil(t, state)
		require.Equal(t, dimension, state.SkipCursor)
		require.EqualValues(t, 1, state.TaskID)
		completed, err := c.prepareCheckpoint()
		require.NoError(t, err)
		require.False(t, completed)
	}
}

// Opt-in destructive-process test. Use a dedicated disposable MySQL database.
// Run with DEVLAKE_CRASH_PHASE=crash, wait for CRASH_READY, SIGKILL the
// container, then rerun the same binary with DEVLAKE_CRASH_PHASE=resume.
// DEVLAKE_CRASH_SCENARIO is single, multi, transaction (kill before page
// commit), batch (non-paginated inputs), or incremental (preserve existing raw
// rows). The DB survives both runs.
// Afterwards repeat verifies the completed task makes no requests, and fresh
// uses a new task ID to verify full sync fetches all records again.
func TestGraphqlCollectorMySQLCrash(t *testing.T) {
	dsn := os.Getenv("DEVLAKE_CRASH_DSN")
	if dsn == "" {
		t.Skip("dedicated crash-test database not configured")
	}
	phase, scenario := os.Getenv("DEVLAKE_CRASH_PHASE"), os.Getenv("DEVLAKE_CRASH_SCENARIO")
	require.Contains(t, []string{"crash", "resume", "repeat", "fresh"}, phase)
	require.Contains(t, []string{"single", "multi", "transaction", "batch", "incremental", "initialize", "final-page", "complete"}, scenario)
	db, err := openGraphqlCrashDB(dsn)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	require.NoError(t, db.AutoMigrate(&graphqlCrashRequest{}))
	if phase == "crash" && (scenario == "transaction" || scenario == "initialize" || scenario == "final-page" || scenario == "complete") {
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register("crash_before_checkpoint", func(tx *gorm.DB) {
			state, ok := tx.Statement.Dest.(*GraphqlCollectorState)
			if ok && ((scenario == "transaction" && state.InputHash != "" && state.SkipCursor == "page-1") ||
				(scenario == "initialize" && state.InputHash == "" && !state.Completed) ||
				(scenario == "final-page" && state.InputHash != "" && state.InputHash != "__raw__" && state.Completed) ||
				(scenario == "complete" && state.InputHash == "" && state.Completed)) {
				fmt.Println("CRASH_READY: raw rows written but page transaction NOT committed")
				select {}
			}
		}))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				After   *string
				Input   string
				Members []string
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		cursor := ""
		if req.Variables.After != nil {
			cursor = *req.Variables.After
		}
		members, _ := json.Marshal(req.Variables.Members)
		if err := db.Exec("INSERT INTO crash_requests (scenario, phase, input_id, cursor_value, input_members) VALUES (?, ?, ?, ?, ?)", scenario, phase, req.Variables.Input, cursor, string(members)).Error; err != nil {
			t.Error(err)
			return
		}
		if phase == "crash" && req.Variables.Input == "b" && (((scenario == "single" || scenario == "multi" || scenario == "incremental") && cursor == "page-1") || scenario == "batch") {
			fmt.Println("CRASH_READY: checkpoint committed; next response withheld")
			<-r.Context().Done()
			return
		}
		id, next := 1, false
		if req.Variables.Input == "b" {
			id = 2
			next = cursor == ""
			if !next {
				id = 3
			}
		}
		if scenario == "batch" {
			next = false
		}
		nodes := []map[string]int{{"id": id}}
		if scenario == "batch" {
			nodes = nil
			for _, member := range req.Variables.Members {
				nodes = append(nodes, map[string]int{"id": map[string]int{"a": 1, "a2": 10, "b": 2, "b2": 20}[member]})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{"items": map[string]interface{}{
			"nodes": nodes, "pageInfo": GraphqlQueryPageInfo{EndCursor: "page-1", HasNextPage: next},
		}}})
	}))
	defer server.Close()
	taskID := uint64(9142)
	if phase == "fresh" {
		taskID++
	}
	ctx, cancel := context.WithCancel(plugin.WithTaskID(context.Background(), taskID))
	defer cancel()
	client := &GraphqlAsyncClient{ctx: ctx, cancel: cancel, client: graphql.NewClient(server.URL, server.Client()), logger: unithelper.DummyLogger(), rateExhaustCond: sync.NewCond(&sync.Mutex{}), rateRemaining: 1000, maxRetry: 1}
	inputs := []string{"b"}
	if scenario == "multi" {
		inputs = []string{"a", "b"}
	}
	inputStep := 1
	if scenario == "batch" {
		inputs = []string{"a", "a2", "b", "b2"}
		inputStep = 2
	}
	taskCtx := unithelper.DummySubTaskContext(dalgorm.NewDalgorm(db))
	taskCtx.On("GetContext").Return(ctx)
	collector, err := NewGraphqlCollector(GraphqlCollectorArgs{
		RawDataSubTaskArgs: RawDataSubTaskArgs{Ctx: taskCtx, Table: "crash_" + scenario, Params: "repo"},
		GraphqlClient:      client, Input: &crashInputs{values: inputs}, InputStep: inputStep, PageSize: 1,
		BuildQuery: func(req *GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			input := req.Input
			var members []string
			if scenario == "batch" {
				input = req.Input.([]interface{})[0]
				for _, member := range req.Input.([]interface{}) {
					members = append(members, member.(string))
				}
			}
			return &resumeTestQuery{}, map[string]interface{}{"after": (*graphql.String)(req.Pager.SkipCursor), "input": graphql.String(input.(string)), "members": members}, nil
		},
		GetPageInfo: func(q interface{}, _ *GraphqlCollectorArgs) (*GraphqlQueryPageInfo, error) {
			return &q.(*resumeTestQuery).Items.PageInfo, nil
		},
		ResponseParser: func(q interface{}) ([]json.RawMessage, deverrors.Error) {
			var rows []json.RawMessage
			for _, node := range q.(*resumeTestQuery).Items.Nodes {
				data, err := json.Marshal(node)
				if err != nil {
					return nil, deverrors.Convert(err)
				}
				rows = append(rows, data)
			}
			return rows, nil
		},
	})
	require.NoError(t, err)
	if scenario == "incremental" {
		collector.args.Incremental = true
		if phase == "crash" {
			require.NoError(t, db.Table(collector.table).AutoMigrate(&RawData{}))
			require.NoError(t, db.Table(collector.table).Create(&RawData{Params: collector.params, Data: json.RawMessage(`{"ID":99}`)}).Error)
		}
	}
	if scenario == "batch" {
		collector.args.GetPageInfo = nil
	}
	require.NoError(t, collector.Execute())
	require.NotEqual(t, "crash", phase, "crash phase must be killed before completing")
	var rows []RawData
	require.NoError(t, db.Table(collector.table).Order("id").Find(&rows).Error)
	var ids []int
	for _, row := range rows {
		var node struct{ ID int }
		require.NoError(t, json.Unmarshal(row.Data, &node))
		ids = append(ids, node.ID)
	}
	expected := []int{2, 3}
	if scenario == "incremental" {
		expected = []int{99, 2, 3}
		if phase == "fresh" {
			expected = append(expected, 2, 3)
		}
	}
	if scenario == "multi" {
		expected = []int{1, 2, 3}
	}
	if scenario == "batch" {
		expected = []int{1, 10, 2, 20}
	}
	require.Equal(t, expected, ids, "raw records must survive restart without duplication or loss")
	var stateCount int64
	require.NoError(t, db.Model(&GraphqlCollectorState{}).Where("raw_data_table = ? AND completed = ? AND input_hash = ?", collector.table, true, "").Count(&stateCount).Error)
	require.EqualValues(t, 1, stateCount, "completed run marker must survive until the next task")
	var requests []struct {
		InputID      string
		CursorValue  string
		InputMembers string
	}
	require.NoError(t, db.Table("crash_requests").Where("scenario = ? AND phase = ?", scenario, phase).Order("id").Find(&requests).Error)
	if phase == "repeat" {
		require.Empty(t, requests)
		return
	}
	if phase == "fresh" {
		count := 2
		if scenario == "multi" {
			count = 3
		}
		require.Len(t, requests, count, "new task must collect all pages")
		require.Empty(t, requests[0].CursorValue)
		return
	}
	if scenario == "complete" {
		require.Empty(t, requests, "all inputs already committed; only completion marker needs retry")
	} else if scenario == "transaction" || scenario == "initialize" {
		require.Len(t, requests, 2, "uncommitted page must be fetched again")
		require.Empty(t, requests[0].CursorValue)
		require.Equal(t, "page-1", requests[1].CursorValue)
	} else {
		require.Len(t, requests, 1, "only unfinished input/page should be fetched")
		require.Equal(t, "b", requests[0].InputID)
		if scenario == "batch" {
			require.Empty(t, requests[0].CursorValue)
			require.JSONEq(t, `["b","b2"]`, requests[0].InputMembers)
		} else {
			require.Equal(t, "page-1", requests[0].CursorValue)
		}
	}
}
