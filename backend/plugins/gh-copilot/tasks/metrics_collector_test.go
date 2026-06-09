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
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseRetryAfterSeconds(t *testing.T) {
	wait := parseRetryAfter("10", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	require.Equal(t, 10*time.Second, wait)
}

func TestParseRetryAfterHttpDate(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	value := now.Add(5 * time.Second).Format(http.TimeFormat)
	wait := parseRetryAfter(value, now)
	require.True(t, wait >= 4*time.Second && wait <= 6*time.Second)
}

func TestComputeReportDateRangeDefaultLookback(t *testing.T) {
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	start, until := computeReportDateRange(now, nil)
	require.Equal(t, time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC), until)
	require.Equal(t, time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC), start)
}

func TestComputeReportDateRangeUsesSince(t *testing.T) {
	// since is far enough in the past that the lookback buffer doesn't apply.
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	since := time.Date(2025, 1, 3, 12, 0, 0, 0, time.UTC)
	start, until := computeReportDateRange(now, &since)
	require.Equal(t, time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC), until)
	require.Equal(t, time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC), start)
}

func TestComputeReportDateRangeClampsToLookback(t *testing.T) {
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	since := time.Date(2024, 6, 24, 12, 0, 0, 0, time.UTC)
	start, until := computeReportDateRange(now, &since)
	require.Equal(t, time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC), until)
	require.Equal(t, time.Date(2024, 6, 24, 0, 0, 0, 0, time.UTC), start)
}

func TestComputeReportDateRangeClampsFutureSince(t *testing.T) {
	// Future since is clamped to until, then the lookback buffer applies.
	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	since := now.Add(24 * time.Hour)
	start, until := computeReportDateRange(now, &since)
	require.Equal(t, time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC), until)
	require.Equal(t, time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC), start)
}

func TestComputeReportDateRangeLookbackBuffer(t *testing.T) {
	// since is yesterday: without the buffer we'd only request 1 day (yesterday).
	// With the buffer we look back reportLookbackDays days to retry any 404'd days.
	now := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)  // midnight run
	since := time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC) // LatestSuccessStart from previous midnight run
	start, until := computeReportDateRange(now, &since)
	require.Equal(t, time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC), until)
	require.Equal(t, time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC), start)
}

func TestClampDailyMetricsStartForBackfillRecentStart(t *testing.T) {
	until := time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC)
	start := time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC)

	clamped := clampDailyMetricsStartForBackfill(start, until)
	require.Equal(t, time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC), clamped)
}

func TestClampDailyMetricsStartForBackfillKeepsOlderStart(t *testing.T) {
	until := time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC)
	start := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)

	clamped := clampDailyMetricsStartForBackfill(start, until)
	require.Equal(t, start, clamped)
}

func TestUserMetricsDateRangeAppliesFourDayBackfillWindow(t *testing.T) {
	now := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	since := time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC)

	start, until := computeReportDateRange(now, &since)
	start = clampDailyMetricsStartForBackfill(start, until)

	require.Equal(t, time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC), until)
	require.Equal(t, time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC), start)
}

func TestParseReportMetadataResponseNoContent(t *testing.T) {
	res := &http.Response{
		StatusCode: http.StatusNoContent,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}

	meta, err := parseReportMetadataResponse(res, nil)
	require.NoError(t, err)
	require.Nil(t, meta)
}

func TestParseReportMetadataResponseEmptyBody(t *testing.T) {
	res := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}

	meta, err := parseReportMetadataResponse(res, nil)
	require.NoError(t, err)
	require.Nil(t, meta)
}

func TestParseReportMetadataResponseEmptyString(t *testing.T) {
	res := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(`""`))),
	}

	meta, err := parseReportMetadataResponse(res, nil)
	require.NoError(t, err)
	require.Nil(t, meta)
}

func TestIsEmptyReport(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want bool
	}{
		{"empty JSON string", []byte(`""`), true},
		{"null", []byte("null"), true},
		{"empty body", []byte{}, true},
		{"whitespace only", []byte("   "), true},
		{"padded empty string", []byte(`  ""  `), true},
		{"valid metadata", []byte(`{"download_links":["https://example.com/report.json"],"report_day":"2026-03-19"}`), false},
		{"valid metadata empty links", []byte(`{"download_links":[],"report_day":"2026-03-19"}`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isEmptyReport(tt.body))
		})
	}
}
