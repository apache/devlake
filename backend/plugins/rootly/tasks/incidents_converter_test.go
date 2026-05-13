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

func TestMapStatus(t *testing.T) {
	cases := []struct {
		in            string
		expectMapped  string
		expectedKnown bool
	}{
		{"triage", ticket.TODO, true},
		{"started", ticket.TODO, true},
		{"mitigated", ticket.IN_PROGRESS, true},
		{"resolved", ticket.DONE, true},
		{"closed", ticket.DONE, true},
		{"cancelled", ticket.DONE, true},
		{"wat", ticket.IN_PROGRESS, false},
		{"", ticket.IN_PROGRESS, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			mapped, known := mapStatus(c.in)
			assert.Equal(t, c.expectMapped, mapped)
			assert.Equal(t, c.expectedKnown, known)
		})
	}
}

func TestMapStatusDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_, _ = mapStatus("brand-new-status-rootly-invented-yesterday")
	})
}

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

func TestComputeLeadTime_Resolved(t *testing.T) {
	started := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 5, 10, 11, 30, 0, 0, time.UTC)
	leadTime, resolutionDate := computeLeadTime(started, &resolved)
	require.NotNil(t, leadTime)
	require.NotNil(t, resolutionDate)
	assert.Equal(t, uint(90), *leadTime)
	assert.Equal(t, resolved, *resolutionDate)
}

func TestComputeLeadTime_Unresolved(t *testing.T) {
	started := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	leadTime, resolutionDate := computeLeadTime(started, nil)
	assert.Nil(t, leadTime)
	assert.Nil(t, resolutionDate)
}

func TestComputeLeadTime_ZeroDuration(t *testing.T) {
	started := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	resolved := started
	leadTime, resolutionDate := computeLeadTime(started, &resolved)
	require.NotNil(t, leadTime)
	require.NotNil(t, resolutionDate)
	assert.Equal(t, uint(0), *leadTime)
}

func TestComputeLeadTime_ResolvedBeforeStarted(t *testing.T) {
	started := time.Date(2026, 5, 10, 11, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	leadTime, resolutionDate := computeLeadTime(started, &resolved)
	assert.Nil(t, leadTime)
	assert.Nil(t, resolutionDate)
}

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
			for _, uid := range c.incident.RoleUserIds() {
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

func TestMapStatus_MitigatedIsKnown(t *testing.T) {
	mapped, known := mapStatus("mitigated")
	assert.Equal(t, ticket.IN_PROGRESS, mapped)
	assert.True(t, known)
	mapped, known = mapStatus("something-else")
	assert.Equal(t, ticket.IN_PROGRESS, mapped)
	assert.False(t, known)
}
