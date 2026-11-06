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
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

const rawAiCodeCommitsTable = "cursor_ai_code_commits"

// CollectAiCodeCommits collects per-commit AI line attribution from
// GET /analytics/ai-code/commits (Enterprise only).
func CollectAiCodeCommits(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if data.Connection.KeyTier != models.KeyTierEnterprise {
		taskCtx.GetLogger().Info("Skipping AI code commit collection: key tier is %q, not enterprise", data.Connection.KeyTier)
		return nil
	}

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), data.Connection)
	if err != nil {
		return err
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Table:   rawAiCodeCommitsTable,
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
		UrlTemplate: "analytics/ai-code/commits",
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

	logUsageCollectionWindow(taskCtx.GetLogger(), "analytics/ai-code/commits", collector.GetSince(), collector.IsIncremental())
	return collector.Execute()
}

type analyticsDateRangeInput struct {
	StartDate string
	EndDate   string
}

func newAnalyticsDateRangeIterator(since *time.Time, isIncremental bool) *helper.QueueIterator {
	startMs, endMs := computeUsageTimeRangeMs(since, time.Now().UTC(), isIncremental)
	iter := helper.NewQueueIterator()
	for _, chunk := range splitAnalyticsDateRange(startMs, endMs, cursorDailyUsageMaxDays) {
		iter.Push(chunk)
	}
	return iter
}

// splitAnalyticsDateRange splits [startMs, endMs] into ISO-date chunks.
func splitAnalyticsDateRange(startMs, endMs int64, maxDays int) []analyticsDateRangeInput {
	if startMs >= endMs || maxDays <= 0 {
		return nil
	}
	maxDuration := time.Duration(maxDays) * 24 * time.Hour
	chunkStart := time.UnixMilli(startMs).UTC()
	end := time.UnixMilli(endMs).UTC()

	var chunks []analyticsDateRangeInput
	for chunkStart.Before(end) {
		chunkEnd := chunkStart.Add(maxDuration)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		chunks = append(chunks, analyticsDateRangeInput{
			StartDate: chunkStart.Format("2006-01-02"),
			EndDate:   chunkEnd.Format("2006-01-02"),
		})
		if !chunkEnd.Before(end) {
			break
		}
		chunkStart = chunkEnd.Add(time.Millisecond)
	}
	return chunks
}
