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
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	plugin "github.com/apache/devlake/core/plugin"
	"github.com/merico-ai/graphql"
)

// CursorPager contains pagination information for a graphql request
type CursorPager struct {
	SkipCursor *string
	Size       int
}

// GraphqlRequestData is the input of `UrlTemplate` `BuildQuery` and `Header`, so we can generate them dynamically
type GraphqlRequestData struct {
	Pager     *CursorPager
	Params    interface{}
	Input     interface{}
	InputJSON []byte
}

// GraphqlQueryPageInfo contains the pagination data
type GraphqlQueryPageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

// DateTime is the type of time in Graphql
// graphql lib can only read this name...
type DateTime struct{ time.Time }

// GraphqlAsyncResponseHandler callback function to handle the Response asynchronously
type GraphqlAsyncResponseHandler func(res *http.Response) error

// GraphqlCollectorArgs arguments needed by GraphqlCollector
type GraphqlCollectorArgs struct {
	RawDataSubTaskArgs
	// BuildQuery would be sent out as part of the request URL
	BuildQuery func(reqData *GraphqlRequestData) (query interface{}, variables map[string]interface{}, err error)
	// PageSize tells ApiCollector the page size
	PageSize int
	// GraphqlClient is a asynchronize api request client with qps
	GraphqlClient *GraphqlAsyncClient
	// Input helps us collect data based on previous collected data, like collecting changelogs based on jira
	// issue ids
	Input Iterator
	// how many times fetched from input, default 1 means only fetch once
	// NOTICE: InputStep=1 will fill value as item and InputStep>1 will fill value as []item
	InputStep int
	// Incremental indicate if this is a incremental collection, the existing data won't get deleted if it was true
	Incremental bool `comment:"indicate if this collection is incremental update"`
	// GetPageInfo is to tell `GraphqlCollector` is page information
	GetPageInfo func(query interface{}, args *GraphqlCollectorArgs) (*GraphqlQueryPageInfo, error)
	BatchSize   int
	// Exactly one of ResponseParser and ResponseParserWithDal is required.
	ResponseParser func(queryWrapper interface{}) ([]json.RawMessage, errors.Error)
	// ResponseParserWithDal must use the supplied transaction for all DB side
	// effects. They commit atomically with raw rows and the input checkpoint.
	ResponseParserWithDal func(queryWrapper interface{}, db dal.Dal) ([]json.RawMessage, errors.Error)
	IgnoreQueryErrors     bool
	// Stable position when a subtask contains multiple collectors sharing a table.
	checkpointIndex    int
	scopeLocked        bool
	checkpointContract string
	inputDigest        string
}

// GraphqlCollector help you collect data from Graphql services
type GraphqlCollector struct {
	*RawDataSubTask
	args             *GraphqlCollectorArgs
	workerErrors     []error
	batchSave        *BatchSave
	taskID           uint64
	scopeHash        string
	errorsMu         sync.Mutex
	pageMu           sync.Mutex
	expectedRawCount *int64
}

// ErrFinishCollect is an error which will finish this collector
var ErrFinishCollect = errors.Default.New("finish collect")

