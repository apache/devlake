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

	"github.com/stretchr/testify/assert"

	"github.com/apache/devlake/helpers/pluginhelper/api"
)

// The expected strings below are the exact query shapes verified live against
// a real stack (see grafana_irm_plan.md §10.1): `isdrill:false`, and the
// `or(...)` date-range group added for an incremental sync.
func TestBuildIncidentsQueryString(t *testing.T) {
	since := time.Date(2026, 9, 19, 11, 2, 0, 0, time.UTC)
	until := time.Date(2026, 9, 19, 23, 59, 59, 0, time.UTC)

	cases := []struct {
		name     string
		since    *time.Time
		until    *time.Time
		expected string
	}{
		{
			name:     "full sync",
			expected: "isdrill:false",
		},
		{
			name:     "incremental",
			since:    &since,
			until:    &until,
			expected: "isdrill:false or(declared:2026-09-19T11:02:00Z,2026-09-19T23:59:59Z resolved:2026-09-19T11:02:00Z,2026-09-19T23:59:59Z)",
		},
		{
			name:     "a since with no until falls back to full sync",
			since:    &since,
			expected: "isdrill:false",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, buildIncidentsQueryString(tc.since, tc.until))
		})
	}
}

// Regression test for a real panic hit on a live pipeline run:
// NewDalCursorIterator (see CollectIncidents' CollectUnfinishedDetails half)
// hands back *simplifiedIncident, not simplifiedIncident — reflect.New always
// yields a pointer — so asserting the value type here panicked with
// "interface conversion: interface {} is *tasks.simplifiedIncident, not
// tasks.simplifiedIncident" the first time this path actually ran against a
// connection with an unresolved incident already synced (§14.1). Neither the
// unit tests nor the e2e fixtures exercised this iterator before that.
func TestUnfinishedDetailsRequestBody(t *testing.T) {
	reqData := &api.RequestData{Input: &simplifiedIncident{Id: "42"}}
	body := unfinishedDetailsRequestBody(reqData)
	assert.Equal(t, map[string]interface{}{"incidentID": "42"}, body)
}

// unfinishedDetailsHeader carries simplifiedIncident.UpdatedDate onto the
// outgoing request so ResponseParser can compare it against the freshly
// fetched modifiedTime (see incidentUnchanged below); this is what
// incidentUnchanged actually reads back off res.Request.Header.
func TestUnfinishedDetailsHeader(t *testing.T) {
	updated := time.Date(2026, 9, 20, 5, 1, 15, 377000000, time.UTC)
	reqData := &api.RequestData{Input: &simplifiedIncident{Id: "6", UpdatedDate: updated}}
	header, err := unfinishedDetailsHeader(reqData, nil)
	assert.NoError(t, err)
	assert.Equal(t, "2026-09-20T05:01:15.377Z", header.Get(knownModifiedHeader))
}

// incidentUnchanged is the actual fix for grafana_irm_plan.md §14.3: without
// it, the "unfinished details" pass inserts a fresh raw row for every
// currently-open incident on every single pipeline run, regardless of
// whether anything changed, since Grafana IRM has no "modified since" filter
// to ask for only what's new (re-confirmed live and against the docs, see
// §14.3).
func TestIncidentUnchanged(t *testing.T) {
	cases := []struct {
		name            string
		fetchedModified string
		knownModified   string
		expectUnchanged bool
	}{
		{
			name:            "identical instant, same precision",
			fetchedModified: "2026-09-20T05:01:15.377000Z",
			knownModified:   "2026-09-20T05:01:15.377Z",
			expectUnchanged: true,
		},
		{
			name: "same instant, fetched has extra sub-millisecond precision " +
				"MySQL's datetime(3) already dropped — must still count as unchanged",
			fetchedModified: "2026-09-20T05:01:15.377404Z",
			knownModified:   "2026-09-20T05:01:15.377Z",
			expectUnchanged: true,
		},
		{
			name:            "genuinely different instant",
			fetchedModified: "2026-09-20T05:05:00.000Z",
			knownModified:   "2026-09-20T05:01:15.377Z",
			expectUnchanged: false,
		},
		{
			name:            "empty header (never set, or a bug) fails safe to changed",
			fetchedModified: "2026-09-20T05:01:15.377Z",
			knownModified:   "",
			expectUnchanged: false,
		},
		{
			name:            "malformed header fails safe to changed",
			fetchedModified: "2026-09-20T05:01:15.377Z",
			knownModified:   "not-a-timestamp",
			expectUnchanged: false,
		},
		{
			name:            "malformed fetched time fails safe to changed",
			fetchedModified: "not-a-timestamp",
			knownModified:   "2026-09-20T05:01:15.377Z",
			expectUnchanged: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectUnchanged, incidentUnchanged(tc.fetchedModified, tc.knownModified))
		})
	}
}
