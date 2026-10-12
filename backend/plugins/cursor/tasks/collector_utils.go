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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

const (
	rawUsageEventsTable = "cursor_usage_events"
	rawUserSpendTable   = "cursor_user_spend"
	rawMembersTable     = "cursor_members"
	rawDailyUsageTable  = "cursor_daily_usage"

	cursorApiPageSize         = 100
	cursorInitialBackfillDays = 90
	// cursorLookbackDays rewinds incremental collection so recent calendar days are
	// re-fetched (partial API pages / lag). Mirrors gh-copilot reportLookbackDays.
	cursorLookbackDays = 7
	// cursorDailyUsageMaxDays is the maximum span allowed by POST /teams/daily-usage-data.
	cursorDailyUsageMaxDays = 30
)

type cursorRawParams struct {
	ConnectionId uint64
	ScopeId      string
	Endpoint     string
}

func (p cursorRawParams) GetParams() any {
	return p
}

func rawParamsFromTaskData(data *CursorTaskData) cursorRawParams {
	endpoint := models.DefaultEndpoint
	if data.Connection != nil {
		data.Connection.Normalize()
		endpoint = data.Connection.Endpoint
	}
	return cursorRawParams{
		ConnectionId: data.Options.ConnectionId,
		ScopeId:      data.Options.ScopeId,
		Endpoint:     endpoint,
	}
}

type cursorTimeRangeInput struct {
	StartDateMs int64 `json:"startDateMs"`
	EndDateMs   int64 `json:"endDateMs"`
}

type cursorPageState struct {
	Page int `json:"page"`
}

type cursorPagination struct {
	NumPages        int  `json:"numPages"`
	CurrentPage     int  `json:"currentPage"`
	PageSize        int  `json:"pageSize"`
	HasNextPage     bool `json:"hasNextPage"`
	HasPreviousPage bool `json:"hasPreviousPage"`
	Page            int  `json:"page"`
	TotalPages      int  `json:"totalPages"`
}

// computeUsageTimeRangeMs returns the [start, end] epoch-ms window for usage
// collectors. Full sync uses up to cursorInitialBackfillDays. Incremental runs
// start at `since` (typically LatestSuccessStart) then rewind by
// cursorLookbackDays so recently-missed or partial calendar days are retried.
func computeUsageTimeRangeMs(since *time.Time, now time.Time, isIncremental bool) (int64, int64) {
	end := now.UTC()
	minStart := end.AddDate(0, 0, -cursorInitialBackfillDays)
	start := minStart
	if since != nil && !since.IsZero() && since.After(start) {
		start = since.UTC()
	}
	if isIncremental {
		lookback := end.AddDate(0, 0, -cursorLookbackDays)
		if start.After(lookback) {
			start = lookback
		}
		if start.Before(minStart) {
			start = minStart
		}
	}
	return start.UnixMilli(), end.UnixMilli()
}

// usageCollectionWindow is the computed API request window for usage collectors.
type usageCollectionWindow struct {
	StartMs    int64
	EndMs      int64
	ChunkCount int
}

// usageCollectionWindowFor computes the collection window and 30-day chunk count.
func usageCollectionWindowFor(since *time.Time, now time.Time, isIncremental bool) usageCollectionWindow {
	startMs, endMs := computeUsageTimeRangeMs(since, now, isIncremental)
	chunks := splitDailyUsageTimeRangeMs(startMs, endMs, cursorDailyUsageMaxDays)
	return usageCollectionWindow{
		StartMs:    startMs,
		EndMs:      endMs,
		ChunkCount: len(chunks),
	}
}

// logUsageCollectionWindow emits one Info line describing the collection window.
func logUsageCollectionWindow(logger log.Logger, endpoint string, since *time.Time, isIncremental bool) {
	if logger == nil {
		return
	}
	window := usageCollectionWindowFor(since, time.Now().UTC(), isIncremental)
	sinceStr := "nil"
	if since != nil {
		sinceStr = since.UTC().Format(time.RFC3339)
	}
	logger.Info(
		"cursor usage collection window: endpoint=%s incremental=%v since=%s start=%s end=%s lookbackDays=%d chunkCount=%d",
		endpoint,
		isIncremental,
		sinceStr,
		time.UnixMilli(window.StartMs).UTC().Format(time.RFC3339),
		time.UnixMilli(window.EndMs).UTC().Format(time.RFC3339),
		cursorLookbackDays,
		window.ChunkCount,
	)
}

