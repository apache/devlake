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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/apache/devlake/core/dal"
	lakeerrors "github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type failingGraphqlInput struct {
	values   []interface{}
	position int
	pending  bool
	readErr  error
	fetchErr lakeerrors.Error
	closeErr lakeerrors.Error
	closes   int
}

func (s *failingGraphqlInput) HasNext() bool {
	if s.position >= len(s.values) {
		return false
	}
	s.position++
	s.pending = true
	return true
}
func (s *failingGraphqlInput) Fetch() (interface{}, lakeerrors.Error) {
	if !s.pending {
		return nil, lakeerrors.Default.New("Fetch without Next")
	}
	s.pending = false
	return s.values[s.position-1], s.fetchErr
}
func (s *failingGraphqlInput) Err() error              { return s.readErr }
func (s *failingGraphqlInput) Close() lakeerrors.Error { s.closes++; return s.closeErr }

func TestGraphqlInvalidCollectionConfiguration(t *testing.T) {
	for _, field := range []string{"build", "input", "page", "batch", "parser", "both-parsers", "retry"} {
		t.Run(field, func(t *testing.T) {
			db := mockdal.NewDal(t)
			c := newResumeTestCollector(t, db, func(http.ResponseWriter, *http.Request) { t.Error("invalid configuration must not send HTTP") })
			args := *c.args
			switch field {
			case "build":
				args.BuildQuery = nil
			case "input":
				args.InputStep = -1
			case "page":
				args.PageSize = -1
			case "batch":
				args.BatchSize = -1
			case "parser":
				args.ResponseParser = nil
			case "both-parsers":
				args.ResponseParserWithDal = func(interface{}, dal.Dal) ([]json.RawMessage, lakeerrors.Error) { return nil, nil }
			case "retry":
				c.args.GraphqlClient.maxRetry = 0
				_, err := c.args.GraphqlClient.Query(&resumeTestQuery{}, nil)
				require.Error(t, err)
				return
			}
			_, err := NewGraphqlCollector(args)
			require.Error(t, err)
		})
	}
}

func TestGraphqlStageInput(t *testing.T) {
	type record struct{ ID int }
	values := []interface{}{&record{1}, &record{2}, "three", nil, map[string]int{"a": 4}}
	for _, failure := range []string{"none", "read", "fetch", "close", "marshal", "cancel", "duplicate"} {
		t.Run(failure, func(t *testing.T) {
			input := &failingGraphqlInput{values: values}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch failure {
			case "read":
				input.readErr = errors.New("DB cursor disconnected")
			case "fetch":
				input.fetchErr = lakeerrors.Default.New("scan failed")
			case "close":
				input.closeErr = lakeerrors.Default.New("close failed")
			case "marshal":
				input.values = []interface{}{make(chan int)}
			case "cancel":
				cancel()
			case "duplicate":
				input.values = []interface{}{"same", "same"}
			}
			staged, digest, err := stageGraphqlInput(ctx, input)
			require.Equal(t, 1, input.closes)
			if failure != "none" {
				require.Error(t, err)
				require.Nil(t, staged)
				return
			}
			require.NoError(t, err)
			require.Len(t, digest, 64)
			defer staged.Close()
			var got []interface{}
			for staged.HasNext() {
				value, err := staged.Fetch()
				require.NoError(t, err)
				got = append(got, value)
			}
			require.NoError(t, staged.Err())
			require.Equal(t, values, got)
			require.NoError(t, staged.Close())
			require.NoError(t, staged.Close())
		})
	}
}

func TestGraphqlInputFailureCannotComplete(t *testing.T) {
	for _, failure := range []string{"read", "fetch", "close"} {
		t.Run(failure, func(t *testing.T) {
			db := mockdal.NewDal(t)
			c := newResumeTestCollector(t, db, func(http.ResponseWriter, *http.Request) { t.Error("must not fetch incomplete input list") })
			input := &failingGraphqlInput{values: []interface{}{"a"}}
			switch failure {
			case "read":
				input.readErr = errors.New("read failed")
			case "fetch":
				input.fetchErr = lakeerrors.Default.New("fetch failed")
			case "close":
				input.closeErr = lakeerrors.Default.New("close failed")
			}
			c.args.Input = input
			require.Error(t, c.Execute())
			require.Equal(t, 1, input.closes)
			// No migration/deletion/checkpoint expectations: all must remain untouched.
		})
	}
}

func TestGraphqlScopeLock(t *testing.T) {
	release, err := acquireGraphqlScope("table", "repo")
	require.NoError(t, err)
	_, err = acquireGraphqlScope("table", "repo")
	require.Error(t, err)
	other, err := acquireGraphqlScope("table", "other")
	require.NoError(t, err)
	other()
	release()
	releaseAgain, err := acquireGraphqlScope("table", "repo")
	require.NoError(t, err)
	releaseAgain()
}

type graphqlContextOverride struct {
	plugin.SubTaskContext
	ctx context.Context
}

func (s graphqlContextOverride) GetContext() context.Context { return s.ctx }

