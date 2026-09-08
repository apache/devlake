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

	"github.com/apache/devlake/plugins/datadog/models"
)

const resolvedIncident = `{
	"id": "50f0bbc6-b490-4369-abed-93b63435af94",
	"type": "incidents",
	"attributes": {
		"public_id": 22,
		"title": "headless - Sev_3 - todo backlog age",
		"state": "resolved",
		"severity": "SEV-3",
		"incident_type_uuid": "type-uuid-1",
		"created": "2026-09-03T12:55:53Z",
		"declared": "2026-09-03T12:55:53Z",
		"detected": "2026-09-03T12:54:52Z",
		"resolved": "2026-09-04T09:39:14Z",
		"modified": "2026-09-04T09:39:14Z",
		"customer_impacted": false,
		"customer_impact_duration": 0,
		"is_test": false,
		"visibility": "organization",
		"time_to_detect": 61,
		"time_to_internal_response": 0,
		"time_to_repair": 74601,
		"time_to_resolve": 74601,
		"fields": {
			"detection_method": {"type": "dropdown", "value": "monitor"},
			"services": {"type": "multiselect", "value": ["headless", "backend"]},
			"severity": {"type": "dropdown", "value": "SEV-3"},
			"slug": {"type": "textbox", "value": "IR-22"},
			"root_cause": {"type": "textbox", "value": null},
			"teams": {"type": "autocomplete", "value": null}
		}
	}
}`

func newTestTaskData() *DatadogTaskData {
	return &DatadogTaskData{
		Options: &DatadogOptions{
			ConnectionId:   7,
			IncidentTypeId: "type-uuid-1",
		},
		WebUrl: "https://app.datadoghq.com",
	}
}

func TestExtractIncident_HappyPath(t *testing.T) {
	data := newTestTaskData()
	results, err := extractIncident([]byte(resolvedIncident), data)
	require.NoError(t, err)
	require.Len(t, results, 1)

	incident, ok := results[0].(*models.Incident)
	require.True(t, ok, "first result should be *models.Incident")
	assert.Equal(t, uint64(7), incident.ConnectionId)
	assert.Equal(t, "50f0bbc6-b490-4369-abed-93b63435af94", incident.Id)
	assert.Equal(t, int64(22), incident.PublicId)
	assert.Equal(t, "type-uuid-1", incident.IncidentTypeId)
	assert.Equal(t, "headless - Sev_3 - todo backlog age", incident.Title)
	assert.Equal(t, "resolved", incident.State)
	assert.Equal(t, "SEV-3", incident.Severity)
	// Datadog mirrors its own attributes into `fields`, slug among them.
	assert.Equal(t, "IR-22", incident.Slug)
	assert.Equal(t, "organization", incident.Visibility)
	assert.Equal(t, "https://app.datadoghq.com/incidents/22", incident.Url)
	assert.Equal(t, time.Date(2026, 9, 3, 12, 55, 53, 0, time.UTC), incident.CreatedDate)
	require.NotNil(t, incident.DetectedDate)
	assert.Equal(t, time.Date(2026, 9, 3, 12, 54, 52, 0, time.UTC), *incident.DetectedDate)
	require.NotNil(t, incident.ResolvedDate)
	require.NotNil(t, incident.TimeToRepair)
	assert.Equal(t, int64(74601), *incident.TimeToRepair)
	// The raw fields object is kept so a scope-config change can remap
	// severity or component without re-collecting.
	assert.Contains(t, incident.CustomFields, "detection_method")
	// No scope config, so nothing is mapped onto Component.
	assert.Equal(t, "", incident.Component)
}

func TestExtractIncident_MapsCustomFieldsPerScopeConfig(t *testing.T) {
	data := newTestTaskData()
	data.Options.ScopeConfig = &models.DatadogScopeConfig{
		ComponentField: "services",
		SeverityField:  "detection_method",
	}

	results, err := extractIncident([]byte(resolvedIncident), data)
	require.NoError(t, err)
	require.Len(t, results, 1)

	incident := results[0].(*models.Incident)
	// A multi-select field keeps its first value; the rest stay in the raw object.
	assert.Equal(t, "headless", incident.Component)
	// A configured severity field wins over Datadog's native SEV value.
	assert.Equal(t, "monitor", incident.Severity)
}

func TestExtractIncident_EmptyConfiguredFieldLeavesNativeSeverity(t *testing.T) {
	data := newTestTaskData()
	// `root_cause` and `teams` carry a null value in real payloads, which
	// must read as absent rather than as an empty override.
	data.Options.ScopeConfig = &models.DatadogScopeConfig{
		ComponentField: "teams",
		SeverityField:  "root_cause",
	}

	results, err := extractIncident([]byte(resolvedIncident), data)
	require.NoError(t, err)
	require.Len(t, results, 1)

	incident := results[0].(*models.Incident)
	assert.Equal(t, "", incident.Component)
	assert.Equal(t, "SEV-3", incident.Severity)
}

func TestExtractIncident_SkipsTestIncidents(t *testing.T) {
	data := newTestTaskData()
	results, err := extractIncident([]byte(`{
		"id": "test-1",
		"attributes": {
			"public_id": 4,
			"title": "[on-call-trial-test]. It's all down",
			"incident_type_uuid": "type-uuid-1",
			"created": "2026-08-25T12:37:05Z",
			"is_test": true
		}
	}`), data)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestExtractIncident_SkipsOtherIncidentTypes(t *testing.T) {
	data := newTestTaskData()
	results, err := extractIncident([]byte(`{
		"id": "other-type",
		"attributes": {
			"public_id": 5,
			"title": "not this scope",
			"incident_type_uuid": "type-uuid-2",
			"created": "2026-08-25T12:37:05Z"
		}
	}`), data)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestExtractIncident_CollectsEveryTypeWhenScopeIsEmpty(t *testing.T) {
	data := newTestTaskData()
	data.Options.IncidentTypeId = ""
	results, err := extractIncident([]byte(`{
		"id": "any-type",
		"attributes": {
			"public_id": 5,
			"title": "any type",
			"incident_type_uuid": "type-uuid-2",
			"created": "2026-08-25T12:37:05Z"
		}
	}`), data)
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestExtractIncident_MissingCreatedIsAnError(t *testing.T) {
	data := newTestTaskData()
	_, err := extractIncident([]byte(`{
		"id": "no-created",
		"attributes": {"public_id": 6, "title": "no timestamps", "incident_type_uuid": "type-uuid-1"}
	}`), data)
	assert.Error(t, err)
}

func TestExtractIncident_NoAttributesIsSkipped(t *testing.T) {
	data := newTestTaskData()
	results, err := extractIncident([]byte(`{"id": "bare", "type": "incidents"}`), data)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestIncidentUrl(t *testing.T) {
	assert.Equal(t, "https://app.datadoghq.com/incidents/22", incidentUrl("https://app.datadoghq.com", 22))
	// A trailing slash must not double up.
	assert.Equal(t, "https://app.datadoghq.com/incidents/22", incidentUrl("https://app.datadoghq.com/", 22))
	// Without a configured UI origin there is no link to give.
	assert.Equal(t, "", incidentUrl("", 22))
	assert.Equal(t, "", incidentUrl("https://app.datadoghq.com", 0))
}
