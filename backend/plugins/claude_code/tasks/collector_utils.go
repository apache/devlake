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
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const (
	rawUserActivityTable    = "claude_code_user_activity"
	rawActivitySummaryTable = "claude_code_activity_summary"
	rawChatProjectTable     = "claude_code_chat_project"
	rawSkillUsageTable      = "claude_code_skill_usage"
	rawConnectorUsageTable  = "claude_code_connector_usage"

	claudeCodeApiPageLimit        = 1000
	claudeCodeInitialBackfillDays = 90
	claudeCodeSummaryMaxDays      = 31
	claudeCodeAvailabilityLagDays = 3
	claudeCodeDateLayout          = "2006-01-02"
)

// claudeCodeRawParams identifies a set of raw data records for a given connection/scope.
type claudeCodeRawParams struct {
	ConnectionId uint64
	ScopeId      string
	Organization string
	Endpoint     string
}

func (p claudeCodeRawParams) GetParams() any {
	return p
}

// claudeCodeUsagePage is the common paginated response envelope used by all analytics endpoints.
type claudeCodeUsagePage struct {
	Data     []json.RawMessage `json:"data"`
	HasMore  bool              `json:"has_more"`
	NextPage *string           `json:"next_page"`
}

// claudeCodeDayInput is the input item for day-based collectors.
type claudeCodeDayInput struct {
	Day string `json:"day"`
}

// claudeCodeDateRangeInput is the input item for the summaries (range-based) collector.
type claudeCodeDateRangeInput struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// utcDate truncates a time to midnight UTC.
func utcDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// computeUsageDateRange returns the [start, until] date range for incremental collection.
// The Anthropic Analytics API only serves data from 2026-01-01 onwards and requires
// dates to be at least three days old, so the current day and previous two days are skipped.
func computeUsageDateRange(now time.Time, since *time.Time) (start, until time.Time) {
	apiFloor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	until = utcDate(now).AddDate(0, 0, -claudeCodeAvailabilityLagDays)
	start = until.AddDate(0, 0, -(claudeCodeInitialBackfillDays - 1))
	if since != nil {
		start = utcDate(*since).AddDate(0, 0, 1)
		minStart := until.AddDate(0, 0, -(claudeCodeInitialBackfillDays - 1))
		if start.Before(minStart) {
			start = minStart
		}
	}
	if start.Before(apiFloor) {
		start = apiFloor
	}
	return start, until
}

// claudeCodeDayIterator iterates over individual calendar days.
type claudeCodeDayIterator struct {
	days []claudeCodeDayInput
	idx  int
}

func newClaudeCodeDayIterator(start, end time.Time) *claudeCodeDayIterator {
	days := make([]claudeCodeDayInput, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		days = append(days, claudeCodeDayInput{Day: d.Format(claudeCodeDateLayout)})
	}
	return &claudeCodeDayIterator{days: days}
}

func (it *claudeCodeDayIterator) HasNext() bool       { return it.idx < len(it.days) }
func (it *claudeCodeDayIterator) Close() errors.Error { return nil }
func (it *claudeCodeDayIterator) Fetch() (interface{}, errors.Error) {
	if it.idx >= len(it.days) {
		return nil, nil
	}
	day := it.days[it.idx]
	it.idx++
	return &day, nil
}

// claudeCodeDateRangeIterator iterates over date ranges in chunks of up to maxDays days.
type claudeCodeDateRangeIterator struct {
	chunks []claudeCodeDateRangeInput
	idx    int
}

func newClaudeCodeDateRangeIterator(start, end time.Time, maxDays int) *claudeCodeDateRangeIterator {
	chunks := make([]claudeCodeDateRangeInput, 0)
	for chunkStart := start; !chunkStart.After(end); {
		chunkEnd := chunkStart.AddDate(0, 0, maxDays-1)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		chunks = append(chunks, claudeCodeDateRangeInput{
			StartDate: chunkStart.Format(claudeCodeDateLayout),
			EndDate:   chunkEnd.AddDate(0, 0, 1).Format(claudeCodeDateLayout), // exclusive end
		})
		chunkStart = chunkEnd.AddDate(0, 0, 1)
	}
	return &claudeCodeDateRangeIterator{chunks: chunks}
}

func (it *claudeCodeDateRangeIterator) HasNext() bool       { return it.idx < len(it.chunks) }
func (it *claudeCodeDateRangeIterator) Close() errors.Error { return nil }
func (it *claudeCodeDateRangeIterator) Fetch() (interface{}, errors.Error) {
	if it.idx >= len(it.chunks) {
		return nil, nil
	}
	chunk := it.chunks[it.idx]
	it.idx++
	return &chunk, nil
}

// parseClaudeCodeUsagePage reads and parses a paginated analytics API response.
func parseClaudeCodeUsagePage(res *http.Response) (*claudeCodeUsagePage, errors.Error) {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to read analytics response")
	}
	res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		snippet := string(body)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		return nil, errors.HttpStatus(res.StatusCode).New(fmt.Sprintf("analytics request failed: %s", snippet))
	}

	var page claudeCodeUsagePage
	if err := errors.Convert(json.Unmarshal(body, &page)); err != nil {
		return nil, errors.Default.Wrap(err, "failed to decode analytics response")
	}

	res.Body = io.NopCloser(strings.NewReader(string(body)))
	return &page, nil
}

// getNextClaudeCodePageCursor returns the next page cursor or ErrFinishCollect.
func getNextClaudeCodePageCursor(_ *helper.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
	page, err := parseClaudeCodeUsagePage(prevPageResponse)
	if err != nil {
		return nil, err
	}
	if !page.HasMore || page.NextPage == nil || strings.TrimSpace(*page.NextPage) == "" {
		return nil, helper.ErrFinishCollect
	}
	return strings.TrimSpace(*page.NextPage), nil
}
