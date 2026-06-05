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

// reportLookbackDays: extra days rewound from 'until' on incremental runs.
// GitHub reports are generated hours after midnight, so a midnight run gets 404 for the previous
// day. Without this buffer, 'LatestSuccessStart' advances past the missed day permanently.
const reportLookbackDays = 2

// dailyMetricsTrailingBackfillDays extends retries for delayed daily report generation.
const dailyMetricsTrailingBackfillDays = 4

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

func clampDailyMetricsStartForBackfill(start, until time.Time) time.Time {
	trailingStart := until.AddDate(0, 0, -(dailyMetricsTrailingBackfillDays - 1))
	if start.After(trailingStart) {
		return trailingStart
	}
	return start
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

func readReportMetadataBody(res *http.Response) ([]byte, errors.Error) {
	body, readErr := io.ReadAll(res.Body)
	res.Body.Close()
	if readErr != nil {
		return nil, errors.Default.Wrap(readErr, "failed to read report metadata")
	}
	return body, nil
}

func logReportMetadataParseError(body []byte, err error, logger log.Logger) {
	if logger == nil {
		return
	}
	snippet := string(body)
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}
	logger.Error(err, "failed to parse report metadata, body=%s", snippet)
}

func reportMetadataRange(meta reportMetadataResponse) string {
	if meta.ReportDay != "" {
		return meta.ReportDay
	}
	if meta.ReportStartDay != "" && meta.ReportEndDay != "" {
		return fmt.Sprintf("%s..%s", meta.ReportStartDay, meta.ReportEndDay)
	}
	return ""
}

func logMissingDownloadLinks(meta reportMetadataResponse, logger log.Logger) {
	if logger == nil || len(meta.DownloadLinks) != 0 {
		return
	}
	reportRange := reportMetadataRange(meta)
	if reportRange != "" {
		logger.Info("No download links for report day=%s, skipping", reportRange)
		return
	}
	logger.Info("No download links in report metadata, skipping")
}

func parseReportMetadata(body []byte, logger log.Logger) (*reportMetadataResponse, errors.Error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		if logger != nil {
			logger.Info("Report metadata response was empty, skipping")
		}
		return nil, nil
	}

	// Handle JSON-encoded empty string ""
	if bytes.Equal(trimmed, []byte(`""`)) {
		if logger != nil {
			logger.Info("Report metadata response was empty string, skipping")
		}
		return nil, nil
	}

	var meta reportMetadataResponse
	if jsonErr := json.Unmarshal(trimmed, &meta); jsonErr != nil {
		logReportMetadataParseError(trimmed, jsonErr, logger)
		return nil, errors.Default.Wrap(jsonErr, "failed to parse report metadata")
	}

	logMissingDownloadLinks(meta, logger)

	return &meta, nil
}

func parseReportMetadataResponse(res *http.Response, logger log.Logger) (*reportMetadataResponse, errors.Error) {
	if res.StatusCode == http.StatusNoContent {
		if logger != nil {
			logger.Info("Report metadata not ready yet (204), skipping for now")
		}
		res.Body.Close()
		return nil, nil
	}

	body, readErr := readReportMetadataBody(res)
	if readErr != nil {
		return nil, readErr
	}

	return parseReportMetadata(body, logger)
}

func collectRawReportRecords(meta *reportMetadataResponse, logger log.Logger) ([]json.RawMessage, errors.Error) {
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
			continue
		}
		results = append(results, json.RawMessage(reportBody))
	}
	return results, nil
}

func parseRawReportResponse(res *http.Response, logger log.Logger) ([]json.RawMessage, errors.Error) {
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
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		logger.Error(jsonErr, "failed to parse report metadata, body=%s", snippet)
		return nil, errors.Default.Wrap(jsonErr, "failed to parse report metadata")
	}

	meta, err := parseReportMetadataResponse(res, logger)
	if err != nil || meta == nil {
		return nil, err
	}

	return collectRawReportRecords(meta, logger)
}

// computeReportDateRange returns the range of dates to collect, clamped to the API max.
// When 'since' is set, 'start' is rewound to at least 'until - reportLookbackDays'
// so days that returned 404 (report not yet generated) are retried on subsequent runs.
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
		// Rewind 'start' by 'reportLookbackDays' so recently-missed days are retried.
		if lookback := until.AddDate(0, 0, -reportLookbackDays); start.After(lookback) {
			start = lookback
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
