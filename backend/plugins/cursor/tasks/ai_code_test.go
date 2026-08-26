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
	"testing"
	"time"
)

func TestParseOptionalISOTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		isNil bool
		want  string
	}{
		{name: "RFC3339", input: "2026-08-20T14:30:00Z", want: "2026-08-20T14:30:00Z"},
		{name: "RFC3339 with offset", input: "2026-08-20T08:30:00-06:00", want: "2026-08-20T14:30:00Z"},
		{name: "RFC3339Nano", input: "2026-08-20T14:30:00.123456789Z", want: "2026-08-20T14:30:00Z"},
		{name: "date only", input: "2026-08-20", want: "2026-08-20T00:00:00Z"},
		{name: "empty string", input: "", isNil: true},
		{name: "whitespace", input: "   ", isNil: true},
		{name: "garbage", input: "not-a-date", isNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOptionalISOTime(tt.input)
			if tt.isNil {
				if result != nil {
					t.Fatalf("expected nil, got %v", *result)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil result for %q", tt.input)
			}
			got := result.UTC().Format(time.RFC3339)
			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSplitAnalyticsDateRange(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC).UnixMilli()

	chunks := splitAnalyticsDateRange(start, end, 30)
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	if chunks[0].StartDate != "2026-07-01" {
		t.Fatalf("first chunk start = %s, want 2026-07-01", chunks[0].StartDate)
	}
	if chunks[len(chunks)-1].EndDate != "2026-08-25" {
		t.Fatalf("last chunk end = %s, want 2026-08-25", chunks[len(chunks)-1].EndDate)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for a 55-day range, got %d", len(chunks))
	}
}

func TestSplitAnalyticsDateRange_Empty(t *testing.T) {
	now := time.Now().UTC().UnixMilli()
	chunks := splitAnalyticsDateRange(now, now, 30)
	if chunks != nil {
		t.Fatalf("expected nil for equal start/end, got %d chunks", len(chunks))
	}
}

func TestSplitAnalyticsDateRange_ShortRange(t *testing.T) {
	start := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC).UnixMilli()

	chunks := splitAnalyticsDateRange(start, end, 30)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for 5-day range, got %d", len(chunks))
	}
	if chunks[0].StartDate != "2026-08-20" || chunks[0].EndDate != "2026-08-25" {
		t.Fatalf("chunk = %v, want 2026-08-20 to 2026-08-25", chunks[0])
	}
}