// NewGraphqlCollector allocates a new GraphqlCollector with the given args.
// GraphqlCollector can help us collect data from api with ease, pass in a AsyncGraphqlClient and tell it which part
// of response we want to save, GraphqlCollector will collect them from remote server and store them into database.
func NewGraphqlCollector(args GraphqlCollectorArgs) (*GraphqlCollector, errors.Error) {
	// process args
	if args.BuildQuery == nil || args.InputStep < 0 || args.PageSize < 0 || args.BatchSize < 0 {
		return nil, errors.BadInput.New("GraphQL BuildQuery is required and collection sizes must not be negative")
	}
	rawDataSubTask, err := NewRawDataSubTask(args.RawDataSubTaskArgs)
	if err != nil {
		return nil, err
	}
	if args.GraphqlClient == nil {
		return nil, errors.Default.New("ApiClient is required")
	}
	if args.ResponseParser == nil && args.ResponseParserWithDal == nil {
		return nil, errors.Default.New("one of ResponseParser and ResponseParserWithDal is required")
	}
	if args.ResponseParser != nil && args.ResponseParserWithDal != nil {
		return nil, errors.BadInput.New("only one GraphQL response parser may be supplied")
	}
	if args.BatchSize == 0 {
		args.BatchSize = 100
	}
	if args.InputStep == 0 {
		args.InputStep = 1
	}
	apiCollector := &GraphqlCollector{
		RawDataSubTask: rawDataSubTask,
		args:           &args,
		taskID:         plugin.TaskID(args.Ctx.GetContext()),
		batchSave: errors.Must1(NewBatchSave(
			args.Ctx,
			reflect.TypeOf(&RawData{}),
			args.BatchSize,
			rawDataSubTask.table,
		)),
	}
	return apiCollector, nil
}

// Execute api collection
func (collector *GraphqlCollector) Execute() errors.Error {
	inputClosed := false
	if collector.args.Input != nil {
		defer func() {
			if !inputClosed {
				_ = collector.args.Input.Close()
			}
		}()
	}
	if err := collector.args.Ctx.GetContext().Err(); err != nil {
		return errors.Convert(err)
	}
	if !collector.args.scopeLocked {
		release, err := acquireGraphqlScope(collector.table, collector.params)
		if err != nil {
			return err
		}
		defer release()
	}
	if collector.taskID != 0 {
		binary, err := graphqlBinaryIdentity()
		if err != nil {
			return errors.Convert(err)
		}
		contract, err := json.Marshal([]interface{}{binary, plugin.TaskCode(collector.args.Ctx.GetContext()), collector.args.InputStep, collector.args.PageSize, collector.args.GetPageInfo != nil})
		if err != nil {
			return errors.Convert(err)
		}
		collector.args.checkpointContract = graphqlCollectorInputHash(string(contract))
		if collector.args.Input != nil {
			inputClosed = true
			staged, digest, err := stageGraphqlInput(collector.args.Ctx.GetContext(), collector.args.Input)
			if err != nil {
				return err
			}
			collector.args.Input = staged
			collector.args.inputDigest = digest
			inputClosed = false
		}
	}
	logger := collector.args.Ctx.GetLogger()
	logger.Info("start graphql collection")

	// make sure table is created
	db := collector.args.Ctx.GetDal()
	err := db.AutoMigrate(&RawData{}, dal.From(collector.table))
	if err != nil {
		return errors.Default.Wrap(err, "error running auto-migrate")
	}
	completed, err := collector.prepareCheckpoint()
	if err != nil {
		return err
	}
	if completed {
		if collector.args.Input != nil {
			inputClosed = true
			return collector.args.Input.Close()
		}
		return nil
	}

	collector.args.Ctx.SetProgress(0, -1)
	if collector.args.Input != nil {
		iterator := collector.args.Input
		// the comment about difference is written at GraphqlCollectorArgs.InputStep
		if collector.args.InputStep == 1 {
			for iterator.HasNext() && !collector.HasError() {
				input, err := iterator.Fetch()
				if err != nil {
					collector.checkError(err)
					break
				}
				collector.exec(input)
			}
		} else {
			for !collector.HasError() {
				var inputs []interface{}
				for i := 0; i < collector.args.InputStep && iterator.HasNext(); i++ {
					input, err := iterator.Fetch()
					if err != nil {
						collector.checkError(err)
						break
					}
					inputs = append(inputs, input)
				}
				if inputs == nil {
					break
				}
				collector.exec(inputs)
			}
		}
	} else {
		// or we just did it once
		collector.exec(nil)
	}

	logger.Debug("wait for all async api to finished")
	collector.args.GraphqlClient.Wait()
	collector.checkError(collector.args.Ctx.GetContext().Err())
	if collector.args.Input != nil {
		if source, ok := collector.args.Input.(interface{ Err() error }); ok {
			collector.checkError(source.Err())
		}
		collector.checkError(collector.args.Input.Close())
		inputClosed = true
	}

	if collector.HasError() {
		err = errors.Default.Combine(collector.workerErrors)
		logger.Error(err, "ended Graphql collector execution with error")
		logger.Error(collector.workerErrors[0], "the first error of them")
		return err
	} else {
		logger.Info("ended api collection without error")
	}

	err = collector.batchSave.Close()
	if err != nil {
		return err
	}
	return collector.completeCheckpoint()
}

