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

	"github.com/apache/incubator-devlake/plugins/rootly/models"
)

const baseHappyPathActive = `{
	"id": "inc_01",
	"type": "incidents",
	"attributes": {
		"sequential_id": 42,
		"title": "db outage",
		"summary": "replica lag blew past threshold",
		"url": "https://rootly.example.com/incidents/inc_01",
		"status": "started",
		"severity": {"data": {"id": "sev-uuid-1", "type": "severities", "attributes": {"slug": "sev1", "name": "SEV1", "severity": "high"}}},
		"started_at": "2026-05-10T10:00:00Z",
		"updated_at": "2026-05-10T10:05:00Z",
		"user": {"data": {"id": "usr_100", "type": "users", "attributes": {"name": "Reporter One", "full_name": "Reporter One", "email": "reporter@example.com"}}}
	},
	"relationships": {
		"services": {"data": [{"id": "svc_02", "type": "services"}]}
	}
}`

func newTestOptions() *RootlyOptions {
	return &RootlyOptions{
		ConnectionId: 7,
		ServiceId:    "svc_02",
	}
}

func collectUsers(results []interface{}) []*models.User {
	users := []*models.User{}
	for _, r := range results {
		if u, ok := r.(*models.User); ok {
			users = append(users, u)
		}
	}
	return users
}

func TestExtractRootlyIncident_HappyPathActive(t *testing.T) {
	op := newTestOptions()
	results, err := extractRootlyIncident([]byte(baseHappyPathActive), op)
	require.NoError(t, err)
	require.Len(t, results, 2)

	incident, ok := results[0].(*models.Incident)
	require.True(t, ok, "first result should be *models.Incident")
	assert.Equal(t, uint64(7), incident.ConnectionId)
	assert.Equal(t, "inc_01", incident.Id)
	assert.Equal(t, 42, incident.Number)
	assert.Equal(t, "svc_02", incident.ServiceId)
	assert.Equal(t, "db outage", incident.Title)
	assert.Equal(t, "replica lag blew past threshold", incident.Summary)
	assert.Equal(t, "https://rootly.example.com/incidents/inc_01", incident.Url)
	assert.Equal(t, "started", incident.Status)
	assert.Equal(t, "sev1", incident.Severity)
	assert.Equal(t, time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC), incident.StartedDate)
	assert.Nil(t, incident.AcknowledgedDate)
	assert.Nil(t, incident.MitigatedDate)
	assert.Nil(t, incident.ResolvedDate)
	assert.Equal(t, time.Date(2026, 5, 10, 10, 5, 0, 0, time.UTC), incident.UpdatedDate)

	assert.Equal(t, "usr_100", incident.CreatorUserId)
	assert.Empty(t, incident.StartedByUserId)
	assert.Empty(t, incident.MitigatedByUserId)
	assert.Empty(t, incident.ResolvedByUserId)
	assert.Empty(t, incident.ClosedByUserId)

	users := collectUsers(results)
	require.Len(t, users, 1)
	assert.Equal(t, "usr_100", users[0].Id)
	assert.Equal(t, uint64(7), users[0].ConnectionId)
	assert.Equal(t, "Reporter One", users[0].Name)
	assert.Equal(t, "reporter@example.com", users[0].Email)
}

func TestExtractRootlyIncident_Resolved(t *testing.T) {
	raw := []byte(`{
		"id": "inc_02",
		"type": "incidents",
		"attributes": {
			"sequential_id": 43,
			"title": "cache cleared",
			"status": "resolved",
			"severity": {"data": {"id": "sev-uuid-3", "type": "severities", "attributes": {"slug": "sev3", "severity": "low"}}},
			"started_at": "2026-05-09T08:00:00Z",
			"acknowledged_at": "2026-05-09T08:05:00Z",
			"mitigated_at": "2026-05-09T08:30:00Z",
			"resolved_at": "2026-05-09T09:00:00Z",
			"updated_at": "2026-05-09T09:01:00Z",
			"user": {"data": {"id": "usr_100", "type": "users", "attributes": {"full_name": "Reporter One"}}},
			"resolved_by": {"data": {"id": "usr_200", "type": "users", "attributes": {"full_name": "Resolver Two"}}}
		},
		"relationships": {
			"services": {"data": [{"id": "svc_02", "type": "services"}]}
		}
	}`)
	op := newTestOptions()
	results, err := extractRootlyIncident(raw, op)
	require.NoError(t, err)
	require.Len(t, results, 3)

	incident := results[0].(*models.Incident)
	require.NotNil(t, incident.AcknowledgedDate)
	require.NotNil(t, incident.MitigatedDate)
	require.NotNil(t, incident.ResolvedDate)
	assert.Equal(t, "resolved", incident.Status)
	assert.Equal(t, time.Date(2026, 5, 9, 9, 0, 0, 0, time.UTC), *incident.ResolvedDate)
	assert.Equal(t, time.Date(2026, 5, 9, 8, 30, 0, 0, time.UTC), *incident.MitigatedDate)
	assert.Equal(t, time.Date(2026, 5, 9, 8, 5, 0, 0, time.UTC), *incident.AcknowledgedDate)

	assert.Equal(t, "usr_100", incident.CreatorUserId)
	assert.Equal(t, "usr_200", incident.ResolvedByUserId)

	users := collectUsers(results)
	require.Len(t, users, 2)
	ids := map[string]string{}
	for _, u := range users {
		ids[u.Id] = u.Name
	}
	assert.Equal(t, "Reporter One", ids["usr_100"])
	assert.Equal(t, "Resolver Two", ids["usr_200"])
}

