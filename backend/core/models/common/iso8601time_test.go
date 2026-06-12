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

package common

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type Iso8601TimeRecord struct {
	Created Iso8601Time
}

type Iso8601TimeRecordP struct {
	Created *Iso8601Time
}

type TimeRecord struct {
	Created time.Time
}

func TimeMustParse(text string) time.Time {
	t, err := time.Parse(time.RFC3339, text)
	if err != nil {
		panic(err)
	}
	return t
}

func TestIso8601Time_Value(t *testing.T) {
	zeroTime := time.Time{}
	testCases := []struct {
		name   string
		input  *Iso8601Time
		output driver.Value
		err    error
	}{
		{
			name:   "Nil value",
			input:  nil,
			output: nil,
			err:    nil,
		},
		{
			name: "Valid time value",
			input: &Iso8601Time{
				Time:   time.Date(2023, 2, 28, 10, 30, 0, 0, time.UTC),
				format: time.RFC3339,
			},
			output: time.Date(2023, 2, 28, 10, 30, 0, 0, time.UTC),
			err:    nil,
		},
		{
			name: "Zero time value",
			input: &Iso8601Time{
				Time:   zeroTime,
				format: time.RFC3339,
			},
			output: nil,
			err:    nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := tc.input.Value()
			if output != tc.output {
				t.Errorf("Expected output to be %v, but got %v", tc.output, output)
			}
			if err != tc.err {
				t.Errorf("Expected error to be %v, but got %v", tc.err, err)
			}
		})
	}
}

func TestIso8601Time_Scan(t *testing.T) {
	testCases := []struct {
		name   string
		input  interface{}
		output *Iso8601Time
		err    error
	}{
		{
			name:   "Valid time value",
			input:  time.Date(2023, 2, 28, 10, 30, 0, 0, time.UTC),
			output: &Iso8601Time{Time: time.Date(2023, 2, 28, 10, 30, 0, 0, time.UTC), format: time.RFC3339},
			err:    nil,
		},
		{
			name:   "Invalid input value",
			input:  "invalid",
			output: &Iso8601Time{},
			err:    fmt.Errorf("can not convert %v to timestamp", "invalid"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var output Iso8601Time
			err := output.Scan(tc.input)
			if !reflect.DeepEqual(tc.output, &output) {
				t.Errorf("Expected output to be %v, but got %v", tc.output, output)
			}
			assert.Equal(t, fmt.Sprintf("%v", err), fmt.Sprintf("%v", tc.err), "Expected error to be %v, but got %v", tc.err, err)
		})
	}
}

func TestConvertStringToTime(t *testing.T) {
	testCases := []struct {
		name   string
		input  string
		output time.Time
		err    error
	}{
		{
			name:   "Valid time string",
			input:  "2023-03-01T12:30:00+0000",
			output: time.Date(2023, 3, 1, 12, 30, 0, 0, time.UTC).Local(),
			err:    nil,
		},
		{
			name:   "Valid date string",
			input:  "2023-03-01",
			output: time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
			err:    nil,
		},
		{
			name:   "Valid time string with milliseconds",
			input:  "2025-09-15T08:53:36.337+00:00",
			output: time.Date(2025, 9, 15, 8, 53, 36, 337000000, time.UTC),
			err:    nil,
		},
		{
			name:   "Valid time string with microseconds (Bitbucket Cloud format)",
			input:  "2025-09-15T08:53:36.337932+00:00",
			output: time.Date(2025, 9, 15, 8, 53, 36, 337932000, time.UTC),
			err:    nil,
		},
		{
			name:   "Invalid time string",
			input:  "invalid",
			output: time.Time{},
			err:    fmt.Errorf("parsing time \"invalid\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"invalid\" as \"2006\""),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := ConvertStringToTime(tc.input)
			if !tc.output.Equal(output) {
				t.Errorf("Expected output to be %v, but got %v", tc.output, output)
			}
			assert.Equal(t, fmt.Sprintf("%v", err), fmt.Sprintf("%v", tc.err), "Expected error to be %v, but got %v", tc.err, err)
		})
	}
}

func TestIsNonDateString(t *testing.T) {
	testCases := []struct {
		name   string
		input  string
		output bool
	}{
		{
			name:   "ZenTao long-term in Chinese",
			input:  "长期",
			output: true,
		},
		{
			name:   "ZenTao long-term in Unicode escape",
			input:  "\\u957f\\u671f",
			output: true,
		},
		{
			name:   "ZenTao long-term in double-escaped Unicode",
			input:  "\\\\u957f\\\\u671f",
			output: true,
		},
		{
			name:   "English long-term",
			input:  "long-term",
			output: true,
		},
		{
			name:   "Chinese permanent",
			input:  "永久",
			output: true,
		},
		{
			name:   "English indefinite",
			input:  "indefinite",
			output: true,
		},
		{
			name:   "English unlimited",
			input:  "unlimited",
			output: true,
		},
		{
			name:   "Valid date string",
			input:  "2023-03-01",
			output: false,
		},
		{
			name:   "Valid datetime string",
			input:  "2023-03-01T12:30:00Z",
			output: false,
		},
		{
			name:   "Random string",
			input:  "random",
			output: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := isNonDateString(tc.input)
			assert.Equal(t, tc.output, output, "Expected output to be %v, but got %v", tc.output, output)
		})
	}
}

func TestIso8601Time_UnmarshalJSON_NonDateStrings(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		shouldErr bool
	}{
		{
			name:      "ZenTao long-term in Chinese",
			input:     `"长期"`,
			shouldErr: false,
		},
		{
			name:      "ZenTao long-term in Unicode escape",
			input:     `"\\u957f\\u671f"`,
			shouldErr: false,
		},
		{
			name:      "English long-term",
			input:     `"long-term"`,
			shouldErr: false,
		},
		{
			name:      "Valid date",
			input:     `"2023-03-01T12:30:00Z"`,
			shouldErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var iso8601Time Iso8601Time
			err := iso8601Time.UnmarshalJSON([]byte(tc.input))
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// For non-date strings, the time should be zero
				if isNonDateString(strings.Trim(tc.input, `"`)) {
					assert.True(t, iso8601Time.Time.IsZero(), "Expected zero time for non-date string")
				}
			}
		})
	}
}

func TestConvertStringToTimeInLoc(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}
	testCases := []struct {
		name   string
		input  string
		loc    *time.Location
		output time.Time
		err    error
	}{
		{
			name:   "Valid time string with location",
			input:  "2023-03-01T12:30:00+0800",
			loc:    loc,
			output: time.Date(2023, 3, 1, 12, 30, 0, 0, loc),
			err:    nil,
		},
		{
			name:   "Valid date string with location",
			input:  "2023-03-01",
			loc:    loc,
			output: time.Date(2023, 3, 1, 0, 0, 0, 0, loc),
			err:    nil,
		},
		{
			name:   "Invalid time string with location",
			input:  "invalid",
			loc:    loc,
			output: time.Time{},
			err:    fmt.Errorf("parsing time \"invalid\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"invalid\" as \"2006\""),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := ConvertStringToTimeInLoc(tc.input, tc.loc)
			if !tc.output.Equal(output) {
				t.Errorf("Expected output to be %v, but got %v", tc.output, output)
			}
			assert.Equal(t, fmt.Sprintf("%v", err), fmt.Sprintf("%v", tc.err), "Expected error to be %v, but got %v", tc.err, err)
		})
	}
}
