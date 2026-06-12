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
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const rawUserMetricsTable = "copilot_user_metrics"

func collectUserMetricsRecords(meta *reportMetadataResponse, logger log.Logger) ([]json.RawMessage, errors.Error) {
	var results []json.RawMessage
	for _, link := range meta.DownloadLinks {
		reportBody, dlErr := downloadReport(link, logger)
		if dlErr != nil {
			return nil, dlErr
		}
		if reportBody == nil {
			continue // blob not found, skip
		}
		// Parse JSONL: split by newlines and return each non-empty line.
		userRecords, parseErr := parseJSONL(reportBody)
		if parseErr != nil {
			return nil, parseErr
		}
		results = append(results, userRecords...)
	}
	return results, nil
}

func parseUserMetricsReportResponse(res *http.Response, logger log.Logger) ([]json.RawMessage, errors.Error) {
	body, readErr := io.ReadAll(res.Body)
	res.Body.Close()
	if readErr != nil {
		return nil, errors.Default.Wrap(readErr, "failed to read report metadata")
	}
	if isEmptyReport(body) {
		return nil, nil
	}

	var meta *reportMetadataResponse
	if jsonErr := json.Unmarshal(body, &meta); jsonErr != nil {
		return nil, errors.Default.Wrap(jsonErr, "failed to parse report metadata")
	}

	meta, err := parseReportMetadataResponse(res, logger)
	if err != nil || meta == nil {
		return nil, err
	}

	return collectUserMetricsRecords(meta, logger)
}

// CollectUserMetrics collects enterprise user-level daily Copilot usage reports.
// These reports are in JSONL format (one JSON object per line per user).
// Utilizes the enterprise or organization endpoints depending on connection configuration
func CollectUserMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), connection)
	if err != nil {
		return err
	}

	var urlTemplate string

	if connection.HasEnterprise() {
		urlTemplate = fmt.Sprintf("enterprises/%s/copilot/metrics/reports/users-1-day", connection.Enterprise)
	} else if connection.Organization != "" {
		urlTemplate = fmt.Sprintf("orgs/%s/copilot/metrics/reports/users-1-day", connection.Organization)
	} else {
		return nil
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:   taskCtx,
		Table: rawUserMetricsTable,
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
	start = clampDailyMetricsStartForBackfill(start, until)
	logger := taskCtx.GetLogger()

	dayIter := newDayIterator(start, until)

	err = collector.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   apiClient,
		Input:       dayIter,
		UrlTemplate: urlTemplate,
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
			return parseUserMetricsReportResponse(res, logger)
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
