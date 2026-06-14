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
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/utils"

	"github.com/merico-ai/graphql"
)

// GraphqlAsyncClient send graphql one by one
type GraphqlAsyncClient struct {
	ctx       context.Context
	cancel    context.CancelFunc
	client    *graphql.Client
	logger    log.Logger
	mu        sync.Mutex
	waitGroup sync.WaitGroup

	maxRetry         int
	waitBeforeRetry  time.Duration
	rateExhaustCond  *sync.Cond
	rateRemaining    int
	getRateRemaining func(context.Context, *graphql.Client, log.Logger) (rateRemaining int, resetAt *time.Time, err errors.Error)
	getRateCost      func(q interface{}) int
}

// defaultRateLimitConst is the generic fallback rate limit for GraphQL requests.
// It is used as the initial remaining quota when dynamic rate limit
// information is unavailable from the provider.
const defaultRateLimitConst = 1000

// CreateAsyncGraphqlClient creates a new GraphqlAsyncClient
func CreateAsyncGraphqlClient(
	taskCtx plugin.TaskContext,
	graphqlClient *graphql.Client,
	logger log.Logger,
	getRateRemaining func(context.Context, *graphql.Client, log.Logger) (rateRemaining int, resetAt *time.Time, err errors.Error),
	opts ...func(*GraphqlAsyncClient),
) (*GraphqlAsyncClient, errors.Error) {
	ctxWithCancel, cancel := context.WithCancel(taskCtx.GetContext())

	graphqlAsyncClient := &GraphqlAsyncClient{
		ctx:              ctxWithCancel,
		cancel:           cancel,
		client:           graphqlClient,
		logger:           logger,
		rateExhaustCond:  sync.NewCond(&sync.Mutex{}),
		rateRemaining:    defaultRateLimitConst,
		getRateRemaining: getRateRemaining,
	}

	// apply options
	for _, opt := range opts {
		opt(graphqlAsyncClient)
	}

	// Env config wins over everything, only if explicitly set
	if rateLimit := resolveRateLimit(taskCtx, logger); rateLimit != -1 {
		logger.Info("GRAPHQL_RATE_LIMIT env override applied: %d (was %d)", rateLimit, graphqlAsyncClient.rateRemaining)
		graphqlAsyncClient.rateRemaining = rateLimit
	}

	if getRateRemaining != nil {
		rateRemaining, resetAt, err := getRateRemaining(taskCtx.GetContext(), graphqlClient, logger)
		if err != nil {
			graphqlAsyncClient.logger.Info("failed to fetch initial graphql rate limit, fallback to default: %v", err)
			graphqlAsyncClient.updateRateRemaining(graphqlAsyncClient.rateRemaining, nil)
		} else {
			graphqlAsyncClient.updateRateRemaining(rateRemaining, resetAt)
		}
	} else {
		graphqlAsyncClient.updateRateRemaining(graphqlAsyncClient.rateRemaining, nil)
	}

	// load retry/timeout from configuration
	// use API_RETRY as max retry time
	// use API_TIMEOUT as retry before wait seconds to confirm the prev request finish
	timeout := 30 * time.Second
	retry, err := utils.StrToIntOr(taskCtx.GetConfig("API_RETRY"), 3)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to parse API_RETRY")
	}
	timeoutConf := taskCtx.GetConfig("API_TIMEOUT")
	if timeoutConf != "" {
		// override timeout value if API_TIMEOUT is provided
		timeout, err = errors.Convert01(time.ParseDuration(timeoutConf))
		if err != nil {
			return nil, errors.BadInput.Wrap(err, "failed to parse API_TIMEOUT")
		}
	}
	graphqlAsyncClient.SetMaxRetry(retry, timeout)

	return graphqlAsyncClient, nil
}

// GetMaxRetry returns the maximum retry attempts for a request
func (apiClient *GraphqlAsyncClient) GetMaxRetry() (int, time.Duration) {
	return apiClient.maxRetry, apiClient.waitBeforeRetry
}

// SetMaxRetry sets the maximum retry attempts for a request
func (apiClient *GraphqlAsyncClient) SetMaxRetry(
	maxRetry int,
	waitBeforeRetry time.Duration,
) {
	apiClient.maxRetry = maxRetry
	apiClient.waitBeforeRetry = waitBeforeRetry
}

