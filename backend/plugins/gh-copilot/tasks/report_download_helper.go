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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// reportMaxDays is the maximum historical window the new report API supports (1 year).
const reportMaxDays = 365

// copilotRawParams identifies a set of raw data records for a given connection/scope.
type copilotRawParams struct {
	ConnectionId uint64
	ScopeId      string
	Organization string
	Endpoint     string
}

func (p copilotRawParams) GetParams() any {
	return p
}

func utcDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ignoreNoContent is an AfterResponse callback that skips 404 and 204 responses.
// The report API returns 404 when no report is available for a given day,
// and 204 (No Content) when data is not yet available or the org had fewer
// than 5 active Copilot users on that day. Both are normal and should not
// be treated as errors.
func ignoreNoContent(res *http.Response) errors.Error {
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusNoContent {
		return helper.ErrIgnoreAndContinue
	}
	return nil
}

// isEmptyReport returns true when the GitHub API returned an HTTP 200 but the
// body carries no usable report data.  For dates before Copilot usage data was
// available the API responds with "" (empty JSON string) instead of a 404.
func isEmptyReport(body []byte) bool {
	b := bytes.TrimSpace(body)
	return len(b) == 0 || string(b) == `""` || string(b) == "null"
}

// reportMetadataResponse represents the JSON returned by the report metadata endpoints.
type reportMetadataResponse struct {
	DownloadLinks []string `json:"download_links"`
	ReportDay     string   `json:"report_day"`
	// 28-day variants use start/end instead of a single day.
	ReportStartDay string `json:"report_start_day"`
	ReportEndDay   string `json:"report_end_day"`
}

// computeReportDateRange returns the range of dates to collect, clamped to the API max.
func computeReportDateRange(now time.Time, since *time.Time) (start, until time.Time) {
	until = utcDate(now).AddDate(0, 0, -1) // reports are available for the previous day
	min := until.AddDate(0, 0, -(reportMaxDays - 1))
	start = min
	if since != nil {
		start = utcDate(*since)
		if start.Before(min) {
			start = min
		}
		if start.After(until) {
			start = until
		}
	}
	return start, until
}

// downloadReport downloads a single report file from a signed URL and returns the raw body.
// Returns nil, nil when the blob is not found (404) — the caller should skip such reports.
func downloadReport(url string, logger log.Logger) ([]byte, errors.Error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to download report file")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		if logger != nil {
			logger.Info("Report blob not found (404), skipping")
		}
		return nil, nil
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Default.New(fmt.Sprintf("report download failed with status %d: %s", resp.StatusCode, string(body)))
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, errors.Default.Wrap(readErr, "failed to read report file body")
	}

	if logger != nil {
		logger.Info("Downloaded report file (%d bytes)", len(body))
	}
	return body, nil
}

// parseJSONL splits a JSONL (JSON Lines) byte slice into individual JSON messages.
// Each non-empty line is treated as a separate JSON object.
func parseJSONL(data []byte) ([]json.RawMessage, errors.Error) {
	var results []json.RawMessage
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		results = append(results, json.RawMessage(line))
	}
	return results, nil
}
