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
	gocontext "context"
	"net/http"
	"net/url"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
	"github.com/merico-ai/graphql"
)

// linearTransport injects the Linear personal API key into every request.
// Linear expects the key verbatim in the Authorization header (no Bearer prefix).
type linearTransport struct {
	token string
	base  http.RoundTripper
}

func (t *linearTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", t.token)
	return t.base.RoundTrip(req)
}

// graphqlQueryViewer is a tiny probe used to validate connectivity / liveness.
type graphqlQueryViewer struct {
	Viewer struct {
		Id graphql.String
	}
}

// defaultRateLimitPerHour is Linear's documented per-API-key request budget.
// Used when the connection does not override RateLimitPerHour.
const defaultRateLimitPerHour = 1500

// NewLinearGraphqlClient builds a rate-limited async GraphQL client for the
// Linear API from the given connection.
func NewLinearGraphqlClient(taskCtx plugin.TaskContext, connection *models.LinearConnection) (*helper.GraphqlAsyncClient, errors.Error) {
	httpClient, err := newLinearHttpClient(connection)
	if err != nil {
		return nil, err
	}

	endpoint := connection.Endpoint
	if endpoint == "" {
		endpoint = "https://api.linear.app/graphql"
	}
	client := graphql.NewClient(endpoint, httpClient)

	rateLimitPerHour := connection.RateLimitPerHour
	if rateLimitPerHour <= 0 {
		rateLimitPerHour = defaultRateLimitPerHour
	}

	return helper.CreateAsyncGraphqlClient(taskCtx, client, taskCtx.GetLogger(),
		func(ctx gocontext.Context, c *graphql.Client, logger log.Logger) (rateRemaining int, resetAt *time.Time, e errors.Error) {
			// Linear does not expose rate-limit info in the GraphQL body (it uses
			// HTTP response headers), so we probe liveness and pace against the
			// configured hourly budget. The async client self-throttles from here.
			var q graphqlQueryViewer
			dataErrors, queryErr := errors.Convert01(c.Query(ctx, &q, nil))
			if queryErr != nil {
				return 0, nil, queryErr
			}
			if len(dataErrors) > 0 {
				return 0, nil, errors.Default.Wrap(dataErrors[0], "linear graphql viewer query failed")
			}
			reset := time.Now().Add(1 * time.Hour)
			logger.Info("linear graphql client initialized, pacing against %d req/hour", rateLimitPerHour)
			return rateLimitPerHour, &reset, nil
		})
}

func newLinearHttpClient(connection *models.LinearConnection) (*http.Client, errors.Error) {
	base := http.DefaultTransport
	if proxy := connection.Proxy; proxy != "" {
		pu, err := url.Parse(proxy)
		if err != nil {
			return nil, errors.BadInput.Wrap(err, "malformed proxy url")
		}
		base = &http.Transport{Proxy: http.ProxyURL(pu)}
	}
	return &http.Client{
		Timeout:   60 * time.Second,
		Transport: &linearTransport{token: connection.Token, base: base},
	}, nil
}
