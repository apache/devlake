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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	plugin "github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/utils"
)

// HttpMinStatusRetryCode is which status will retry
var HttpMinStatusRetryCode = http.StatusBadRequest

// ApiAsyncClient is built on top of ApiClient, to provide a asynchronous semantic
// You may submit multiple requests at once by calling `DoGetAsync`, and those requests
// will be performed in parallel with rate-limit support
type ApiAsyncClient struct {
	*ApiClient
	*WorkerScheduler
	maxRetry     int
	numOfWorkers int
	logger       log.Logger
}

const defaultTimeout = 120 * time.Second

// CreateAsyncApiClient creates a new ApiAsyncClient
func CreateAsyncApiClient(
	taskCtx plugin.TaskContext,
	apiClient *ApiClient,
	rateLimiter *ApiRateLimitCalculator,
) (*ApiAsyncClient, errors.Error) {
	// load retry/timeout from configuration
	retry, err := utils.StrToIntOr(taskCtx.GetConfig("API_RETRY"), 3)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to parse API_RETRY")
	}

	timeoutConf := taskCtx.GetConfig("API_TIMEOUT")
	if timeoutConf != "" {
		// override timeout value if API_TIMEOUT is provided
		timeout, err := time.ParseDuration(timeoutConf)
		if err != nil {
			return nil, errors.BadInput.Wrap(err, "failed to parse API_TIMEOUT")
		}
		apiClient.SetTimeout(timeout)
	} else if apiClient.GetTimeout() == 0 {
		// Use DEFAULT_TIMEOUT when API_TIMEOUT is empty and ApiClient has no timeout set
		apiClient.SetTimeout(defaultTimeout)
	}

	apiClient.SetLogger(taskCtx.GetLogger())

	globalRateLimitPerHour, err := utils.StrToIntOr(taskCtx.GetConfig("API_REQUESTS_PER_HOUR"), 18000)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to parse API_REQUESTS_PER_HOUR")
	}
	if rateLimiter == nil {
		rateLimiter = &ApiRateLimitCalculator{}
	}
	rateLimiter.GlobalRateLimitPerHour = globalRateLimitPerHour
	rateLimiter.MaxRetry = retry

	// ok, calculate api rate limit based on response (normally from headers)
	requests, duration, err := rateLimiter.Calculate(apiClient)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to calculate rateLimit for api")
	}

	// it is hard to tell how many workers would be sufficient, it depends on how slow the server responds.
	// we need more workers when server is responding slowly, because requests are sent in a fixed pace.
	// and because workers are relatively cheap, lets assume response takes 5 seconds
	const RESPONSE_TIME = 5 * time.Second
	// in order for scheduler to hold requests of 3 seconds, we need:
	d := duration / RESPONSE_TIME
	numOfWorkers := requests / int(d)
	tickInterval, err := CalcTickInterval(requests, duration)
	if err != nil {
		return nil, err
	}

	logger := taskCtx.GetLogger().Nested("api async client")
	logger.Info(
		"creating scheduler for api \"%s\", number of workers: %d, %d reqs / %s (interval: %s)",
		apiClient.GetEndpoint(),
		numOfWorkers,
		requests,
		duration.String(),
		tickInterval.String(),
	)
	scheduler, err := NewWorkerScheduler(
		taskCtx.GetContext(),
		numOfWorkers,
		tickInterval,
		logger,
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create scheduler")
	}

	// finally, wrap around api client with async semantic
	return &ApiAsyncClient{
		apiClient,
		scheduler,
		retry,
		numOfWorkers,
		logger,
	}, nil
}

// GetMaxRetry returns the maximum retry attempts for a request
func (apiClient *ApiAsyncClient) GetMaxRetry() int {
	return apiClient.maxRetry
}

// SetMaxRetry sets the maximum retry attempts for a request
func (apiClient *ApiAsyncClient) SetMaxRetry(
	maxRetry int,
) {
	apiClient.maxRetry = maxRetry
}