// updateRateRemaining call getRateRemaining to update rateRemaining periodically
func (apiClient *GraphqlAsyncClient) updateRateRemaining(rateRemaining int, resetAt *time.Time) {
	apiClient.rateRemaining = rateRemaining
	if rateRemaining > 0 {
		apiClient.rateExhaustCond.Signal()
	}
	go func() {
		if apiClient.getRateRemaining == nil {
			return
		}

		nextDuring := 3 * time.Minute
		if resetAt != nil && resetAt.After(time.Now()) {
			nextDuring = time.Until(*resetAt)
		}
		select {
		case <-apiClient.ctx.Done():
			// finish go routine when context done
			return
		case <-time.After(nextDuring):
			newRateRemaining, newResetAt, err := apiClient.getRateRemaining(apiClient.ctx, apiClient.client, apiClient.logger)
			if err != nil {
				apiClient.logger.Info("failed to update graphql rate limit, will retry next cycle: %v", err)
				// Floor the reused value so Signal() always fires; prevents deadlock when
				// rateRemaining is 0 and the rate-limit endpoint keeps erroring (e.g. GHE).
				fallback := apiClient.rateRemaining
				if fallback < defaultRateLimitConst {
					fallback = defaultRateLimitConst
				}
				apiClient.updateRateRemaining(fallback, nil)
				return
			}
			apiClient.updateRateRemaining(newRateRemaining, newResetAt)
		}
	}()
}

// SetGetRateCost to calculate how many rate cost
// if not set, all query just cost 1
func (apiClient *GraphqlAsyncClient) SetGetRateCost(getRateCost func(q interface{}) int) {
	apiClient.getRateCost = getRateCost
}

// Query send a graphql request when get lock
// []graphql.DataError are the errors returned in response body
// errors.Error is other error
func (apiClient *GraphqlAsyncClient) Query(q interface{}, variables map[string]interface{}) ([]graphql.DataError, error) {
	apiClient.waitGroup.Add(1)
	defer apiClient.waitGroup.Done()
	apiClient.mu.Lock()
	defer apiClient.mu.Unlock()

	apiClient.rateExhaustCond.L.Lock()
	defer apiClient.rateExhaustCond.L.Unlock()
	for apiClient.rateRemaining <= 0 {
		apiClient.logger.Info(`rate limit remaining exhausted, waiting for next period.`)
		apiClient.rateExhaustCond.Wait()
	}

	retryTime := 0
	var err error
	//  if it needs retry, check and retry
	for retryTime < apiClient.maxRetry {
		select {
		case <-apiClient.ctx.Done():
			return nil, nil
		default:
			var dataErrors []graphql.DataError
			dataErrors, err := apiClient.client.Query(apiClient.ctx, q, variables)
			if err == context.Canceled {
				return nil, err
			}
			if err != nil {
				apiClient.logger.Warn(err, "retry #%d graphql calling after %ds", retryTime, apiClient.waitBeforeRetry/time.Second)
				retryTime++
				<-time.After(apiClient.waitBeforeRetry)
				continue
			}
			if dataErrors != nil {
				return dataErrors, nil
			}
			cost := 1
			if apiClient.getRateCost != nil {
				cost = apiClient.getRateCost(q)
			}
			apiClient.rateRemaining -= cost
			apiClient.logger.Debug(`query cost %d in %v`, cost, variables)
			return nil, nil
		}
	}
	return nil, errors.Default.Wrap(err, fmt.Sprintf("got error when querying GraphQL (from the %dth retry)", retryTime))
}

// NextTick to return the NextTick of scheduler
func (apiClient *GraphqlAsyncClient) NextTick(task func() errors.Error, taskErrorChecker func(err error)) {
	// to make sure task will be enqueued
	apiClient.waitGroup.Add(1)
	go func() {
		select {
		case <-apiClient.ctx.Done():
			return
		default:
			go func() {
				// if set waitGroup done here, a serial of goroutine will block until sub-goroutine finish.
				// But if done out of this go func, so task will run after waitGroup finish
				// I have no idea about this now...
				defer apiClient.waitGroup.Done()
				taskErrorChecker(task())
			}()
		}
	}()
}

// Wait blocks until all async requests were done
func (apiClient *GraphqlAsyncClient) Wait() {
	apiClient.waitGroup.Wait()
}

// Release will release the ApiAsyncClient with scheduler
func (apiClient *GraphqlAsyncClient) Release() {
	apiClient.cancel()
}

// WithFallbackRateLimit sets the initial/fallback rate limit used when
// rate limit information cannot be fetched dynamically.
// This value may be overridden later by getRateRemaining.
func WithFallbackRateLimit(limit int) func(*GraphqlAsyncClient) {
	return func(c *GraphqlAsyncClient) {
		if limit > 0 {
			c.rateRemaining = limit
		}
	}
}

// resolveRateLimit returns -1 if GRAPHQL_RATE_LIMIT is not set or invalid
func resolveRateLimit(taskCtx plugin.TaskContext, logger log.Logger) int {
	if v := taskCtx.GetConfig("GRAPHQL_RATE_LIMIT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
		logger.Warn(nil, "invalid GRAPHQL_RATE_LIMIT, using default")
	}
	return -1
}
