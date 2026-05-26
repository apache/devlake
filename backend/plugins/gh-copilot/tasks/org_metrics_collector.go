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
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const rawOrgMetricsTable = "copilot_org_metrics"

// CollectOrgMetrics collects organization-level daily Copilot usage reports
// using the new report download API. Replaces the deprecated /orgs/{org}/copilot/metrics endpoint.
func CollectOrgMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	if connection.Organization == "" {
		taskCtx.GetLogger().Info("No organization configured, skipping org metrics collection")
		return nil
	}

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), connection)
	if err != nil {
		return err
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:   taskCtx,
		Table: rawOrgMetricsTable,
		Options: copilotRawParams{
			ConnectionId: data.Options.ConnectionId,
			ScopeId:      data.Options.ScopeId,
			Organization: connection.Organization,
			Endpoint:     connection.Endpoint,
		},
	}

	collector, err := helper.NewStatefulApiCollector(rawArgs)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	start, until := computeReportDateRange(now, collector.GetSince())
	logger := taskCtx.GetLogger()

	dayIter := newDayIterator(start, until)

	err = collector.InitCollector(helper.ApiCollectorArgs{
		ApiClient: apiClient,
		Input:     dayIter,
		UrlTemplate: fmt.Sprintf("orgs/%s/copilot/metrics/reports/organization-1-day",
			connection.Organization),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(*dayInput)
			q := url.Values{}
			q.Set("day", input.Day)
			return q, nil
		},
		Incremental:   true,
		Concurrency:   1,
		AfterResponse: ignoreNoContent,
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			body, readErr := io.ReadAll(res.Body)
			res.Body.Close()
			if readErr != nil {
				return nil, errors.Default.Wrap(readErr, "failed to read report metadata")
			}
			if isEmptyReport(body) {
				return nil, nil
			}

			var meta reportMetadataResponse
			if jsonErr := json.Unmarshal(body, &meta); jsonErr != nil {
				snippet := string(body)
				if len(snippet) > 200 {
					snippet = snippet[:200]
				}
				logger.Error(jsonErr, "failed to parse report metadata, body=%s", snippet)
				return nil, errors.Default.Wrap(jsonErr, "failed to parse report metadata")
			}

			if len(meta.DownloadLinks) == 0 {
				logger.Info("No download links for report day=%s, skipping", meta.ReportDay)
				return nil, nil
			}

			var results []json.RawMessage
			for _, link := range meta.DownloadLinks {
				reportBody, dlErr := downloadReport(link, logger)
				if dlErr != nil {
					return nil, dlErr
				}
				if reportBody == nil {
					continue // blob not found, skip
				}
				results = append(results, json.RawMessage(reportBody))
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
