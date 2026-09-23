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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

// defaultRateLimitPerHour is the connection-level request budget when the
// connection does not override RateLimitPerHour. YouTrack documents
// no rate limits; the observed failure mode is 504 on expensive queries, so
// the default is generous and user-configurable.
const defaultRateLimitPerHour = 10000

// NewYoutrackApiClient builds the rate-limited async REST client for the
// YouTrack API from the given connection.
//
// Retry/timeout policy has exactly ONE owner: the transport
// (retry_transport.go) retries 429 and 5xx (incl. the observed 504) with
// exponential backoff + jitter, honouring Retry-After. The framework async
// client's own loop — any status >= 400, fixed tick, no backoff — is
// explicitly disabled via SetMaxRetry(0): left active it would multiply the
// transport's attempts (3x3 network calls per logical request) and retry
// pointlessly on YouTrack's 400s (bad query/fields params). When
// API_TIMEOUT is unset the framework applies a 120s default.
func NewYoutrackApiClient(taskCtx plugin.TaskContext, connection *models.YoutrackConnection) (*helper.ApiAsyncClient, errors.Error) {
	apiClient, err := helper.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}
	apiClient.GetClient().Transport = newRetryTransport(apiClient.GetClient().Transport)
	rateLimitPerHour := connection.RateLimitPerHour
	if rateLimitPerHour <= 0 {
		rateLimitPerHour = defaultRateLimitPerHour
	}
	asyncClient, err := helper.CreateAsyncApiClient(taskCtx, apiClient, &helper.ApiRateLimitCalculator{
		UserRateLimitPerHour: rateLimitPerHour,
	})
	if err != nil {
		return nil, err
	}
	// single retry owner: the transport (see above)
	asyncClient.SetMaxRetry(0)
	return asyncClient, nil
}

// ignoreHTTPStatus404 is the AfterResponse for per-issue collectors: an issue
// deleted in YouTrack stays in the tool layer (incremental runs never prune
// it), so its comments/activities endpoint answers 404. Skipping that request
// keeps the subtask — and its bookmark — moving instead of failing every run
// until a full resync. 401 keeps the framework's default authentication error.
func ignoreHTTPStatus404(res *http.Response) errors.Error {
	if res.StatusCode == http.StatusUnauthorized {
		return errors.Unauthorized.New("authentication failed, please check your AccessToken")
	}
	if res.StatusCode == http.StatusNotFound {
		return helper.ErrIgnoreAndContinue
	}
	return nil
}
