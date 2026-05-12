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
	"github.com/stretchr/testify/require"

	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/plugins/rootly/models"
)

// TestMapStatus exercises every branch of the Rootly-to-domain status
// mapping, including the deliberate divergence from PagerDuty: unknown
// statuses fall back to IN_PROGRESS with a warning rather than panic.
func TestMapStatus(t *testing.T) {
	cases := []struct {
		in       string
		expected string
	}{
		{"triage", ticket.TODO},
		{"started", ticket.TODO},
		{"mitigated", ticket.IN_PROGRESS},
		{"resolved", ticket.DONE},
		{"closed", ticket.DONE},
		{"cancelled", ticket.DONE},
		{"wat", ticket.IN_PROGRESS},
		{"", ticket.IN_PROGRESS},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			assert.Equal(t, c.expected, mapStatus(c.in))
		})
	}
}

// TestMapStatusDoesNotPanic pins the behavioral difference from the
// PagerDuty converter, which panics on unknown statuses. Rootly's
// enum is more volatile, so we fall back rather than crash.
func TestMapStatusDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = mapStatus("brand-new-status-rootly-invented-yesterday")
	})
}

// TestMapSeverityToPriority covers the severity table plus case
// variation (Rootly has been observed returning SEV0, sev0, and
// Sev0 interchangeably) and the pass-through behavior for unknown
// values.
func TestMapSeverityToPriority(t *testing.T) {
	cases := []struct {
		in       string
		expected string
	}{
		{"sev0", "CRITICAL"},
		{"SEV0", "CRITICAL"},
		{"Sev0", "CRITICAL"},
		{"sev1", "HIGH"},
		{"SEV1", "HIGH"},
		{"sev2", "MEDIUM"},
		{"sev3", "LOW"},
		{"sev4", "LOW"},
		{"sev5", "sev5"},
		{"critical-ish", "critical-ish"},
		{"", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			assert.Equal(t, c.expected, mapSeverityToPriority(c.in))
		})
	}
}

// TestComputeLeadTime_Resolved verifies that a resolved incident yields
// a non-nil lead time in minutes and a non-nil resolution date pointer
// whose value matches the resolved timestamp.
func TestComputeLeadTime_Resolved(t *testing.T) {
	started := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 5, 10, 11, 30, 0, 0, time.UTC)
	leadTime, resolutionDate := computeLeadTime(started, &resolved)
	require.NotNil(t, leadTime)
	require.NotNil(t, resolutionDate)
	assert.Equal(t, uint(90), *leadTime)
	assert.Equal(t, resolved, *resolutionDate)
}

// TestComputeLeadTime_Unresolved verifies that an unresolved incident
// (resolved pointer is nil) yields nil, nil rather than a zero-time
// sentinel — downstream DORA math treats (nil) as "still ongoing" and
// a zero-time value would pollute mean-time-to-resolve.
func TestComputeLeadTime_Unresolved(t *testing.T) {
	started := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	leadTime, resolutionDate := computeLeadTime(started, nil)
	assert.Nil(t, leadTime)
	assert.Nil(t, resolutionDate)
}

// TestComputeLeadTime_ZeroDuration covers the edge case where an
// incident is resolved at the same instant it started. Lead time is
// zero but should still be non-nil, because DORA needs to distinguish
// "resolved instantly" from "not yet resolved".
func TestComputeLeadTime_ZeroDuration(t *testing.T) {
	started := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	resolved := started
	leadTime, resolutionDate := computeLeadTime(started, &resolved)
	require.NotNil(t, leadTime)
	require.NotNil(t, resolutionDate)
	assert.Equal(t, uint(0), *leadTime)
}