func (collector *GraphqlCollector) exec(input interface{}) {
	inputJson, err := json.Marshal(input)
	if err != nil {
		collector.checkError(errors.Default.Wrap(err, `input can not be marshal to json`))
		return
	}
	reqData := new(GraphqlRequestData)
	reqData.Input = input
	reqData.InputJSON = inputJson
	reqData.Pager = &CursorPager{
		SkipCursor: nil,
		Size:       collector.args.PageSize,
	}
	if collector.taskID != 0 {
		state, err := loadGraphqlCollectorState(collector.args.Ctx.GetDal(), collector.scopeHash, graphqlCollectorInputHash(string(inputJson)))
		if err != nil {
			collector.checkError(err)
			return
		}
		if state != nil && state.TaskID == collector.taskID {
			if state.Completed {
				return
			}
			if state.SkipCursor != "" {
				reqData.Pager.SkipCursor = &state.SkipCursor
			}
		}
	}
	collector.fetchAsync(reqData)
}

func (collector *GraphqlCollector) fetchAsync(reqData *GraphqlRequestData) {
	if collector.HasError() {
		return
	}
	if reqData.Pager == nil {
		reqData.Pager = &CursorPager{
			SkipCursor: nil,
			Size:       collector.args.PageSize,
		}
	}
	query, variables, err := collector.args.BuildQuery(reqData)
	if err != nil {
		collector.checkError(errors.Default.Wrap(err, `graphql collector BuildQuery failed`))
		return
	}

	logger := collector.args.Ctx.GetLogger()
	db := collector.args.Ctx.GetDal()
	dataErrors, err := collector.args.GraphqlClient.Query(query, variables)
	if err != nil {
		if err == context.Canceled {
			// direct error message for error combine
			collector.checkError(err)
		} else {
			collector.checkError(errors.Default.Wrap(err, `graphql query failed`))
		}
		return
	}
	if len(dataErrors) > 0 {
		if !collector.args.IgnoreQueryErrors || collector.taskID != 0 {
			hasNonIgnorableDataErrors := false
			for _, dataError := range dataErrors {
				if isIgnorableGraphqlQueryError(dataError) {
					logger.Warn(nil, "Issue may have been transferred or deleted.")
					continue
				}
				hasNonIgnorableDataErrors = true
				collector.checkError(errors.Default.Wrap(dataError, `graphql query got error`))
			}
			if hasNonIgnorableDataErrors {
				return
			}
		}
		// Task-less legacy callers may explicitly opt into partial responses.
	}
	defer logger.Debug("fetchAsync >>> done for %v %v", query, variables)

	queryStr, _ := graphql.ConstructQuery(query, variables)
	variablesJson, err := json.Marshal(variables)
	if err != nil {
		collector.checkError(errors.Default.Wrap(err, `variables in graphql query can not marshal to json`))
		return
	}

	var pageInfo *GraphqlQueryPageInfo
	var insertedRows int64
	// Parse within the transaction too: parsers may delete stale source data.
	savePage := func(db dal.Dal) errors.Error {
		var results []json.RawMessage
		var parseErr errors.Error
		if collector.args.ResponseParserWithDal != nil {
			results, parseErr = collector.args.ResponseParserWithDal(query, db)
		} else {
			results, parseErr = collector.args.ResponseParser(query)
		}
		finished := errors.Is(parseErr, ErrFinishCollect)
		if parseErr != nil && !finished {
			return errors.Default.Wrap(parseErr, "graphql response parser failed")
		}
		if !finished && collector.args.GetPageInfo != nil {
			var pageErr error
			pageInfo, pageErr = collector.args.GetPageInfo(query, collector.args)
			if pageErr != nil {
				return errors.Convert(pageErr)
			}
			if pageInfo == nil {
				return errors.Default.New("graphql pageInfo is nil")
			}
			if pageInfo.HasNextPage && (pageInfo.EndCursor == "" || (reqData.Pager.SkipCursor != nil && pageInfo.EndCursor == *reqData.Pager.SkipCursor)) {
				return errors.Default.New("graphql cursor did not advance")
			}
		}
		if err := collector.args.Ctx.GetContext().Err(); err != nil {
			return errors.Convert(err)
		}
		for _, result := range results {
			if err := db.Create(&RawData{Params: collector.params, Data: result, Url: queryStr, Input: variablesJson}, dal.From(collector.table)); err != nil {
				return err
			}
			insertedRows++
		}
		if collector.taskID != 0 {
			state := collector.checkpoint(graphqlCollectorInputHash(string(reqData.InputJSON)))
			state.Completed = pageInfo == nil || !pageInfo.HasNextPage
			if !state.Completed {
				state.SkipCursor = pageInfo.EndCursor
			}
			return saveGraphqlCollectorState(db, state)
		}
		return nil
	}
	var saveErr errors.Error
	collector.pageMu.Lock()
	defer collector.pageMu.Unlock()
	if collector.taskID != 0 || collector.args.ResponseParserWithDal != nil {
		tx := db.Begin()
		defer tx.Rollback()
		saveErr = savePage(tx)
		var rawCount int64
		if saveErr == nil && collector.args.checkpointContract != "" {
			// Pure parsers only append rows. Recounting the entire growing raw
			// scope on every page would make a long collection quadratic.
			if collector.args.ResponseParserWithDal == nil && collector.expectedRawCount != nil {
				rawCount = *collector.expectedRawCount + insertedRows
			} else {
				// Transactional parsers may also delete raw rows.
				rawCount, saveErr = tx.Count(dal.From(collector.table), dal.Where("params = ?", collector.params))
			}
			if saveErr == nil {
				marker := collector.rawCheckpoint()
				marker.RawCount = &rawCount
				saveErr = saveGraphqlCollectorState(tx, marker)
			}
		}
		if saveErr == nil {
			saveErr = tx.Commit()
			if saveErr == nil && collector.args.checkpointContract != "" {
				collector.expectedRawCount = &rawCount
			}
		}
	} else {
		saveErr = savePage(db)
	}
	if saveErr != nil {
		collector.checkError(saveErr)
		return
	}
	collector.args.Ctx.IncProgress(1)
	if pageInfo != nil && pageInfo.HasNextPage {
		next := *reqData
		next.Pager = &CursorPager{SkipCursor: &pageInfo.EndCursor, Size: collector.args.PageSize}
		collector.args.GraphqlClient.NextTick(func() errors.Error {
			collector.fetchAsync(&next)
			return nil
		}, collector.checkError)
	}
}

func (collector *GraphqlCollector) checkError(err error) {
	if err == nil {
		return
	}
	collector.errorsMu.Lock()
	defer collector.errorsMu.Unlock()
	collector.workerErrors = append(collector.workerErrors, err)
}

// HasError return if any error occurred
func (collector *GraphqlCollector) HasError() bool {
	collector.errorsMu.Lock()
	defer collector.errorsMu.Unlock()
	return len(collector.workerErrors) > 0
}

func isIgnorableGraphqlQueryError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Could not resolve to an Issue")
}

var _ plugin.SubTask = (*GraphqlCollector)(nil)