func TestExtractRootlyIncident_MissingOptionalTimestamps(t *testing.T) {
	raw := []byte(`{
		"id": "inc_03",
		"type": "incidents",
		"attributes": {
			"sequential_id": 44,
			"title": "ongoing issue",
			"status": "started",
			"started_at": "2026-05-10T12:00:00Z",
			"updated_at": "2026-05-10T12:05:00Z"
		},
		"relationships": {
			"services": {"data": [{"id": "svc_02", "type": "services"}]}
		}
	}`)
	op := newTestOptions()
	results, err := extractRootlyIncident(raw, op)
	require.NoError(t, err)
	require.Len(t, results, 1)
	incident := results[0].(*models.Incident)
	assert.Nil(t, incident.MitigatedDate)
	assert.Nil(t, incident.ResolvedDate)
	assert.Nil(t, incident.AcknowledgedDate)
}

func TestExtractRootlyIncident_NullSeverity(t *testing.T) {
	raw := []byte(`{
		"id": "inc_04",
		"type": "incidents",
		"attributes": {
			"sequential_id": 45,
			"title": "no sev yet",
			"status": "mitigated",
			"severity": null,
			"started_at": "2026-05-10T14:00:00Z",
			"updated_at": "2026-05-10T14:05:00Z"
		},
		"relationships": {
			"services": {"data": [{"id": "svc_02", "type": "services"}]}
		}
	}`)
	op := newTestOptions()
	results, err := extractRootlyIncident(raw, op)
	require.NoError(t, err)
	require.Len(t, results, 1)
	incident := results[0].(*models.Incident)
	assert.Equal(t, "", incident.Severity)
}

func TestExtractRootlyIncident_NoRolesFilled(t *testing.T) {
	raw := []byte(`{
		"id": "inc_05",
		"type": "incidents",
		"attributes": {
			"sequential_id": 46,
			"title": "ghost incident",
			"status": "started",
			"started_at": "2026-05-10T15:00:00Z",
			"updated_at": "2026-05-10T15:05:00Z",
			"user": null,
			"started_by": null,
			"mitigated_by": null,
			"resolved_by": null,
			"closed_by": null
		},
		"relationships": {
			"services": {"data": [{"id": "svc_02", "type": "services"}]}
		}
	}`)
	op := newTestOptions()
	results, err := extractRootlyIncident(raw, op)
	require.NoError(t, err)
	require.Len(t, results, 1)
	incident := results[0].(*models.Incident)
	assert.Empty(t, incident.CreatorUserId)
	assert.Empty(t, incident.StartedByUserId)
	assert.Empty(t, incident.MitigatedByUserId)
	assert.Empty(t, incident.ResolvedByUserId)
	assert.Empty(t, incident.ClosedByUserId)
	assert.Empty(t, collectUsers(results))
}

func TestExtractRootlyIncident_SameUserInMultipleRoles(t *testing.T) {
	raw := []byte(`{
		"id": "inc_dup",
		"type": "incidents",
		"attributes": {
			"sequential_id": 47,
			"title": "solo fire",
			"status": "resolved",
			"started_at": "2026-05-10T16:00:00Z",
			"resolved_at": "2026-05-10T16:30:00Z",
			"updated_at": "2026-05-10T16:31:00Z",
			"user":        {"data": {"id": "usr_100", "type": "users", "attributes": {"full_name": "Solo Operator"}}},
			"resolved_by": {"data": {"id": "usr_100", "type": "users", "attributes": {"full_name": "Solo Operator"}}}
		},
		"relationships": {
			"services": {"data": [{"id": "svc_02", "type": "services"}]}
		}
	}`)
	op := newTestOptions()
	results, err := extractRootlyIncident(raw, op)
	require.NoError(t, err)
	require.Len(t, results, 2, "one incident + one deduped user")

	incident := results[0].(*models.Incident)
	assert.Equal(t, "usr_100", incident.CreatorUserId)
	assert.Equal(t, "usr_100", incident.ResolvedByUserId)

	users := collectUsers(results)
	require.Len(t, users, 1)
	assert.Equal(t, "usr_100", users[0].Id)
	assert.Equal(t, "Solo Operator", users[0].Name)
}

