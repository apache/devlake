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

package gcshelper

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// FinishedJSON mirrors the structure of finished.json written by Prow after
// each build completes.
type FinishedJSON struct {
	Timestamp int64  `json:"timestamp"`
	Passed    bool   `json:"passed"`
	Result    string `json:"result"`
	Revision  string `json:"revision"`
}

// StartedJSON mirrors the structure of started.json written by Prow when a
// build starts.
type StartedJSON struct {
	Timestamp int64  `json:"timestamp"`
	Commit    string `json:"commit"`
}

// ParseFinishedJSON unmarshals a finished.json payload.
func ParseFinishedJSON(data []byte) (*FinishedJSON, error) {
	var f FinishedJSON
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse finished.json: %w", err)
	}
	return &f, nil
}

// ReadStartedJSON fetches and parses the started.json file located directly
// inside buildPrefix (e.g. "pr-logs/pull/org_repo/42/job-name/1234/").
func ReadStartedJSON(ctx context.Context, store HistoryStore, buildPrefix string) (*StartedJSON, error) {
	data, err := store.ReadFile(ctx, buildPrefix+"started.json")
	if err != nil {
		return nil, fmt.Errorf("read started.json at %s: %w", buildPrefix, err)
	}
	var s StartedJSON
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse started.json at %s: %w", buildPrefix, err)
	}
	return &s, nil
}

// MapProwResult converts a Prow result string and passed flag to a
// normalised CI result string ("success", "failure", "aborted", "error").
func MapProwResult(result string, passed bool) string {
	switch strings.ToUpper(result) {
	case "SUCCESS":
		return "success"
	case "FAILURE":
		return "failure"
	case "ABORTED":
		return "aborted"
	case "ERROR":
		return "error"
	}
	// Fallback to the passed flag when result string is absent or unrecognised.
	if passed {
		return "success"
	}
	return "failure"
}

// LastSegment returns the last non-empty path component of a slash-delimited
// GCS prefix. For example:
//
//	"pr-logs/pull/org_repo/42/job-name/1234/" → "1234"
func LastSegment(prefix string) string {
	parts := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return prefix
}

// SortBuildIDsDescending sorts a slice of GCS build prefixes by their numeric
// build ID (last path segment) in descending order. Prefixes whose last
// segment is not a valid integer are placed at the end.
func SortBuildIDsDescending(prefixes []string) {
	sort.Slice(prefixes, func(i, j int) bool {
		a, errA := strconv.ParseInt(LastSegment(prefixes[i]), 10, 64)
		b, errB := strconv.ParseInt(LastSegment(prefixes[j]), 10, 64)
		if errA != nil || errB != nil {
			return errA == nil
		}
		return a > b
	})
}