func TestGraphqlCancelledEmptyInput(t *testing.T) {
	db := mockdal.NewDal(t)
	c := newResumeTestCollector(t, db, func(http.ResponseWriter, *http.Request) { t.Error("unexpected HTTP") })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.args.Ctx = graphqlContextOverride{c.args.Ctx, ctx}
	c.args.Input = &failingGraphqlInput{}
	require.ErrorIs(t, c.Execute(), context.Canceled)
}

func TestGraphqlCancellationDuringRateWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &GraphqlAsyncClient{ctx: ctx, logger: unithelper.DummyLogger(), rateExhaustCond: sync.NewCond(&sync.Mutex{}), maxRetry: 1}
	done := make(chan error, 1)
	go func() { _, err := client.Query(&resumeTestQuery{}, nil); done <- err }()
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("rate-limit wait ignored cancellation")
	}
	client.Wait()
}

func TestGraphqlCancellationDuringRetry(t *testing.T) {
	db := mockdal.NewDal(t)
	requested := make(chan struct{}, 1)
	c := newResumeTestCollector(t, db, func(w http.ResponseWriter, _ *http.Request) {
		requested <- struct{}{}
		http.Error(w, "retry", 503)
	})
	client := c.args.GraphqlClient
	client.maxRetry, client.waitBeforeRetry = 3, time.Hour
	done := make(chan error, 1)
	go func() { _, err := client.Query(&resumeTestQuery{}, nil); done <- err }()
	<-requested
	client.cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("retry wait ignored cancellation")
	}
}

func TestGraphqlInvalidPageDoesNotAdvanceCheckpoint(t *testing.T) {
	for _, mode := range []string{"nil", "error", "empty-cursor", "same-cursor"} {
		t.Run(mode, func(t *testing.T) {
			db := mockdal.NewDal(t)
			c := newResumeTestCollector(t, db, func(w http.ResponseWriter, _ *http.Request) { resumeTestResponse(w, true) })
			expectInput(db, "null", &GraphqlCollectorState{TaskID: 1, SkipCursor: "saved"}, nil)
			c.args.GetPageInfo = func(interface{}, *GraphqlCollectorArgs) (*GraphqlQueryPageInfo, error) {
				switch mode {
				case "nil":
					return nil, nil
				case "error":
					return nil, errors.New("invalid page")
				case "empty-cursor":
					return &GraphqlQueryPageInfo{HasNextPage: true}, nil
				default:
					return &GraphqlQueryPageInfo{HasNextPage: true, EndCursor: "saved"}, nil
				}
			}
			tx := &checkpointTestTx{Dal: db}
			db.On("Begin").Return(tx).Once()
			c.exec(nil)
			c.args.GraphqlClient.Wait()
			require.True(t, c.HasError())
			require.True(t, tx.rolledBack)
		})
	}
}

func TestGraphqlPartialErrorCannotComplete(t *testing.T) {
	for _, ignore := range []bool{false, true} {
		db := mockdal.NewDal(t)
		c := newResumeTestCollector(t, db, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"items":{"nodes":[{"id":1}],"pageInfo":{"hasNextPage":false}}},"errors":[{"message":"upstream unavailable"}]}`))
		})
		c.args.IgnoreQueryErrors = ignore
		expectInput(db, "null", &GraphqlCollectorState{TaskID: 1}, nil)
		c.exec(nil)
		c.args.GraphqlClient.Wait()
		require.True(t, c.HasError())
	}
}

func TestGraphqlNestedCollectorRegistration(t *testing.T) {
	db := mockdal.NewDal(t)
	c := newResumeTestCollector(t, db, func(http.ResponseWriter, *http.Request) {})
	manager := &StatefulApiCollector{RawDataSubTaskArgs: c.args.RawDataSubTaskArgs}
	for i := 0; i < 2; i++ {
		args := *c.args
		args.Incremental = true
		require.NoError(t, manager.InitGraphQLCollector(args))
		nested := manager.nestedCollectors[i].(*GraphqlCollector)
		require.Equal(t, i, nested.args.checkpointIndex)
		require.True(t, nested.args.Incremental)
	}
}

func TestGraphqlParserUsesPageTransaction(t *testing.T) {
	db := mockdal.NewDal(t)
	c := newResumeTestCollector(t, db, func(w http.ResponseWriter, _ *http.Request) { resumeTestResponse(w, false) })
	expectInput(db, "null", &GraphqlCollectorState{TaskID: 1}, nil)
	tx := &checkpointTestTx{Dal: db}
	db.On("Begin").Return(tx).Once()
	c.args.ResponseParser = nil
	c.args.ResponseParserWithDal = func(_ interface{}, actual dal.Dal) ([]json.RawMessage, lakeerrors.Error) {
		require.Equal(t, reflect.ValueOf(tx).Pointer(), reflect.ValueOf(actual).Pointer())
		return nil, lakeerrors.Default.New("side effect failed")
	}
	c.exec(nil)
	c.args.GraphqlClient.Wait()
	require.True(t, c.HasError())
	require.True(t, tx.rolledBack)
	db.AssertNotCalled(t, "CreateOrUpdate", mock.Anything)
}
