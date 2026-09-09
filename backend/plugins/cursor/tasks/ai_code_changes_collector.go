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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

const rawAiCodeChangesTable = "cursor_ai_code_changes"

// CollectAiCodeChanges collects granular accepted AI changes from
// GET /analytics/ai-code/changes (Enterprise only).
func CollectAiCodeChanges(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if data.Connection.KeyTier != models.KeyTierEnterprise {
		taskCtx.GetLogger().Info("Skipping AI code change collection: key tier is %q, not enterprise", data.Connection.KeyTier)
		return nil
	}

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), data.Connection)
	if err != nil {
		return err
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Table:   rawAiCodeChangesTable,
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
		UrlTemplate: "analytics/ai-code/changes",
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
			return parseAiCodeTrackingTotalPages(body, args.PageSize)
		},
		ResponseParser: parseAiCodeTrackingItemsResponse,
	})
	if err != nil {
		return err
	}

	logUsageCollectionWindow(taskCtx.GetLogger(), "analytics/ai-code/changes", collector.GetSince(), collector.IsIncremental())
	return collector.Execute()
}
