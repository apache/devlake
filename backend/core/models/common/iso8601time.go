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
	"regexp"
	"strings"
	"time"
)

/*
parse iso8601 datetime format for json.Unmarshal

declare your field type as:

type Foo struct {
	Created Iso8601Time
}

foo := &Foo{}
err := errors.ConvertError(json.Unmarshal("{\"created\": \"2021-02-19T01:53:35.340+0800\"}", foo))
var time time.Time
time = foo.Created.ToTime()
*/

// DateTimeFormatItem FIXME ...
// TODO: move this to helper
type DateTimeFormatItem struct {
	Matcher *regexp.Regexp
	Format  string
}

// DateTimeFormats FIXME ...
var DateTimeFormats []DateTimeFormatItem

func init() {
	DateTimeFormats = []DateTimeFormatItem{
		{
			Matcher: regexp.MustCompile(`[+-][\d]{4}$`),
			Format:  "2006-01-02T15:04:05-0700",
		},
		{
			Matcher: regexp.MustCompile(`[\d]{6}[+-][\d]{2}:[\d]{2}$`),
			Format:  "2006-01-02T15:04:05.000000-07:00",
		},
		{
			Matcher: regexp.MustCompile(`[\d]{3}[+-][\d]{2}:[\d]{2}$`),
			Format:  "2006-01-02T15:04:05.000-07:00",
		},
		{
			Matcher: regexp.MustCompile(`[+-][\d]{2}:[\d]{2}$`),
			Format:  "2006-01-02T15:04:05-07:00",
		},
		{
			Matcher: regexp.MustCompile(`[\d]{4}-[\d]{2}-[\d]{2} [\d]{2}:[\d]{2}:[\d]{2}$`),
			Format:  "2006-01-02 15:04:05",
		},
		{
			Matcher: regexp.MustCompile(`[+-][\d]{2}-[\d]{2}$`),
			Format:  "2006-01-02",
		},
		{
			Matcher: regexp.MustCompile(`[\d]{4}-[\d]{2}-[\d]{2} [\d]{2}:[\d]{2}$`),
			Format:  "2006-01-02 15:04",
		},
	}
}

// Iso8601Time is type time.Time
type Iso8601Time struct {
	Time   time.Time
	format string
}

func (jt *Iso8601Time) String() string {
	format := jt.format
	if format == "" {
		format = DateTimeFormats[0].Format
	}
	return jt.Time.Format(format)
}

// MarshalJSON FIXME ...
func (jt Iso8601Time) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, jt.String())), nil
}

// UnmarshalJSON FIXME ...
func (jt *Iso8601Time) UnmarshalJSON(b []byte) error {
	timeString := string(b)
	if timeString == `""` {
		return nil
	}
	if timeString == "null" {
		return nil
	}
	if strings.Contains(timeString, "0000-00-00") {
		return nil
	}
	timeString = strings.Trim(timeString, `"`)

	// Handle special cases for non-standard date representations
	// Some systems may use text like "长期" (long-term) instead of actual dates
	if isNonDateString(timeString) {
		jt.Time = time.Time{}
		return nil
	}

	t, err := ConvertStringToTime(timeString)
	if err != nil {
		return err
	}
	jt.Time = t
	return nil
}

// isNonDateString checks if a string represents a non-date value like "long-term"
func isNonDateString(s string) bool {
	// Handle various representations of "long-term" in different systems
	nonDateStrings := []string{
		"长期",                 // Chinese for "long-term"
		"\\u957f\\u671f",     // Unicode escape sequence for "长期"
		"\\\\u957f\\\\u671f", // Double-escaped Unicode sequence
		"long-term",          // English
		"永久",                 // Chinese for "permanent"
		"indefinite",         // English
		"unlimited",          // English
	}

	for _, nonDate := range nonDateStrings {
		if s == nonDate {
			return true
		}
	}

	// Also check if the string contains the Unicode escape pattern for "长期"
	// This handles cases where escape sequences might be processed differently
	if strings.Contains(s, "957f") && strings.Contains(s, "671f") {
		return true
	}

	return false
}

// ToTime FIXME ...
func (jt *Iso8601Time) ToTime() time.Time {
	return jt.Time
}

// ToNullableTime FIXME ...
func (jt *Iso8601Time) ToNullableTime() *time.Time {
	if jt == nil {
		return nil
	}
	return &jt.Time
}

// ConvertStringToTime FIXME ...
func ConvertStringToTime(timeString string) (t time.Time, err error) {
	for _, formatItem := range DateTimeFormats {
		if formatItem.Matcher.MatchString(timeString) {
			return time.Parse(formatItem.Format, timeString)
		}
	}
	return time.Parse(time.RFC3339, timeString)
}

// ConvertStringToTimeInLoc converts a string to time.Time in the given location.
func ConvertStringToTimeInLoc(timeString string, loc *time.Location) (t time.Time, err error) {
	for _, formatItem := range DateTimeFormats {
		if formatItem.Matcher.MatchString(timeString) {
			return time.ParseInLocation(formatItem.Format, timeString, loc)
		}
	}
	return time.ParseInLocation(time.RFC3339, timeString, loc)
}

// Iso8601TimeToTime FIXME ...
func Iso8601TimeToTime(iso8601Time *Iso8601Time) *time.Time {
	if iso8601Time == nil {
		return nil
	}
	t := iso8601Time.ToTime()
	return &t
}

// Value FIXME ...
func (jt *Iso8601Time) Value() (driver.Value, error) {
	if jt == nil {
		return nil, nil
	}
	var zeroTime time.Time
	t := jt.Time
	if t.UnixNano() == zeroTime.UnixNano() {
		return nil, nil
	}
	return t, nil
}

// Scan FIXME ...
func (jt *Iso8601Time) Scan(v interface{}) error {
	value, ok := v.(time.Time)
	if ok {
		*jt = Iso8601Time{
			Time:   value,
			format: time.RFC3339,
		}
		return nil
	}
	return fmt.Errorf("can not convert %v to timestamp", v)
}
