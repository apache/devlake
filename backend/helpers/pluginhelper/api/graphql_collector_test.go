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
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/apache/devlake/core/dal"
	deverrors "github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/merico-ai/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIsIgnorableGraphqlQueryError(t *testing.T) {
	assert.True(t, isIgnorableGraphqlQueryError(errors.New("Could not resolve to an Issue with the number of 17.")))
	assert.False(t, isIgnorableGraphqlQueryError(errors.New("some other graphql error")))
	assert.False(t, isIgnorableGraphqlQueryError(nil))
}

type resumeTestQuery struct {
	Items struct {
		Nodes    []struct{ ID int }
		PageInfo GraphqlQueryPageInfo
	} `graphql:"items(after: $after)"`
}

type checkpointTestTx struct {
	dal.Dal
	committed  bool
	rolledBack bool
	commitErr  deverrors.Error
}

func (tx *checkpointTestTx) Commit() deverrors.Error {
	tx.committed = tx.commitErr == nil
	return tx.commitErr
}
func (tx *checkpointTestTx) Rollback() deverrors.Error                 { tx.rolledBack = !tx.committed; return nil }
func (tx *checkpointTestTx) LockTables(dal.LockTables) deverrors.Error { return nil }
func (tx *checkpointTestTx) UnlockTables() deverrors.Error             { return nil }

func newResumeTestCollector(t *testing.T, db *mockdal.Dal, handler http.HandlerFunc) *GraphqlCollector {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(plugin.WithTaskID(context.Background(), 1))
	t.Cleanup(cancel)
	logger := unithelper.DummyLogger()
	logger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	client := &GraphqlAsyncClient{
		ctx: ctx, cancel: cancel, client: graphql.NewClient(server.URL, server.Client()),
		logger: logger, rateExhaustCond: sync.NewCond(&sync.Mutex{}), rateRemaining: 1000, maxRetry: 1,
	}
	field, ok := reflect.TypeOf(RawData{}).FieldByName("ID")
	require.True(t, ok)
	db.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{field})
	taskCtx := unithelper.DummySubTaskContext(db)
	taskCtx.On("GetContext").Return(ctx)
	collector, err := NewGraphqlCollector(GraphqlCollectorArgs{
		RawDataSubTaskArgs: RawDataSubTaskArgs{Ctx: taskCtx, Table: "resume_test", Params: "repo"},
		GraphqlClient:      client, PageSize: 1,
		BuildQuery: func(req *GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			return &resumeTestQuery{}, map[string]interface{}{"after": (*graphql.String)(req.Pager.SkipCursor)}, nil
		},
		GetPageInfo: func(q interface{}, _ *GraphqlCollectorArgs) (*GraphqlQueryPageInfo, error) {
			return &q.(*resumeTestQuery).Items.PageInfo, nil
		},
		ResponseParser: func(q interface{}) ([]json.RawMessage, deverrors.Error) {
			var rows []json.RawMessage
			for _, node := range q.(*resumeTestQuery).Items.Nodes {
				row, err := json.Marshal(node)
				require.NoError(t, err)
				rows = append(rows, row)
			}
			return rows, nil
		},
	})
	require.NoError(t, err)
	collector.scopeHash = "scope"
	return collector
}

func resumeTestResponse(w http.ResponseWriter, next bool) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{"items": map[string]interface{}{
		"nodes": []map[string]int{{"id": 1}}, "pageInfo": GraphqlQueryPageInfo{EndCursor: "page-1", HasNextPage: next},
	}}})
}

func expectInput(db *mockdal.Dal, input string, state *GraphqlCollectorState, readErr deverrors.Error) {
	db.On("First", mock.Anything, []dal.Clause{dal.Where("scope_hash = ? AND input_hash = ?", "scope", graphqlCollectorInputHash(input))}).
		Run(func(args mock.Arguments) {
			if state != nil {
				*args.Get(0).(*GraphqlCollectorState) = *state
			}
		}).Return(readErr).Once()
	if readErr != nil {
		db.On("IsErrorNotFound", readErr).Return(false).Once()
	}
}

