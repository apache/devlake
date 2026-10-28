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

package parse

import (
	"regexp"
	"strings"
)

var (
	dateOnlyPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	folderPattern   = regexp.MustCompile(`^\d{8}-\d{6}$`)
	zuluPattern     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$`)
	offsetPattern   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?[+-]\d{2}:?\d{2}$`)
	bareTimePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?$`)
)

// TimeEntry is one source timestamp with its precision left intact.
type TimeEntry struct {
	Type           string
	Raw            string
	Normalized     string
	Precision      string
	TimezoneStatus string
	AppliesTo      string
}

// ClassifyTime preserves date-only and timezone-free values.
// A date-only value is not converted to midnight UTC.
func ClassifyTime(raw string) TimeEntry {
	raw = strings.TrimSpace(raw)
	entry := TimeEntry{Raw: raw, TimezoneStatus: "unspecified"}
	switch {
	case raw == "":
		entry.Precision = "unknown"
	case dateOnlyPattern.MatchString(raw):
		entry.Precision = "date"
	case folderPattern.MatchString(raw):
		entry.Precision = "second"
	case zuluPattern.MatchString(raw):
		entry.Precision = "second"
		entry.TimezoneStatus = "utc"
		entry.Normalized = raw
	case offsetPattern.MatchString(raw):
		entry.Precision = "second"
		entry.TimezoneStatus = "known"
		entry.Normalized = raw
	case bareTimePattern.MatchString(raw):
		entry.Precision = "second"
	default:
		entry.Precision = "unknown"
	}
	return entry
}
