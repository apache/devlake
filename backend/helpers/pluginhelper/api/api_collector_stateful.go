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
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

// StatefulApiCollector runs multiple collectors as a single subtask and maintains the state of the collector
// mainly the time range to collect across multiple collections. It is useful when you need to support timeAfter
// and diff sync for APIs that do not support filtering by the updated date.
type StatefulApiCollector struct {
	RawDataSubTaskArgs
	CollectorStateManager
	// *ApiCollector
	// *GraphqlCollector
	nestedCollectors []plugin.SubTask
}

// NewStatefulApiCollector create a new StatefulApiCollector
func NewStatefulApiCollector(args RawDataSubTaskArgs) (*StatefulApiCollector, errors.Error) {
	syncPolicy := args.Ctx.TaskContext().SyncPolicy()
	rawDataSubTask, err := NewRawDataSubTask(args)
	if err != nil {
		return nil, err
	}
	stateManager, err := NewCollectorStateManager(args.Ctx, syncPolicy, rawDataSubTask.table, rawDataSubTask.params)
	if err != nil {
		return nil, err
	}
	return &StatefulApiCollector{
		RawDataSubTaskArgs:    args,
		CollectorStateManager: *stateManager,
	}, nil
}

// InitCollector appends a new collector to the list
func (m *StatefulApiCollector) InitCollector(args ApiCollectorArgs) errors.Error {
	args.RawDataSubTaskArgs = m.RawDataSubTaskArgs
	args.Incremental = m.CollectorStateManager.IsIncremental()
	apiCollector, err := NewApiCollector(args)
	if err != nil {
		return err
	}
	m.nestedCollectors = append(m.nestedCollectors, apiCollector)
	return nil
}

// InitGraphQLCollector appends a new GraphQL collector to the list
func (m *StatefulApiCollector) InitGraphQLCollector(args GraphqlCollectorArgs) errors.Error {
	args.RawDataSubTaskArgs = m.RawDataSubTaskArgs
	// highest priority: caller may hardcode the incremental flag.
	//   e.g. github graphql pr_collector need to refetch OPENing PRs existing in the database
	if !args.Incremental {
		// medium priority: force incremental flag to false when full sync is enabled
		syncPolicy := args.Ctx.TaskContext().SyncPolicy()
		if syncPolicy != nil && syncPolicy.FullSync {
			args.Incremental = false
		} else {
			// lowest priority: use the incremental flag from the state manager
			args.Incremental = m.CollectorStateManager.IsIncremental()
		}
	}

	graphqlCollector, err := NewGraphqlCollector(args)
	if err != nil {
		return err
	}
	m.nestedCollectors = append(m.nestedCollectors, graphqlCollector)
	return nil
}

// Execute all nested collectors and save the state if all collectors succeed
func (m *StatefulApiCollector) Execute() errors.Error {
	for _, subtask := range m.nestedCollectors {
		err := subtask.Execute()
		if err != nil {
			return err
		}
	}

	return m.CollectorStateManager.Close()
}

