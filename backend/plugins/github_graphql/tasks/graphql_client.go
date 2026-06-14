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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/merico-ai/graphql"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func CreateGraphqlClient(
	taskCtx plugin.TaskContext,
	connection *models.GithubConnection,
	httpClient *http.Client,
	getRateRemaining func(context.Context, *graphql.Client, log.Logger) (rateRemaining int, resetAt *time.Time, err errors.Error),
) (*helper.GraphqlAsyncClient, errors.Error) {
	// Build endpoint
	endpoint, err := errors.Convert01(url.Parse(connection.Endpoint))
	if err != nil {
		return nil, errors.BadInput.Wrap(err, fmt.Sprintf("malformed connection endpoint supplied: %s", connection.Endpoint))
	}
	// github.com and github enterprise have different graphql endpoints
	if endpoint.Hostname() == "api.github.com" {
		// see https://docs.github.com/en/graphql/guides/forming-calls-with-graphql
		endpoint.Path = "/graphql"
	} else {
		// see https://docs.github.com/en/enterprise-server@3.11/graphql/guides/forming-calls-with-graphql
		endpoint.Path = "/api/graphql"
	}

	gqlClient := graphql.NewClient(endpoint.String(), httpClient)

	return helper.CreateAsyncGraphqlClient(
		taskCtx,
		gqlClient,
		taskCtx.GetLogger(),
		getRateRemaining,
		// GitHub GraphQL default fallback aligns with GitHub's standard rate limit (~5000)
		helper.WithFallbackRateLimit(5000),
	)
}