func TestGraphqlPageCountAccounting(t *testing.T) {
	for _, mode := range []string{"append", "empty", "delete", "commit-error"} {
		t.Run(mode, func(t *testing.T) {
			db := mockdal.NewDal(t)
			c := newResumeTestCollector(t, db, func(w http.ResponseWriter, _ *http.Request) { resumeTestResponse(w, false) })
			count := int64(100)
			c.expectedRawCount = &count
			c.args.checkpointContract = "contract"
			expectInput(db, "null", &GraphqlCollectorState{TaskID: 1}, nil)
			tx := &checkpointTestTx{Dal: db}
			db.On("Begin").Return(tx).Once()
			expected := int64(101)
			if mode == "empty" {
				c.args.ResponseParser = func(interface{}) ([]json.RawMessage, deverrors.Error) { return nil, nil }
				expected = 100
			} else {
				db.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
			}
			if mode == "delete" {
				parser := c.args.ResponseParser
				c.args.ResponseParser = nil
				c.args.ResponseParserWithDal = func(q interface{}, _ dal.Dal) ([]json.RawMessage, deverrors.Error) { return parser(q) }
				expected = 99
				db.On("Count", mock.Anything).Return(expected, nil).Once()
			}
			db.On("CreateOrUpdate", mock.MatchedBy(func(s *GraphqlCollectorState) bool { return s.InputHash == graphqlCollectorInputHash("null") })).Return(nil).Once()
			db.On("CreateOrUpdate", mock.MatchedBy(func(s *GraphqlCollectorState) bool {
				return s.InputHash == "__raw__" && s.RawCount != nil && *s.RawCount == expected
			})).Return(nil).Once()
			if mode == "commit-error" {
				tx.commitErr = deverrors.Default.New("commit failed")
			}
			c.exec(nil)
			c.args.GraphqlClient.Wait()
			require.Equal(t, mode == "commit-error", c.HasError())
			if mode == "commit-error" {
				require.EqualValues(t, 100, *c.expectedRawCount)
			} else {
				require.Equal(t, expected, *c.expectedRawCount)
			}
			if mode != "delete" {
				db.AssertNotCalled(t, "Count", mock.Anything)
			}
		})
	}
}

func TestGraphqlCollectorResumeAndCompleteInputs(t *testing.T) {
	for _, mode := range []string{"resume", "completed", "batching", "finish-with-records", "parser-error", "http-error", "db-read-error"} {
		t.Run(mode, func(t *testing.T) {
			db := mockdal.NewDal(t)
			requests := 0
			collector := newResumeTestCollector(t, db, func(w http.ResponseWriter, r *http.Request) {
				requests++
				var request struct{ Variables map[string]interface{} }
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&request))
				if mode == "resume" {
					assert.Equal(t, "saved", request.Variables["after"])
				}
				if mode == "http-error" {
					http.Error(w, "interrupted", 503)
					return
				}
				resumeTestResponse(w, mode == "finish-with-records" || mode == "parser-error")
			})
			state := &GraphqlCollectorState{TaskID: 1, SkipCursor: "saved", Completed: mode == "completed"}
			var readErr deverrors.Error
			if mode == "db-read-error" {
				readErr = deverrors.Default.New("read failed")
			}
			expectInput(db, "null", state, readErr)
			if mode == "batching" {
				collector.args.GetPageInfo = nil
			}
			if mode == "finish-with-records" || mode == "parser-error" {
				collector.args.ResponseParser = func(interface{}) ([]json.RawMessage, deverrors.Error) {
					err := ErrFinishCollect
					if mode == "parser-error" {
						err = deverrors.Default.New("parser failed")
					}
					return []json.RawMessage{json.RawMessage(`{"id":1}`)}, err
				}
			}
			success := mode == "resume" || mode == "batching" || mode == "finish-with-records"
			tx := &checkpointTestTx{Dal: db}
			if mode == "parser-error" {
				db.On("Begin").Return(tx).Once()
			}
			if success {
				db.On("Begin").Return(tx).Once()
				raw := db.On("Create", mock.AnythingOfType("*api.RawData"), mock.Anything).Return(nil).Once()
				db.On("CreateOrUpdate", mock.MatchedBy(func(s *GraphqlCollectorState) bool {
					return s.Completed && s.TaskID == 1 && s.InputHash == graphqlCollectorInputHash("null")
				})).Return(nil).Once().NotBefore(raw)
			}
			collector.exec(nil)
			collector.args.GraphqlClient.Wait()
			assert.Equal(t, !success && mode != "completed", collector.HasError())
			if mode == "completed" || mode == "db-read-error" {
				assert.Zero(t, requests)
			} else {
				assert.Equal(t, 1, requests)
			}
			if success {
				assert.True(t, tx.committed)
			}
		})
	}
}

func TestGraphqlCollectorInputIsolation(t *testing.T) {
	db := mockdal.NewDal(t)
	var got []interface{}
	collector := newResumeTestCollector(t, db, func(w http.ResponseWriter, r *http.Request) {
		var request struct{ Variables map[string]interface{} }
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		got = append(got, request.Variables["after"])
		resumeTestResponse(w, false)
	})
	for _, input := range []string{"a", "b"} {
		encoded, _ := json.Marshal(input)
		expectInput(db, string(encoded), &GraphqlCollectorState{TaskID: 1, SkipCursor: "cursor-" + input}, nil)
		db.On("Begin").Return(&checkpointTestTx{Dal: db}).Once()
		db.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		db.On("CreateOrUpdate", mock.MatchedBy(func(s *GraphqlCollectorState) bool {
			return s.InputHash == graphqlCollectorInputHash(string(encoded)) && s.Completed
		})).Return(nil).Once()
		collector.exec(input)
		collector.args.GraphqlClient.Wait()
	}
	assert.False(t, collector.HasError())
	assert.Equal(t, []interface{}{"cursor-a", "cursor-b"}, got)
}