// NewStatefulApiCollectorForFinalizableEntity aims to add timeFilter/diffSync support for
// APIs that do NOT support filtering data by the updated date. However, it comes with the
// following constraints:
//  1. The entity is a short-lived object or it is likely to be irrelevant
//     a. ci/id pipelines are short-lived objects
//     b. pull request might took a year to be closed or never, but it is likely irrelevant
//  2. The entity must be Finalizable, meaning no future modifications will happen to it once it
//     enter some sort of `Closed`/`Finished` status.
//  3. The API must fit one of the following traits:
//     a. it supports filtering by Created Date, in this case, you must implement the filtering
//     via the `UrlTemplate`, `Query` or `Header` hook based on the API specification.
//     b. or sorting by Created Date in Descending order, in this case, you must use `Concurrency`
//     or `GetNextPageCustomData` instead of `GetTotalPages` for Undetermined Strategy since we have
//     to stop the process in the middle.
//
// Assuming the API fits the bill, the strategies can be categoried into:
//   - Determined Strategy: if the API supports filtering by the Created Date, use the `GetTotalPages` hook
//   - Undetermind Strategy: if the API supports sorting by the Created Date in Descending order and
//     fetching by Page Number, use the `Concurrent` hook
//   - Sequential Strategy: if the API supports sorting by the Created Date in Descending order but
//     the next page can only be fetched by the Cursor/Token from the previous page, use the `GetNextPageCustomData` hook
func NewStatefulApiCollectorForFinalizableEntity(args FinalizableApiCollectorArgs) (plugin.SubTask, errors.Error) {
	// create a manager which could execute multiple collector but acts as a single subtask to callers
	manager, err := NewStatefulApiCollector(RawDataSubTaskArgs{
		Ctx:     args.Ctx,
		Options: args.Options,
		Params:  args.Params,
		Table:   args.Table,
	})
	if err != nil {
		return nil, err
	}

	createdAfter := manager.CollectorStateManager.GetSince()
	isIncremental := manager.CollectorStateManager.IsIncremental()

	var inputIterator Iterator
	if args.CollectNewRecordsByList.BuildInputIterator != nil {
		inputIterator, err = args.CollectNewRecordsByList.BuildInputIterator(isIncremental, createdAfter)
		if err != nil {
			return nil, err
		}
	}

	// step 1: create a collector to collect newly added records
	err = manager.InitCollector(ApiCollectorArgs{
		ApiClient: args.ApiClient,
		// common
		Input:       inputIterator,
		Incremental: isIncremental,
		UrlTemplate: args.CollectNewRecordsByList.UrlTemplate,
		Query: func(reqData *RequestData) (url.Values, errors.Error) {
			if args.CollectNewRecordsByList.Query != nil {
				return args.CollectNewRecordsByList.Query(reqData, createdAfter)
			}
			return nil, nil
		},
		Header: func(reqData *RequestData) (http.Header, errors.Error) {
			if args.CollectNewRecordsByList.Header != nil {
				return args.CollectNewRecordsByList.Header(reqData, createdAfter)
			}
			return nil, nil
		},
		MinTickInterval: args.CollectNewRecordsByList.MinTickInterval,
		ResponseParser: newStatefulResponseParser(
			args.CollectNewRecordsByList.ResponseParser,
			args.CollectNewRecordsByList.GetCreated,
			createdAfter,
		),
		AfterResponse: args.CollectNewRecordsByList.AfterResponse,
		RequestBody:   args.CollectNewRecordsByList.RequestBody,
		Method:        args.CollectNewRecordsByList.Method,
		// pagination
		PageSize:              args.CollectNewRecordsByList.PageSize,
		Concurrency:           args.CollectNewRecordsByList.Concurrency,
		GetNextPageCustomData: args.CollectNewRecordsByList.GetNextPageCustomData,
		GetTotalPages:         args.CollectNewRecordsByList.GetTotalPages,
	})

	if err != nil {
		return nil, err
	}

	if args.CollectUnfinishedDetails == nil || !isIncremental {
		return manager, nil
	}

	// step 2: create another collector to collect updated records
	// TODO: this creates cursor before previous step gets executed, which is too early, to be optimized
	input, err := args.CollectUnfinishedDetails.BuildInputIterator()
	if err != nil {
		return nil, err
	}
	err = manager.InitCollector(ApiCollectorArgs{
		ApiClient: args.ApiClient,
		// common
		Incremental: true,
		Input:       input,
		UrlTemplate: args.CollectUnfinishedDetails.UrlTemplate,
		Query: func(reqData *RequestData) (url.Values, errors.Error) {
			if args.CollectUnfinishedDetails.Query != nil {
				return args.CollectUnfinishedDetails.Query(reqData, createdAfter)
			}
			return nil, nil
		},
		Header: func(reqData *RequestData) (http.Header, errors.Error) {
			if args.CollectUnfinishedDetails.Header != nil {
				return args.CollectUnfinishedDetails.Header(reqData, createdAfter)
			}
			return nil, nil
		},
		MinTickInterval: args.CollectUnfinishedDetails.MinTickInterval,
		ResponseParser:  args.CollectUnfinishedDetails.ResponseParser,
		AfterResponse:   args.CollectUnfinishedDetails.AfterResponse,
		RequestBody:     args.CollectUnfinishedDetails.RequestBody,
		Method:          args.CollectUnfinishedDetails.Method,
	})
	return manager, err
}

