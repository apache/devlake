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

package tasks

import (
	"net/http"
	"strconv"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func CreateApiClient(taskCtx plugin.TaskContext, connection *models.GithubConnection) (*api.ApiAsyncClient, errors.Error) {
	apiClient, err := api.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}

	// inject the shared auth layer
	_, err = CreateAuthenticatedHttpClient(taskCtx, connection, apiClient.GetClient())
	if err != nil {
		return nil, err
	}

	// create rate limit calculator
	rateLimiter := &api.ApiRateLimitCalculator{
		UserRateLimitPerHour: connection.RateLimitPerHour,
		Method:               http.MethodGet,
		DynamicRateLimit: func(res *http.Response) (int, time.Duration, errors.Error) {
			/* calculate by number of remaining requests
			remaining, err := strconv.Atoi(res.Header.Get("X-RateLimit-Remaining"))
			if err != nil {
				return 0,0, errors.Default.New("failed to parse X-RateLimit-Remaining header: %w", err)
			}
			reset, err := strconv.Atoi(res.Header.Get("X-RateLimit-Reset"))
			if err != nil {
				return 0, 0, errors.Default.New("failed to parse X-RateLimit-Reset header: %w", err)
			}
			date, err := http.ParseTime(res.Header.Get("Date"))
			if err != nil {
				return 0, 0, errors.Default.New("failed to parse Date header: %w", err)
			}
			return remaining * len(tokens), time.Unix(int64(reset), 0).Sub(date), nil
			*/
			var rateLimit int
			headerRateLimit := res.Header.Get("X-RateLimit-Limit")
			if len(headerRateLimit) > 0 {
				var e error
				rateLimit, e = strconv.Atoi(headerRateLimit)
				if e != nil {
					return 0, 0, errors.Default.Wrap(err, "failed to parse X-RateLimit-Limit header")
				}
			} else {
				// if we can't find "X-RateLimit-Limit" in header, we will return globalRatelimit in ApiRateLimitCalculator.Calculate
				return 0, 0, nil
			}
			// even though different token could have different rate limit, but it is hard to support it
			// so, we calculate the rate limit of a single token, and presume all tokens are the same, to
			// simplify the algorithm for now
			// TODO: consider different token has different rate-limit
			return rateLimit * connection.GetTokensCount(), 1 * time.Hour, nil
		},
	}
	asyncApiClient, err := api.CreateAsyncApiClient(
		taskCtx,
		apiClient,
		rateLimiter,
	)
	if err != nil {
		return nil, err
	}
	return asyncApiClient, nil
}
