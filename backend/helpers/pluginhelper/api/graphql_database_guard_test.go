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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/unithelper"
	"github.com/apache/devlake/impls/dalgorm"
	"github.com/merico-ai/graphql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func checkpointRealDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DEVLAKE_CRASH_DSN")
	if dsn == "" {
		t.Skip("dedicated database not configured")
	}
	db, err := openGraphqlCrashDB(dsn)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.CollectorLatestState{}))
	return db
}

func realCheckpointArgs(t *testing.T, db *gorm.DB, table string, task uint64) GraphqlCollectorArgs {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"items":{"nodes":[{"id":1},{"id":2}],"pageInfo":{"endCursor":"","hasNextPage":false}}}}`))
	}))
	t.Cleanup(upstream.Close)
	ctx, cancel := context.WithCancel(plugin.WithTaskID(context.Background(), task))
	t.Cleanup(cancel)
	taskCtx := unithelper.DummySubTaskContext(dalgorm.NewDalgorm(db))
	taskCtx.On("GetContext").Return(ctx)
	client := &GraphqlAsyncClient{ctx: ctx, cancel: cancel, client: graphql.NewClient(upstream.URL, upstream.Client()), logger: unithelper.DummyLogger(), rateExhaustCond: sync.NewCond(&sync.Mutex{}), rateRemaining: 1000, maxRetry: 1}
	return GraphqlCollectorArgs{
		RawDataSubTaskArgs: RawDataSubTaskArgs{Ctx: taskCtx, Table: table, Params: "repo"},
		GraphqlClient:      client, PageSize: 2,
		BuildQuery: func(req *GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			return &resumeTestQuery{}, map[string]interface{}{"after": (*graphql.String)(req.Pager.SkipCursor)}, nil
		},
		ResponseParser: func(q interface{}) ([]json.RawMessage, errors.Error) {
			var rows []json.RawMessage
			for _, n := range q.(*resumeTestQuery).Items.Nodes {
				b, err := json.Marshal(n)
				if err != nil {
					return nil, errors.Convert(err)
				}
				rows = append(rows, b)
			}
			return rows, nil
		},
	}
}

func rawCheckpointIDs(t *testing.T, db *gorm.DB, table string) []int {
	t.Helper()
	var rows []RawData
	require.NoError(t, db.Table(table).Order("id").Find(&rows).Error)
	ids := []int{}
	for _, row := range rows {
		var n struct{ ID int }
		require.NoError(t, json.Unmarshal(row.Data, &n))
		ids = append(ids, n.ID)
	}
	return ids
}

func TestGraphqlDatabaseGuard(t *testing.T) {
	for _, change := range []string{"input", "order", "config", "binary", "plugin", "raw", "superseded"} {
		t.Run(change, func(t *testing.T) {
			db := checkpointRealDB(t)
			args := realCheckpointArgs(t, db, "guard_"+change, 100)
			args.Input = &crashInputs{values: []string{"a", "b"}}
			// Initialization commits; the first query fails, retaining the manifest.
			build := args.BuildQuery
			args.BuildQuery = func(*GraphqlRequestData) (interface{}, map[string]interface{}, error) {
				return nil, nil, fmt.Errorf("interrupted")
			}
			c, err := NewGraphqlCollector(args)
			require.NoError(t, err)
			require.Error(t, c.Execute())
			args.BuildQuery = build
			args.Input = &crashInputs{values: []string{"a", "b"}}
			switch change {
			case "input":
				args.Input = &crashInputs{values: []string{"a", "c"}}
			case "order":
				args.Input = &crashInputs{values: []string{"b", "a"}}
			case "config":
				args.PageSize++
			case "plugin":
				plugin.RegisterPluginCode("changed-plugin", "updated-shared-object")
				args.Ctx = graphqlContextOverride{args.Ctx, plugin.WithTaskCode(args.Ctx.GetContext(), "changed-plugin")}
			case "binary":
				require.NoError(t, db.Model(&GraphqlCollectorState{}).Where("scope_hash = ? AND input_hash = ?", c.scopeHash, "").Update("contract", "old-binary").Error)
			case "raw":
				require.NoError(t, db.Table(c.table).Create(&RawData{Params: c.params, Data: json.RawMessage(`{"ID":99}`)}).Error)
			case "superseded":
				newer := realCheckpointArgs(t, db, "guard_"+change, 101)
				n, err := NewGraphqlCollector(newer)
				require.NoError(t, err)
				require.NoError(t, n.Execute())
			}
			retry, err := NewGraphqlCollector(args)
			require.NoError(t, err)
			require.Error(t, retry.Execute(), "unsafe reuse must not silently mark data complete")
		})
	}
}

func TestGraphqlDatabasePageRollback(t *testing.T) {
	for _, failure := range []string{"second-row", "checkpoint", "parser", "commit-ack"} {
		t.Run(failure, func(t *testing.T) {
			db := checkpointRealDB(t)
			args := realCheckpointArgs(t, db, "rollback_"+failure, 200)
			args.Table = "rollback_" + map[string]string{"second-row": "row", "checkpoint": "state", "parser": "parser", "commit-ack": "ack"}[failure]
			originalParser := args.ResponseParser
			args.ResponseParser = nil
			require.NoError(t, db.Exec("CREATE TABLE IF NOT EXISTS graphql_parser_effects (name VARCHAR(64) PRIMARY KEY)").Error)
			require.NoError(t, db.Exec("INSERT INTO graphql_parser_effects (name) VALUES (?)", failure).Error)
			injecting := true
			args.ResponseParserWithDal = func(q interface{}, tx dal.Dal) ([]json.RawMessage, errors.Error) {
				if err := tx.Delete(nil, dal.From("graphql_parser_effects"), dal.Where("name = ?", failure)); err != nil {
					return nil, err
				}
				if injecting && failure == "parser" {
					return nil, errors.Default.New("parser interrupted")
				}
				return originalParser(q)
			}
			creates := 0
			require.NoError(t, db.Callback().Create().After("gorm:create").Register("fail_page", func(tx *gorm.DB) {
				if !injecting {
					return
				}
				if _, ok := tx.Statement.Dest.(*RawData); ok {
					creates++
					if failure == "second-row" && creates == 2 {
						tx.AddError(fmt.Errorf("second row failed"))
					}
				}
				if s, ok := tx.Statement.Dest.(*GraphqlCollectorState); ok && s.InputHash != "" && s.InputHash != "__raw__" && failure == "checkpoint" {
					tx.AddError(fmt.Errorf("checkpoint failed"))
				}
			}))
			if failure == "commit-ack" {
				// Commit succeeds but its acknowledgement is lost. Resume must trust the DB,
				// not retry a page based on the caller's error alone.
				args.Ctx = graphqlDalOverride{args.Ctx, &uncertainCommitDal{Dal: args.Ctx.GetDal()}}
			}
			c, err := NewGraphqlCollector(args)
			require.NoError(t, err)
			require.Error(t, c.Execute())
			if failure != "commit-ack" {
				require.Empty(t, rawCheckpointIDs(t, db, c.table))
				var n int64
				require.NoError(t, db.Table("graphql_parser_effects").Where("name = ?", failure).Count(&n).Error)
				require.EqualValues(t, 1, n)
			}
			injecting = false
			// Use a clean DAL and new collector as after a process restart.
			args.Ctx = realCheckpointArgs(t, db, args.Table, 200).Ctx
			retry, err := NewGraphqlCollector(args)
			require.NoError(t, err)
			require.NoError(t, retry.Execute())
			require.Equal(t, []int{1, 2}, rawCheckpointIDs(t, db, c.table))
			var n int64
			require.NoError(t, db.Table("graphql_parser_effects").Where("name = ?", failure).Count(&n).Error)
			require.Zero(t, n)
		})
	}
}

type graphqlDalOverride struct {
	plugin.SubTaskContext
	db dal.Dal
}

func TestGraphqlDatabaseTasklessAndEmpty(t *testing.T) {
	for _, mode := range []string{"taskless-full", "taskless-incremental", "empty", "finish-empty"} {
		t.Run(mode, func(t *testing.T) {
			db := checkpointRealDB(t)
			args := realCheckpointArgs(t, db, "compat_"+map[string]string{"taskless-full": "full", "taskless-incremental": "inc", "empty": "empty", "finish-empty": "finish"}[mode], 0)
			if mode == "taskless-incremental" {
				args.Incremental = true
			}
			if mode == "empty" {
				args.Input = &crashInputs{}
			}
			if mode == "finish-empty" {
				args.ResponseParser = func(interface{}) ([]json.RawMessage, errors.Error) { return nil, ErrFinishCollect }
			}
			for i := 0; i < 2; i++ {
				c, err := NewGraphqlCollector(args)
				require.NoError(t, err)
				require.NoError(t, c.Execute())
			}
			raw, err := NewRawDataSubTask(args.RawDataSubTaskArgs)
			require.NoError(t, err)
			expected := []int{1, 2}
			if mode == "taskless-incremental" {
				expected = []int{1, 2, 1, 2}
			}
			if mode == "empty" || mode == "finish-empty" {
				expected = []int{}
			}
			require.Equal(t, expected, rawCheckpointIDs(t, db, raw.table))
			var n int64
			require.NoError(t, db.Model(&GraphqlCollectorState{}).Where("raw_data_table = ?", raw.table).Count(&n).Error)
			require.Zero(t, n, "legacy task-less calls must not leave checkpoint state")
		})
	}
}
func (c graphqlDalOverride) GetDal() dal.Dal { return c.db }

type uncertainCommitDal struct {
	dal.Dal
	commits int
}

func (d *uncertainCommitDal) Begin() dal.Transaction {
	return &uncertainCommitTx{Transaction: d.Dal.Begin(), owner: d}
}

type uncertainCommitTx struct {
	dal.Transaction
	owner *uncertainCommitDal
}

func (tx *uncertainCommitTx) Commit() errors.Error {
	err := tx.Transaction.Commit()
	if err != nil {
		return err
	}
	tx.owner.commits++
	if tx.owner.commits == 2 {
		return errors.Default.New("commit acknowledgement lost")
	}
	return nil
}

func TestGraphqlDatabaseNestedResume(t *testing.T) {
	t.Run("state-manager-save", func(t *testing.T) {
		db := checkpointRealDB(t)
		args := realCheckpointArgs(t, db, "manager_save", 301)
		injected := func(tx *gorm.DB) {
			if _, ok := tx.Statement.Dest.(*models.CollectorLatestState); ok {
				tx.AddError(fmt.Errorf("watermark write failed"))
			}
		}
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register("fail_watermark", injected))
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register("fail_watermark", injected))
		m, err := NewStatefulApiCollector(args.RawDataSubTaskArgs)
		require.NoError(t, err)
		require.NoError(t, m.InitGraphQLCollector(args))
		require.Error(t, m.Execute())
		c := m.nestedCollectors[0].(*GraphqlCollector)
		require.Equal(t, []int{1, 2}, rawCheckpointIDs(t, db, c.table))
		require.NoError(t, db.Callback().Create().Remove("fail_watermark"))
		require.NoError(t, db.Callback().Update().Remove("fail_watermark"))
		args.BuildQuery = func(*GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			return nil, nil, fmt.Errorf("completed collector must not query again")
		}
		m, err = NewStatefulApiCollector(args.RawDataSubTaskArgs)
		require.NoError(t, err)
		require.NoError(t, m.InitGraphQLCollector(args))
		require.NoError(t, m.Execute())
		require.Equal(t, []int{1, 2}, rawCheckpointIDs(t, db, c.table))
	})
	db := checkpointRealDB(t)
	args := realCheckpointArgs(t, db, "nested_resume", 300)
	fail := true
	makeManager := func() *StatefulApiCollector {
		m, err := NewStatefulApiCollector(args.RawDataSubTaskArgs)
		require.NoError(t, err)
		require.NoError(t, m.InitGraphQLCollector(args))
		second := args
		second.ResponseParser = func(interface{}) ([]json.RawMessage, errors.Error) {
			if fail {
				return nil, errors.Default.New("second collector interrupted")
			}
			return []json.RawMessage{json.RawMessage(`{"ID":3}`)}, nil
		}
		require.NoError(t, m.InitGraphQLCollector(second))
		return m
	}
	first := makeManager()
	require.Error(t, first.Execute())
	table := first.nestedCollectors[0].(*GraphqlCollector).table
	require.Equal(t, []int{1, 2}, rawCheckpointIDs(t, db, table))
	fail = false
	require.NoError(t, makeManager().Execute())
	require.Equal(t, []int{1, 2, 3}, rawCheckpointIDs(t, db, table))
	require.NoError(t, makeManager().Execute())
	require.Equal(t, []int{1, 2, 3}, rawCheckpointIDs(t, db, table))
}
