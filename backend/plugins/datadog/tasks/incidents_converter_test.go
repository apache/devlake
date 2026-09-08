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
	"github.com/apache/devlake/plugins/datadog/models"
)

func TestMapState(t *testing.T) {
	for state, expected := range map[string]string{
		"active":   ticket.IN_PROGRESS,
		"stable":   ticket.IN_PROGRESS,
		"resolved": ticket.DONE,
		"complete": ticket.IN_PROGRESS,
	} {
		mapped, _ := mapState(state)
		assert.Equal(t, expected, mapped, "state %s", state)
	}

	_, known := mapState("resolved")
	assert.True(t, known)
	// An unrecognized state must not fail a pipeline; it reports itself
	// as unknown so the caller can log it.
	_, known = mapState("something-new")
	assert.False(t, known)
}

func TestRestoreMinutes_PrefersTimeToRepair(t *testing.T) {
	repair := int64(1800)
	created := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	resolved := created.Add(48 * time.Hour)

	minutes := restoreMinutes(&models.Incident{
		CreatedDate:  created,
		ResolvedDate: &resolved,
		TimeToRepair: &repair,
	})

	// 30 minutes of restore time, not the two days until the incident was
	// finally closed out.
	require.NotNil(t, minutes)
	assert.Equal(t, uint(30), *minutes)
}

func TestRestoreMinutes_FallsBackToResolvedMinusCreated(t *testing.T) {
	created := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	resolved := created.Add(90 * time.Minute)

	minutes := restoreMinutes(&models.Incident{CreatedDate: created, ResolvedDate: &resolved})

	require.NotNil(t, minutes)
	assert.Equal(t, uint(90), *minutes)
}

func TestRestoreMinutes_UnresolvedHasNone(t *testing.T) {
	minutes := restoreMinutes(&models.Incident{CreatedDate: time.Now()})
	assert.Nil(t, minutes)
}

func TestRestoreMinutes_RetrospectiveIncidentHasNone(t *testing.T) {
	created := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	resolved := created.Add(-2 * time.Hour)

	// Declaring an incident after the fact makes resolved precede created.
	// A naive uint cast of that negative duration would wrap to a huge
	// value and silently corrupt MTTR.
	minutes := restoreMinutes(&models.Incident{CreatedDate: created, ResolvedDate: &resolved})
	assert.Nil(t, minutes)
}

func TestRestoreMinutes_NegativeTimeToRepairIsIgnored(t *testing.T) {
	repair := int64(-5)
	created := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	resolved := created.Add(10 * time.Minute)

	minutes := restoreMinutes(&models.Incident{
		CreatedDate:  created,
		ResolvedDate: &resolved,
		TimeToRepair: &repair,
	})

	require.NotNil(t, minutes)
	assert.Equal(t, uint(10), *minutes)
}