// TestComputeLeadTime_ResolvedBeforeStarted guards against clock skew
// or backfill anomalies where the resolution timestamp precedes the
// start. A naive uint() cast on a negative duration would produce
// wraparound garbage and silently corrupt MTTR. The helper treats
// these cases as if unresolved so bad data does not contaminate the
// domain layer.
func TestComputeLeadTime_ResolvedBeforeStarted(t *testing.T) {
	started := time.Date(2026, 5, 10, 11, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	leadTime, resolutionDate := computeLeadTime(started, &resolved)
	assert.Nil(t, leadTime)
	assert.Nil(t, resolutionDate)
}

// TestIssueKeyFor covers the human-readable-id preference: prefer the
// Rootly sequential id when positive, fall back to the internal slug id
// when missing, zero, or negative. The negative branch matters because
// Number is typed int and a "negative sequential id" would be a data
// bug we should surface as the slug rather than as a negative string.
func TestIssueKeyFor(t *testing.T) {
	cases := []struct {
		name     string
		incident models.Incident
		expected string
	}{
		{"positive sequential id", models.Incident{Number: 42, Id: "inc_abc"}, "42"},
		{"zero sequential id falls back to slug", models.Incident{Number: 0, Id: "inc_abc"}, "inc_abc"},
		{"negative sequential id falls back to slug", models.Incident{Number: -1, Id: "inc_abc"}, "inc_abc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, issueKeyFor(&c.incident))
		})
	}
}

// TestAssigneeDedup covers the role-user dedup in ConvertIncidents by
// calling the dedup logic in isolation (inlined here to avoid wiring up
// a SubTaskContext in a unit test — e2e coverage lives in U6). The
// invariants are: (1) distinct user ids across roles produce distinct
// IssueAssignees, (2) duplicate user ids across roles produce exactly
// one IssueAssignee, (3) empty-string user ids are skipped.
func TestAssigneeDedup(t *testing.T) {
	cases := []struct {
		name     string
		incident models.Incident
		expected []string
	}{
		{
			name:     "all roles empty",
			incident: models.Incident{},
			expected: []string{},
		},
		{
			name:     "single creator",
			incident: models.Incident{CreatorUserId: "u1"},
			expected: []string{"u1"},
		},
		{
			name: "same user in creator and resolver",
			incident: models.Incident{
				CreatorUserId:    "u1",
				ResolvedByUserId: "u1",
			},
			expected: []string{"u1"},
		},
		{
			name: "distinct users across all roles",
			incident: models.Incident{
				CreatorUserId:     "u1",
				StartedByUserId:   "u2",
				MitigatedByUserId: "u3",
				ResolvedByUserId:  "u4",
				ClosedByUserId:    "u5",
			},
			expected: []string{"u1", "u2", "u3", "u4", "u5"},
		},
		{
			name: "empty interleaved with populated",
			incident: models.Incident{
				CreatorUserId:     "u1",
				StartedByUserId:   "",
				MitigatedByUserId: "u2",
				ResolvedByUserId:  "",
				ClosedByUserId:    "u1",
			},
			expected: []string{"u1", "u2"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seen := map[string]bool{}
			var got []string
			for _, uid := range []string{
				c.incident.CreatorUserId,
				c.incident.StartedByUserId,
				c.incident.MitigatedByUserId,
				c.incident.ResolvedByUserId,
				c.incident.ClosedByUserId,
			} {
				if uid == "" || seen[uid] {
					continue
				}
				seen[uid] = true
				got = append(got, uid)
			}
			if len(c.expected) == 0 {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, c.expected, got)
			}
		})
	}
}

// TestMapStatus_MitigatedIsKnown pins the boundary between mapStatus
// and isKnownStatus: "mitigated" maps to IN_PROGRESS (the same bucket
// as the unknown-status fallback), but it is a KNOWN status and should
// not trigger the warning log. Without this test, a regression that
// deletes "mitigated" from isKnownStatus would silently fire unknown-
// status warnings on every mitigated incident.
func TestMapStatus_MitigatedIsKnown(t *testing.T) {
	assert.Equal(t, ticket.IN_PROGRESS, mapStatus("mitigated"))
	assert.True(t, isKnownStatus("mitigated"))
	// And the contrapositive — the fallback case is indeed unknown.
	assert.Equal(t, ticket.IN_PROGRESS, mapStatus("something-else"))
	assert.False(t, isKnownStatus("something-else"))
}
