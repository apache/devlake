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
	"net/http"
	"net/url"
	"strconv"

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

// slackCollectorPageSize is the PageSize given to the ApiCollector of the
// cursor-paginated Slack collectors. It is not the number of items requested
// per call: that is the `limit` query parameter (see setSlackCursorQuery).
//
// With GetNextPageCustomData set, ApiCollector stops as soon as a page has
// fewer items than PageSize. Slack can return fewer items than `limit` while
// more remain ("It's possible to receive fewer results than your specified
// limit, even when there are additional results to retrieve",
// https://docs.slack.dev/apis/web-api/pagination), so only an empty
// response_metadata.next_cursor may end the collection. A PageSize of 1 keeps
// ApiCollector from stopping on a short page.
const slackCollectorPageSize = 1

// slackCursorPage is the part of a Slack Web API response used for pagination.
type slackCursorPage struct {
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// getSlackNextCursor returns the cursor of the next page, or ErrFinishCollect
// when response_metadata.next_cursor is empty.
func getSlackNextCursor(_ *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
	res := slackCursorPage{}
	err := api.UnmarshalResponse(prevPageResponse, &res)
	if err != nil {
		return nil, err
	}
	if res.ResponseMetadata.NextCursor == "" {
		return nil, api.ErrFinishCollect
	}
	return res.ResponseMetadata.NextCursor, nil
}

// setSlackCursorQuery sets the page size and, after the first page, the
// cursor returned by getSlackNextCursor.
func setSlackCursorQuery(query url.Values, reqData *api.RequestData, limit int) {
	query.Set("limit", strconv.Itoa(limit))
	if cursor, ok := reqData.CustomData.(string); ok && cursor != "" {
		query.Set("cursor", cursor)
	}
}