// DoAsync would carry out an asynchronous request
func (apiClient *ApiAsyncClient) DoAsync(
	method string,
	path string,
	query url.Values,
	body interface{},
	header http.Header,
	handler plugin.ApiAsyncCallback,
	retry int,
) {
	var request func() errors.Error
	request = func() errors.Error {
		var err error
		var res *http.Response
		var respBody []byte

		apiClient.logger.Debug("endpoint: %s  method: %s  header: %s  body: %s query: %s", path, method, header, body, query)
		res, err = apiClient.Do(method, path, query, body, header)
		if err == ErrIgnoreAndContinue {
			// make sure defer func got be executed
			err = nil //nolint
			return nil
		}

		// make sure response body is read successfully, or we might have to retry
		if err == nil {
			// make sure response.Body stream will be closed to avoid running out of file handle
			defer func(readCloser io.ReadCloser) { _ = readCloser.Close() }(res.Body)
			// replace NetworkStream with MemoryBuffer
			respBody, err = io.ReadAll(res.Body)
			if err == nil {
				res.Body = io.NopCloser(bytes.NewBuffer(respBody))
			}
		}

		// check
		needRetry := false
		errMessage := "unknown"
		if err != nil {
			needRetry = true
			errMessage = err.Error()
		} else if res.StatusCode >= HttpMinStatusRetryCode {
			needRetry = true
			errMessage = fmt.Sprintf("Http DoAsync error calling [method:%s path:%s query:%s]. Response: %s", method, path, query, string(respBody))
			err = errors.HttpStatus(res.StatusCode).New(errMessage)
		}

		//  if it needs retry, check and retry
		if needRetry {
			// check whether we still have retry times and not error from handler and canceled error
			if retry < apiClient.maxRetry && err != context.Canceled {
				apiClient.logger.Warn(err, "retry #%d calling %s", retry, path)
				retry++
				apiClient.NextTick(func() errors.Error {
					apiClient.SubmitBlocking(request)
					return nil
				})
				return nil
			}
		}

		if err != nil {
			err = errors.Default.Wrap(err, fmt.Sprintf("Retry exceeded %d times calling %s. The last error was: %s", retry, path, errMessage))
			apiClient.logger.Error(err, "")
			return errors.Convert(err)
		}

		// it is important to let handler have a chance to handle error, or it can hang indefinitely
		// when error occurs
		return handler(res)
	}
	apiClient.SubmitBlocking(request)
}

// DoGetAsync Enqueue an api get request, the request may be sent sometime in future in parallel with other api requests
func (apiClient *ApiAsyncClient) DoGetAsync(
	path string,
	query url.Values,
	header http.Header,
	handler plugin.ApiAsyncCallback,
) {
	apiClient.DoAsync(http.MethodGet, path, query, nil, header, handler, 0)
}

// DoPostAsync Enqueue an api post request, the request may be sent sometime in future in parallel with other api requests
func (apiClient *ApiAsyncClient) DoPostAsync(
	path string,
	query url.Values,
	body interface{},
	header http.Header,
	handler plugin.ApiAsyncCallback,
) {
	apiClient.DoAsync(http.MethodPost, path, query, body, header, handler, 0)
}

// GetNumOfWorkers to return the Workers count if scheduler.
func (apiClient *ApiAsyncClient) GetNumOfWorkers() int {
	return apiClient.numOfWorkers
}

// RateLimitedApiClient FIXME ...
type RateLimitedApiClient interface {
	DoGetAsync(path string, query url.Values, header http.Header, handler plugin.ApiAsyncCallback)
	DoPostAsync(path string, query url.Values, body interface{}, header http.Header, handler plugin.ApiAsyncCallback)
	WaitAsync() errors.Error
	HasError() bool
	NextTick(task func() errors.Error)
	GetNumOfWorkers() int
	GetAfterFunction() plugin.ApiClientAfterResponse
	SetAfterFunction(callback plugin.ApiClientAfterResponse)
	Reset(d time.Duration)
	GetTickInterval() time.Duration
	Release()
}

var _ RateLimitedApiClient = (*ApiAsyncClient)(nil)
