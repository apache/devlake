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
	"fmt"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/slack/models"
)

func NewSlackApiClient(taskCtx plugin.TaskContext, connection *models.SlackConnection) (*api.ApiAsyncClient, errors.Error) {
	apiClient, err := api.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}

	// create async api client
	asyncApiClient, err := api.CreateAsyncApiClient(taskCtx, apiClient, &api.ApiRateLimitCalculator{
		UserRateLimitPerHour: connection.RateLimitPerHour,
	})
	if err != nil {
		return nil, err
	}

	return asyncApiClient, nil
}

// checkSlackApiResult turns a failed Slack Web API call into an error. Slack
// answers most failed calls (not_in_channel, channel_not_found, missing_scope,
// invalid_auth, ...) with HTTP 200 and a body like {"ok": false, "error": "..."},
// so the HTTP status alone does not tell whether the call worked.
func checkSlackApiResult(method string, ok bool, errorCode string) errors.Error {
	if ok {
		return nil
	}
	if errorCode == "" {
		errorCode = "unknown error"
	}
	return errors.Default.New(fmt.Sprintf("slack %s error: %s", method, errorCode))
}