func TestExtractRootlyIncident_UserNamePreference(t *testing.T) {
	raw := []byte(`{
		"id": "inc_names",
		"type": "incidents",
		"attributes": {
			"sequential_id": 48,
			"title": "name preference",
			"status": "started",
			"started_at": "2026-05-10T17:00:00Z",
			"updated_at": "2026-05-10T17:05:00Z",
			"user":        {"data": {"id": "usr_full",  "type": "users", "attributes": {"full_name": "Full Name",  "name": "Ignored", "email": "ignored@example.com"}}},
			"started_by":  {"data": {"id": "usr_short", "type": "users", "attributes": {"name": "Short Name",      "email": "ignored@example.com"}}},
			"resolved_by": {"data": {"id": "usr_mail",  "type": "users", "attributes": {"email": "fallback@example.com"}}}
		},
		"relationships": {
			"services": {"data": [{"id": "svc_02", "type": "services"}]}
		}
	}`)
	op := newTestOptions()
	results, err := extractRootlyIncident(raw, op)
	require.NoError(t, err)

	users := collectUsers(results)
	require.Len(t, users, 3)
	byId := map[string]*models.User{}
	for _, u := range users {
		byId[u.Id] = u
	}
	require.Contains(t, byId, "usr_full")
	require.Contains(t, byId, "usr_short")
	require.Contains(t, byId, "usr_mail")
	assert.Equal(t, "Full Name", byId["usr_full"].Name)
	assert.Equal(t, "Short Name", byId["usr_short"].Name)
	assert.Equal(t, "fallback@example.com", byId["usr_mail"].Name)
}

func TestExtractRootlyIncident_WrongServiceSkipped(t *testing.T) {
	raw := []byte(`{
		"id": "inc_wrong_svc",
		"type": "incidents",
		"attributes": {
			"sequential_id": 49,
			"title": "other service",
			"status": "started",
			"started_at": "2026-05-10T18:00:00Z",
			"updated_at": "2026-05-10T18:05:00Z"
		},
		"relationships": {
			"services": {"data": [{"id": "svc_99", "type": "services"}]}
		}
	}`)
	op := newTestOptions()
	results, err := extractRootlyIncident(raw, op)
	require.NoError(t, err)
	assert.Empty(t, results, "incident for unrelated service should produce no rows")
}

func TestExtractRootlyIncident_EmptyServicesAccepted(t *testing.T) {
	raw := []byte(`{
		"id": "inc_no_svc",
		"type": "incidents",
		"attributes": {
			"sequential_id": 50,
			"title": "services omitted",
			"status": "started",
			"started_at": "2026-05-10T19:00:00Z",
			"updated_at": "2026-05-10T19:05:00Z"
		}
	}`)
	op := newTestOptions()
	results, err := extractRootlyIncident(raw, op)
	require.NoError(t, err)
	require.Len(t, results, 1)
	incident := results[0].(*models.Incident)
	assert.Equal(t, "svc_02", incident.ServiceId)
}

func TestExtractRootlyIncident_MissingStartedAtReturnsError(t *testing.T) {
	raw := []byte(`{
		"id": "inc_bad",
		"type": "incidents",
		"attributes": {
			"sequential_id": 51,
			"title": "bad row",
			"status": "started",
			"updated_at": "2026-05-10T20:05:00Z"
		},
		"relationships": {
			"services": {"data": [{"id": "svc_02", "type": "services"}]}
		}
	}`)
	op := newTestOptions()
	_, err := extractRootlyIncident(raw, op)
	assert.Error(t, err)
}

func TestExtractRootlyIncident_MissingSequentialId(t *testing.T) {
	raw := []byte(`{
		"id": "inc_no_num",
		"type": "incidents",
		"attributes": {
			"title": "no sequential id",
			"status": "started",
			"started_at": "2026-05-10T21:00:00Z",
			"updated_at": "2026-05-10T21:05:00Z"
		},
		"relationships": {
			"services": {"data": [{"id": "svc_02", "type": "services"}]}
		}
	}`)
	op := newTestOptions()
	results, err := extractRootlyIncident(raw, op)
	require.NoError(t, err)
	require.Len(t, results, 1)
	incident := results[0].(*models.Incident)
	assert.Equal(t, 0, incident.Number)
}
