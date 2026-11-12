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

	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/plugins/incidentio/models"
)

func TestMapStatusCategory(t *testing.T) {
	cases := []struct {
		in            string
		expectMapped  string
		expectedKnown bool
	}{
		{"triage", ticket.IN_PROGRESS, true},
		{"live", ticket.IN_PROGRESS, true},
		{"learning", ticket.IN_PROGRESS, true},
		{"paused", ticket.IN_PROGRESS, true},
		{"closed", ticket.DONE, true},
		{"declined", ticket.DONE, true},
		{"merged", ticket.DONE, true},
		{"canceled", ticket.DONE, true},
		// App display names, not API values.
		{"active", ticket.IN_PROGRESS, false},
		{"post-incident", ticket.IN_PROGRESS, false},
		{"wat", ticket.IN_PROGRESS, false},
		{"", ticket.IN_PROGRESS, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			mapped, known := mapStatusCategory(c.in)
			assert.Equal(t, c.expectMapped, mapped)
			assert.Equal(t, c.expectedKnown, known)
		})
	}
}

func TestMapStatusCategoryDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_, _ = mapStatusCategory("brand-new-category-incidentio-invented-yesterday")
	})
}

func TestComputeLeadTime_Resolved(t *testing.T) {
	declared := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 5, 10, 11, 30, 0, 0, time.UTC)
	leadTime, resolutionDate := computeLeadTime(declared, &resolved)
	require.NotNil(t, leadTime)
	require.NotNil(t, resolutionDate)
	assert.Equal(t, uint(90), *leadTime)
	assert.Equal(t, resolved, *resolutionDate)
}

func TestComputeLeadTime_Unresolved(t *testing.T) {
	declared := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	leadTime, resolutionDate := computeLeadTime(declared, nil)
	assert.Nil(t, leadTime)
	assert.Nil(t, resolutionDate)
}

func TestComputeLeadTime_ZeroDuration(t *testing.T) {
	declared := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	resolved := declared
	leadTime, resolutionDate := computeLeadTime(declared, &resolved)
	require.NotNil(t, leadTime)
	require.NotNil(t, resolutionDate)
	assert.Equal(t, uint(0), *leadTime)
}

// Retrospective incidents are declared after resolution: the resolution
// date must survive even though the declared→resolved duration is
// meaningless and the lead time is dropped.
func TestComputeLeadTime_ResolvedBeforeDeclared(t *testing.T) {
	declared := time.Date(2026, 5, 10, 11, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	leadTime, resolutionDate := computeLeadTime(declared, &resolved)
	assert.Nil(t, leadTime)
	require.NotNil(t, resolutionDate)
	assert.Equal(t, resolved, *resolutionDate)
}

func TestIssueKeyFor(t *testing.T) {
	cases := []struct {
		name     string
		incident models.Incident
		expected string
	}{
		{"reference present", models.Incident{Reference: "INC-61", Id: "inc_abc"}, "INC-61"},
		{"missing reference falls back to id", models.Incident{Reference: "", Id: "inc_abc"}, "inc_abc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, issueKeyFor(&c.incident))
		})
	}
}
