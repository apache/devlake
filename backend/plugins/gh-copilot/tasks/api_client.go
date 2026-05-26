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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/gh-copilot/models"
)

const gitHubApiVersion = "2026-03-10"

type nowFunc func() time.Time
type sleepFunc func(time.Duration)

func handleGitHubRetryAfter(res *http.Response, logger log.Logger, now nowFunc, sleep sleepFunc) errors.Error {
	if res == nil {
		return nil
	}
	if res.StatusCode != http.StatusTooManyRequests {
		return nil
	}

	if now == nil {
		now = time.Now
	}
	if sleep == nil {
		sleep = time.Sleep
	}

	wait := parseRetryAfter(res.Header.Get("Retry-After"), now().UTC())
	if wait > 0 {
		if logger != nil {
			logger.Warn(nil, "GitHub returned 429; sleeping %s per Retry-After", wait.String())
		}
		sleep(wait)
	}
	// Return an error so the async client will retry.
	return errors.HttpStatus(http.StatusTooManyRequests).New("GitHub rate limited the request")
}

func CreateApiClient(taskCtx plugin.TaskContext, connection *models.GhCopilotConnection) (*helper.ApiAsyncClient, errors.Error) {
	apiClient, err := helper.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}

	apiClient.SetHeaders(map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": gitHubApiVersion,
	})

	rateLimiter := &helper.ApiRateLimitCalculator{UserRateLimitPerHour: connection.RateLimitPerHour}
	asyncClient, err := helper.CreateAsyncApiClient(taskCtx, apiClient, rateLimiter)
	if err != nil {
		return nil, err
	}

	// Ensure we respect GitHub Retry-After on 429s before retrying.
	apiClient.SetAfterFunction(func(res *http.Response) errors.Error {
		return handleGitHubRetryAfter(res, taskCtx.GetLogger(), time.Now, time.Sleep)
	})

	return asyncClient, nil
}