func TestGraphqlCollectorCheckpointFailures(t *testing.T) {
	for _, stage := range []string{"raw", "checkpoint", "commit"} {
		t.Run(stage, func(t *testing.T) {
			db := mockdal.NewDal(t)
			requests := 0
			collector := newResumeTestCollector(t, db, func(w http.ResponseWriter, r *http.Request) { requests++; resumeTestResponse(w, true) })
			expectInput(db, "null", &GraphqlCollectorState{TaskID: 1}, nil)
			dbErr := deverrors.Default.New("injected failure")
			tx := &checkpointTestTx{Dal: db}
			if stage == "commit" {
				tx.commitErr = dbErr
			}
			db.On("Begin").Return(tx).Once()
			var rawErr, checkpointErr deverrors.Error
			if stage == "raw" {
				rawErr = dbErr
			}
			if stage == "checkpoint" {
				checkpointErr = dbErr
			}
			raw := db.On("Create", mock.Anything, mock.Anything).Return(rawErr).Once()
			if stage != "raw" {
				db.On("CreateOrUpdate", mock.MatchedBy(func(s *GraphqlCollectorState) bool { return !s.Completed && s.SkipCursor == "page-1" })).Return(checkpointErr).Once().NotBefore(raw)
			}
			collector.exec(nil)
			collector.args.GraphqlClient.Wait()
			require.True(t, collector.HasError())
			assert.True(t, tx.rolledBack)
			assert.False(t, tx.committed)
			assert.Equal(t, 1, requests)
		})
	}
}

func TestGraphqlCollectorRunIdentity(t *testing.T) {
	for _, mode := range []string{"resume", "completed", "new-task", "taskless", "read-error"} {
		t.Run(mode, func(t *testing.T) {
			db := mockdal.NewDal(t)
			collector := newResumeTestCollector(t, db, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected HTTP") })
			tx := &checkpointTestTx{Dal: db}
			if mode == "taskless" {
				collector.taskID = 0
				db.On("Delete", mock.AnythingOfType("*api.RawData"), mock.Anything).Return(nil).Once()
			} else {
				db.On("AutoMigrate", mock.Anything).Return(nil).Once()
				var readErr deverrors.Error
				if mode == "read-error" {
					readErr = deverrors.Default.New("read failed")
					db.On("IsErrorNotFound", readErr).Return(false).Once()
				}
				db.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					s := args.Get(0).(*GraphqlCollectorState)
					s.TaskID = 1
					if mode == "new-task" {
						s.TaskID = 99
					}
					s.Completed = mode == "completed"
				}).Return(readErr).Once()
				if mode == "new-task" {
					db.On("Begin").Return(tx).Once()
					db.On("Delete", mock.AnythingOfType("*api.RawData"), mock.Anything).Return(nil).Once()
					db.On("Delete", mock.AnythingOfType("*models.GraphqlCollectorState"), mock.Anything).Return(nil).Once()
					db.On("CreateOrUpdate", mock.MatchedBy(func(s *GraphqlCollectorState) bool { return s.TaskID == 1 && s.InputHash == "" && !s.Completed })).Return(nil).Once()
				}
			}
			completed, err := collector.prepareCheckpoint()
			assert.Equal(t, mode == "read-error", err != nil)
			assert.Equal(t, mode == "completed", completed)
			if mode == "new-task" {
				assert.True(t, tx.committed)
			}
		})
	}
}

func TestGraphqlCollectorCompletionTransaction(t *testing.T) {
	db := mockdal.NewDal(t)
	collector := newResumeTestCollector(t, db, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected HTTP") })
	tx := &checkpointTestTx{Dal: db}
	db.On("Begin").Return(tx).Once()
	marker := db.On("CreateOrUpdate", mock.MatchedBy(func(s *GraphqlCollectorState) bool { return s.Completed && s.InputHash == "" && s.TaskID == 1 })).Return(nil).Once()
	db.On("Delete", mock.Anything, []dal.Clause{dal.Where("scope_hash = ? AND input_hash <> ?", "scope", "")}).Return(nil).Once().NotBefore(marker)
	require.NoError(t, collector.completeCheckpoint())
	assert.True(t, tx.committed)
}

func TestGraphqlCancelledNextPage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &GraphqlAsyncClient{ctx: ctx}
	reported := make(chan error, 1)
	client.NextTick(func() deverrors.Error { t.Error("cancelled page executed"); return nil }, func(err error) { reported <- err })
	done := make(chan struct{})
	go func() { client.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled request leaked its wait group")
	}
	require.ErrorIs(t, <-reported, context.Canceled)
}

