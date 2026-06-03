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
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// CollectConnectorUsage collects per-connector daily usage from /v1/organizations/analytics/connectors.
func CollectConnectorUsage(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*ClaudeCodeTaskData)
	if !ok {
		return errors.Default.New("task data is not ClaudeCodeTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	if strings.TrimSpace(connection.Organization) == "" {
		taskCtx.GetLogger().Info("No organization configured, skipping connector usage collection")
		return nil
	}

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), connection)
	if err != nil {
		return err
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:   taskCtx,
		Table: rawConnectorUsageTable,
		Options: claudeCodeRawParams{
			ConnectionId: data.Options.ConnectionId,
			ScopeId:      data.Options.ScopeId,
			Organization: connection.Organization,
			Endpoint:     "analytics/connectors",
		},
	}

	collector, err := helper.NewStatefulApiCollector(rawArgs)
	if err != nil {
		return err
	}

	start, until := computeUsageDateRange(time.Now().UTC(), collector.GetSince())
	dayIter := newClaudeCodeDayIterator(start, until)

	err = collector.InitCollector(helper.ApiCollectorArgs{
		ApiClient:             apiClient,
		Input:                 dayIter,
		PageSize:              1,
		Incremental:           true,
		UrlTemplate:           "v1/organizations/analytics/connectors",
		GetNextPageCustomData: getNextClaudeCodePageCursor,
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(*claudeCodeDayInput)
			query := url.Values{}
			query.Set("date", input.Day)
			query.Set("limit", fmt.Sprintf("%d", claudeCodeApiPageLimit))
			if cursor, ok := reqData.CustomData.(string); ok && strings.TrimSpace(cursor) != "" {
				query.Set("page", cursor)
			}
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			page, err := parseClaudeCodeUsagePage(res)
			if err != nil {
				return nil, err
			}
			return page.Data, nil
		},
	})
	if err != nil {
		return err
	}

	return collector.Execute()
}
