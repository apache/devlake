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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const rawBugbotReviewsTable = "cursor_bugbot_reviews"

// CollectBugbotReviews collects completed BugBot reviews from
// GET /analytics/team/bugbot-reviews.
func CollectBugbotReviews(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if !data.Connection.HasBugbotReviews {
		taskCtx.GetLogger().Info("Skipping BugBot review collection: connection lacks bugbot-reviews access")
		return nil
	}

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), data.Connection)
	if err != nil {
		return err
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Table:   rawBugbotReviewsTable,
		Options: rawParamsFromTaskData(data),
	}

	collector, err := helper.NewStatefulApiCollector(rawArgs)
	if err != nil {
		return err
	}

	dateRangeIter := newAnalyticsDateRangeIterator(collector.GetSince(), collector.IsIncremental())

	err = collector.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   apiClient,
		Input:       dateRangeIter,
		UrlTemplate: "analytics/team/bugbot-reviews",
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(analyticsDateRangeInput)
			q := url.Values{}
			q.Set("startDate", input.StartDate)
			q.Set("endDate", input.EndDate)
			q.Set("page", fmt.Sprintf("%d", reqData.Pager.Page))
			q.Set("pageSize", fmt.Sprintf("%d", reqData.Pager.Size))
			return q, nil
		},
		PageSize: cursorApiPageSize,
		GetTotalPages: func(res *http.Response, args *helper.ApiCollectorArgs) (int, errors.Error) {
			body, readErr := readResponseBody(res)
			if readErr != nil {
				return 0, readErr
			}
			var pagination struct {
				Pagination struct {
					TotalPages int `json:"totalPages"`
				} `json:"pagination"`
			}
			if jsonErr := json.Unmarshal(body, &pagination); jsonErr != nil {
				return 0, errors.Default.Wrap(errors.Convert(jsonErr), "failed to parse bugbot-reviews pagination")
			}
			if pagination.Pagination.TotalPages <= 0 {
				return 1, nil
			}
			return pagination.Pagination.TotalPages, nil
		},
		ResponseParser: parseAnalyticsDataArrayResponse,
	})
	if err != nil {
		return err
	}

	logUsageCollectionWindow(taskCtx.GetLogger(), "analytics/team/bugbot-reviews", collector.GetSince(), collector.IsIncremental())
	return collector.Execute()
}

func parseAnalyticsDataArrayResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	body, err := readResponseBody(res)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return nil, errors.Default.Wrap(errors.Convert(jsonErr), "failed to decode analytics response")
	}
	return response.Data, nil
}
