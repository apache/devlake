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

const rawUserTeamsTable = "copilot_user_teams"

// CollectUserTeams collects user-team mapping data from the user-teams-1-day report.
// This enables team-level metrics aggregation by joining with per-user daily metrics.
func CollectUserTeams(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	var urlTemplate string

	if connection.HasEnterprise() {
		urlTemplate = fmt.Sprintf("enterprises/%s/copilot/metrics/reports/user-teams-1-day", connection.Enterprise)
	} else if connection.Organization != "" {
		urlTemplate = fmt.Sprintf("orgs/%s/copilot/metrics/reports/user-teams-1-day", connection.Organization)
	} else {
		return nil
	}

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), connection)
	if err != nil {
		return err
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:   taskCtx,
		Table: rawUserTeamsTable,
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
				return nil, errors.Default.Wrap(jsonErr, "failed to parse report metadata")
			}

			var results []json.RawMessage
			for _, link := range meta.DownloadLinks {
				reportBody, dlErr := downloadReport(link, logger)
				if dlErr != nil {
					return nil, dlErr
				}
				if reportBody == nil {
					continue
				}
				// User-teams reports are JSONL format
				records, parseErr := parseJSONL(reportBody)
				if parseErr != nil {
					return nil, parseErr
				}
				results = append(results, records...)
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