// splitDailyUsageTimeRangeMs splits [startMs, endMs] into chunks of at most maxDays.
// Used by daily-usage-data (30-day API limit) and filtered-usage-events collectors.
func splitDailyUsageTimeRangeMs(startMs, endMs int64, maxDays int) []cursorTimeRangeInput {
	if startMs >= endMs || maxDays <= 0 {
		return nil
	}
	maxDuration := time.Duration(maxDays) * 24 * time.Hour
	chunkStart := time.UnixMilli(startMs).UTC()
	end := time.UnixMilli(endMs).UTC()

	var chunks []cursorTimeRangeInput
	for chunkStart.Before(end) {
		chunkEnd := chunkStart.Add(maxDuration)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		chunks = append(chunks, cursorTimeRangeInput{
			StartDateMs: chunkStart.UnixMilli(),
			EndDateMs:   chunkEnd.UnixMilli(),
		})
		if !chunkEnd.Before(end) {
			break
		}
		chunkStart = chunkEnd.Add(time.Millisecond)
	}
	return chunks
}

func parseUsageEventsResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	body, err := readResponseBody(res)
	if err != nil {
		return nil, err
	}
	var response struct {
		UsageEvents []json.RawMessage `json:"usageEvents"`
	}
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return nil, errors.Default.Wrap(errors.Convert(jsonErr), "failed to decode usage events response")
	}
	return response.UsageEvents, nil
}

func parseDailyUsageResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	body, err := readResponseBody(res)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return nil, errors.Default.Wrap(errors.Convert(jsonErr), "failed to decode daily usage response")
	}
	return response.Data, nil
}

type aiCodeTrackingListResponse struct {
	Items      []json.RawMessage `json:"items"`
	Data       []json.RawMessage `json:"data"`
	TotalCount int               `json:"totalCount"`
	PageSize   int               `json:"pageSize"`
	TotalPages int               `json:"totalPages"`
}

// parseAiCodeTrackingItemsResponse decodes AI Code Tracking list payloads.
// Cursor docs use "items"; some responses may still expose "data".
func parseAiCodeTrackingItemsResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	body, err := readResponseBody(res)
	if err != nil {
		return nil, err
	}
	return parseAiCodeTrackingItemsBody(body)
}

func parseAiCodeTrackingItemsBody(body []byte) ([]json.RawMessage, errors.Error) {
	var response aiCodeTrackingListResponse
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return nil, errors.Default.Wrap(errors.Convert(jsonErr), "failed to decode ai-code tracking response")
	}
	if len(response.Items) > 0 {
		return response.Items, nil
	}
	return response.Data, nil
}

func parseAiCodeTrackingTotalPages(body []byte, defaultPageSize int) (int, errors.Error) {
	var response aiCodeTrackingListResponse
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return 0, errors.Default.Wrap(errors.Convert(jsonErr), "failed to decode ai-code tracking pagination")
	}
	if response.TotalPages > 0 {
		return response.TotalPages, nil
	}
	if response.TotalCount <= 0 {
		return 1, nil
	}
	pageSize := response.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize <= 0 {
		pageSize = 1
	}
	totalPages := (response.TotalCount + pageSize - 1) / pageSize
	if totalPages <= 0 {
		return 1, nil
	}
	return totalPages, nil
}

func parseMembersResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	body, err := readResponseBody(res)
	if err != nil {
		return nil, err
	}
	var response struct {
		TeamMembers []json.RawMessage `json:"teamMembers"`
	}
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return nil, errors.Default.Wrap(errors.Convert(jsonErr), "failed to decode members response")
	}
	return response.TeamMembers, nil
}

func parsePaginationBody(body []byte) (cursorPagination, errors.Error) {
	var response struct {
		Pagination cursorPagination `json:"pagination"`
	}
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return cursorPagination{}, errors.Default.Wrap(errors.Convert(jsonErr), "failed to decode pagination")
	}
	return response.Pagination, nil
}

func parsePagination(res *http.Response) (cursorPagination, errors.Error) {
	body, err := readResponseBody(res)
	if err != nil {
		return cursorPagination{}, err
	}
	return parsePaginationBody(body)
}

func readResponseBody(res *http.Response) ([]byte, errors.Error) {
	if res == nil || res.Body == nil {
		return nil, errors.Default.New("response body is nil")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to read response body")
	}
	return body, nil
}

func parseEventTimestampMs(raw string) (time.Time, errors.Error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.BadInput.New("event timestamp is empty")
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, errors.BadInput.Wrap(err, "invalid event timestamp")
	}
	return time.UnixMilli(ms).UTC(), nil
}

func computeEventId(timestamp, userEmail, conversationId, model string, chargedCents, requestsCosts float64) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%v|%v", timestamp, userEmail, conversationId, model, chargedCents, requestsCosts)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func ignoreHTTPStatus404(res *http.Response) errors.Error {
	if res.StatusCode == http.StatusUnauthorized {
		return errors.Unauthorized.New("authentication failed, please check your AccessToken")
	}
	if res.StatusCode == http.StatusNotFound {
		return helper.ErrIgnoreAndContinue
	}
	return nil
}

func normalizeNullableString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "null") {
		return ""
	}
	return value
}

func billingCycleTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