// newStatefulResponseParser wraps a plugin's ResponseParser with the incremental time filter, so a
// page is cut down to the records created after the last successful collection.
//
// Both this filter and the plugin parsers it wraps stop collecting by returning the valid prefix of
// a page together with ErrFinishCollect. fetchAsync persists whatever is returned and then stops
// paginating, so the prefix must be passed through rather than dropped: discarding it silently loses
// the in-window records on the last page of a time-bounded collection.
func newStatefulResponseParser(
	parseResponse func(res *http.Response) ([]json.RawMessage, errors.Error),
	getCreated func(item json.RawMessage) (time.Time, errors.Error),
	createdAfter *time.Time,
) func(res *http.Response) ([]json.RawMessage, errors.Error) {
	return func(res *http.Response) ([]json.RawMessage, errors.Error) {
		items, err := parseResponse(res)
		parserFinished := errors.Is(err, ErrFinishCollect)
		if err != nil && !parserFinished {
			return nil, err
		}
		if len(items) == 0 {
			if parserFinished {
				return nil, ErrFinishCollect
			}
			return nil, nil
		}

		// time filter or diff sync
		if createdAfter != nil && getCreated != nil {
			// if the first record of the page was created before createdAfter and not a zero value, return empty set and stop
			firstCreated, err := getCreated(items[0])
			if err != nil {
				return nil, err
			}
			if firstCreated.Before(*createdAfter) && !firstCreated.IsZero() {
				return nil, ErrFinishCollect
			}

			// If last record was created before CreatedAfter, including a zero value, check each record individually
			lastCreated, err := getCreated(items[len(items)-1])
			if err != nil {
				return nil, err
			}
			if lastCreated.Before(*createdAfter) {
				var validItems []json.RawMessage
				// Only collect items that were created after the last successful collection to prevent duplicates
				for _, item := range items {
					itemCreatedAt, err := getCreated(item)
					if err != nil {
						return nil, err
					}

					if itemCreatedAt.IsZero() {
						// If zero then timestamp is null on the response - accept as valid for downstream processing
						validItems = append(validItems, item)
						continue
					}

					if itemCreatedAt.Before(*createdAfter) {
						// Once we reach an item that was created before the last successful collection, stop & return
						return validItems, ErrFinishCollect
					}
					validItems = append(validItems, item)
				}
			}
		}
		return items, err
	}
}

type FinalizableApiCollectorArgs struct {
	RawDataSubTaskArgs
	ApiClient                RateLimitedApiClient
	CollectNewRecordsByList  FinalizableApiCollectorListArgs
	CollectUnfinishedDetails *FinalizableApiCollectorDetailArgs
}

// FinalizableApiCollectorCommonArgs is the common arguments for both list and detail collectors
// Note that all request-related arguments would be called or utilized before any response-related arguments
type FinalizableApiCollectorCommonArgs struct {
	UrlTemplate     string                                                                          // required, url path template for the request, e.g. repos/{{ .Params.Name }}/pulls or incident/{{ .Input.Number }} (if using iterators)
	Method          string                                                                          // optional, request method, e.g. GET(default), POST, PUT, DELETE
	Query           func(reqData *RequestData, createdAfter *time.Time) (url.Values, errors.Error)  // optional, build query params for the request
	Header          func(reqData *RequestData, createdAfter *time.Time) (http.Header, errors.Error) // optional, build header for the request
	RequestBody     func(reqData *RequestData) map[string]interface{}                               // optional, build request body for the request if the Method set to POST or PUT
	MinTickInterval *time.Duration                                                                  // optional, minimum interval between two requests, some endpoints might have a more conservative rate limit than others within the same instance, you can mitigate this by setting a higher MinTickInterval to override the connection level rate limit.
	AfterResponse   plugin.ApiClientAfterResponse                                                   // optional, hook to run after each response, would be called before the ResponseParser
	ResponseParser  func(res *http.Response) ([]json.RawMessage, errors.Error)                      // required, parse the response body and return a list of entities
}

// FinalizableApiCollectorListArgs is the arguments for the list collector
type FinalizableApiCollectorListArgs struct {
	FinalizableApiCollectorCommonArgs
	GetCreated            func(item json.RawMessage) (time.Time, errors.Error)                                        // optional, to extract create date from a raw json of a single record, leave it be `nil` if API supports filtering by updated date (Don't forget to set the Query)
	PageSize              int                                                                                         // required, number of records per page
	Concurrency           int                                                                                         // required for Undetermined Strategy, number of concurrent requests
	GetNextPageCustomData func(prevReqData *RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) // required for Sequential Strategy, to extract the next page cursor from the given response
	GetTotalPages         func(res *http.Response, args *ApiCollectorArgs) (int, errors.Error)                        // required for Determined Strategy, to extract the total number of pages from the given response
	BuildInputIterator    func(isIncremental bool, createdAfter *time.Time) (Iterator, errors.Error)
}

// FinalizableApiCollectorDetailArgs is the arguments for the detail collector
type FinalizableApiCollectorDetailArgs struct {
	FinalizableApiCollectorCommonArgs
	BuildInputIterator func() (Iterator, errors.Error) // required, create an iterator that iterates through all unfinalized records in the database. These records will be fed as the "Input" (or {{ .Input.* }} in URLTemplate) argument back into FinalizableApiCollectorCommonArgs which makes the API calls to re-collect their newest states.
}
