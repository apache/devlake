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
	"net/http"
	"net/url"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/cursor/models"
)

const (
	rawConversationInsightsTable      = "cursor_conversation_insights"
	conversationInsightsIncludeParam    = "intents,complexity,categories,guidanceLevels,workTypes"
)

// CollectConversationInsights collects aggregate Conversation Insights from
// GET /analytics/team/conversation-insights (Enterprise only).
func CollectConversationInsights(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if data.Connection.KeyTier != models.KeyTierEnterprise {
		taskCtx.GetLogger().Info("Skipping Conversation Insights collection: key tier is %q, not enterprise", data.Connection.KeyTier)
		return nil
	}
	if !data.Connection.HasConversationInsights {
		taskCtx.GetLogger().Info("Skipping Conversation Insights collection: connection lacks conversation-insights access")
		return nil
	}

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), data.Connection)
	if err != nil {
		return err
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Table:   rawConversationInsightsTable,
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
		UrlTemplate: "analytics/team/conversation-insights",
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(analyticsDateRangeInput)
			q := url.Values{}
			q.Set("startDate", input.StartDate)
			q.Set("endDate", input.EndDate)
			q.Set("include", conversationInsightsIncludeParam)
			return q, nil
		},
		PageSize: 1,
		GetTotalPages: func(_ *http.Response, _ *helper.ApiCollectorArgs) (int, errors.Error) {
			return 1, nil
		},
		ResponseParser: parseConversationInsightsResponse,
	})
	if err != nil {
		return err
	}

	logUsageCollectionWindow(taskCtx.GetLogger(), "analytics/team/conversation-insights", collector.GetSince(), collector.IsIncremental())
	return collector.Execute()
}

func parseConversationInsightsResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	body, err := readResponseBody(res)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, nil
	}
	return []json.RawMessage{body}, nil
}