// Override only the subtask name while retaining the usual collector harness.
type namedCheckpointContext struct {
	plugin.SubTaskContext
	name string
}

func (c namedCheckpointContext) GetName() string { return c.name }

func TestGraphqlCollectorScopeIsolation(t *testing.T) {
	scopes := map[string]bool{}
	for _, dimension := range []string{"base", "table", "params", "subtask", "index"} {
		t.Run(dimension, func(t *testing.T) {
			db := mockdal.NewDal(t)
			c := newResumeTestCollector(t, db, func(http.ResponseWriter, *http.Request) { t.Error("unexpected HTTP") })
			switch dimension {
			case "table":
				c.table += "_other"
			case "params":
				c.params = "other-repository"
			case "subtask":
				c.args.Ctx = namedCheckpointContext{c.args.Ctx, "other-subtask"}
			case "index":
				c.args.checkpointIndex++
			}
			db.On("AutoMigrate", mock.Anything).Return(nil).Once()
			db.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				*args.Get(0).(*GraphqlCollectorState) = GraphqlCollectorState{TaskID: 1, Completed: true}
			}).Return(nil).Once()
			completed, err := c.prepareCheckpoint()
			require.NoError(t, err)
			require.True(t, completed)
			assert.False(t, scopes[c.scopeHash], "independent collectors must not share checkpoints")
			scopes[c.scopeHash] = true
		})
	}
}

func TestGraphqlCollectorIncrementalInitialization(t *testing.T) {
	for _, taskID := range []uint64{0, 1} {
		db := mockdal.NewDal(t)
		c := newResumeTestCollector(t, db, func(http.ResponseWriter, *http.Request) { t.Error("unexpected HTTP") })
		c.taskID, c.args.Incremental = taskID, true
		if taskID != 0 {
			db.On("AutoMigrate", mock.Anything).Return(nil).Once()
			db.On("First", mock.Anything, mock.Anything).Return(nil).Once()
			db.On("Begin").Return(&checkpointTestTx{Dal: db}).Once()
			db.On("Delete", mock.AnythingOfType("*models.GraphqlCollectorState"), mock.Anything).Return(nil).Once()
			db.On("CreateOrUpdate", mock.Anything).Return(nil).Once()
		}
		completed, err := c.prepareCheckpoint()
		require.NoError(t, err)
		require.False(t, completed)
		// No raw-data Delete expectation: attempting to flush fails this test.
	}
}

func TestGraphqlCollectorLifecycleFailures(t *testing.T) {
	for _, operation := range []string{"prepare", "complete"} {
		for _, stage := range []string{"raw-delete", "state-delete", "state-save", "commit"} {
			if operation == "complete" && stage == "raw-delete" {
				continue
			}
			t.Run(operation+"/"+stage, func(t *testing.T) {
				db := mockdal.NewDal(t)
				c := newResumeTestCollector(t, db, func(http.ResponseWriter, *http.Request) { t.Error("unexpected HTTP") })
				failure := deverrors.Default.New("injected lifecycle failure")
				tx := &checkpointTestTx{Dal: db}
				if stage == "commit" {
					tx.commitErr = failure
				}
				db.On("Begin").Return(tx).Once()
				var rawErr, deleteErr, saveErr deverrors.Error
				if stage == "raw-delete" {
					rawErr = failure
				}
				if stage == "state-delete" {
					deleteErr = failure
				}
				if stage == "state-save" {
					saveErr = failure
				}
				if operation == "prepare" {
					db.On("AutoMigrate", mock.Anything).Return(nil).Once()
					db.On("First", mock.Anything, mock.Anything).Return(nil).Once()
					db.On("Delete", mock.AnythingOfType("*api.RawData"), mock.Anything).Return(rawErr).Once()
					if rawErr == nil {
						db.On("Delete", mock.AnythingOfType("*models.GraphqlCollectorState"), mock.Anything).Return(deleteErr).Once()
					}
					if rawErr == nil && deleteErr == nil {
						db.On("CreateOrUpdate", mock.Anything).Return(saveErr).Once()
					}
					_, err := c.prepareCheckpoint()
					require.Error(t, err)
				} else {
					db.On("CreateOrUpdate", mock.Anything).Return(saveErr).Once()
					if saveErr == nil {
						db.On("Delete", mock.Anything, mock.Anything).Return(deleteErr).Once()
					}
					require.Error(t, c.completeCheckpoint())
				}
				assert.False(t, tx.committed)
				assert.True(t, tx.rolledBack)
			})
		}
	}
}
