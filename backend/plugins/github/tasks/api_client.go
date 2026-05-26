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
	"github.com/apache/incubator-devlake/plugins/github/token"
)

func CreateApiClient(taskCtx plugin.TaskContext, connection *models.GithubConnection) (*api.ApiAsyncClient, errors.Error) {
	apiClient, err := api.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}

	logger := taskCtx.GetLogger()
	db := taskCtx.GetDal()
	encryptionSecret := taskCtx.GetConfig(plugin.EncodeKeyEnvStr)

	// Inject TokenProvider for OAuth refresh or GitHub App installation tokens.
	var tp *token.TokenProvider
	if connection.RefreshToken != "" {
		tp = token.NewTokenProvider(connection, db, apiClient.GetClient(), logger, encryptionSecret)
	} else if connection.AuthMethod == models.AppKey && connection.InstallationID != 0 {
		tp = token.NewAppInstallationTokenProvider(connection, db, apiClient.GetClient(), logger, encryptionSecret)
	}
	if tp != nil {
		// Wrap the transport
		baseTransport := apiClient.GetClient().Transport
		if baseTransport == nil {
			baseTransport = http.DefaultTransport
		}

		rt := token.NewRefreshRoundTripper(baseTransport, tp)
		apiClient.GetClient().Transport = rt
		logger.Info("Installed token refresh round tripper for connection %d (authMethod=%s)",
			connection.ID, connection.AuthMethod)
	}

	// Persist the freshly minted token so the DB has a correctly encrypted value.
	// PrepareApiClient (called by NewApiClientFromConnection) mints the token
	// in-memory but does not persist it; without this, the DB may contain a stale
	// or corrupted token that breaks GET /connections.
	if connection.AuthMethod == models.AppKey && connection.Token != "" {
		if err := token.PersistEncryptedTokenColumns(db, connection, encryptionSecret, logger, false); err != nil {
			logger.Warn(err, "Failed to persist initial token for connection %d", connection.ID)
		} else {
			logger.Info("Persisted initial token for connection %d", connection.ID)
		}
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
