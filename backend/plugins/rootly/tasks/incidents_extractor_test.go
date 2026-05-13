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

// Fixture shapes match a real GET /v1/incidents response:
//   - each incident is a JSON:API envelope {id, type, attributes, relationships}
//   - role-bearing users (user, started_by, …) live on attributes as
//     nested JSON:API envelopes: {"data": {"id": "…", "type": "users",
//     "attributes": {"name", "full_name", "email"}}}
//   - severity lives on attributes as a nested JSON:API envelope:
//     {"data": {"id": "…", "type": "severities", "attributes":
//     {"slug", "severity", "name"}}}
//   - service membership lives on the sibling relationships block as
//     plain JSON:API pointers: {"services": {"data": [{"id":"…","type":"services"}]}}
//     (Full service records are only returned when the caller passes
//     `?include=services`; we don't, so we only see pointer ids here.)

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

// collectUsers pulls the *models.User rows out of a heterogeneous result
// slice so individual tests can make assertions without worrying about
// the incident row's ordering.
func collectUsers(results []interface{}) []*models.User {
	users := []*models.User{}
	for _, r := range results {
		if u, ok := r.(*models.User); ok {
			users = append(users, u)
		}
	}
	return users
}

// TestExtractRootlyIncident_HappyPathActive covers the base case: a
// started incident with a creator user in attributes.user produces one
// Incident row (with CreatorUserId populated) and one User row.
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

// TestExtractRootlyIncident_Resolved verifies that a resolved incident
// populates AcknowledgedDate / MitigatedDate / ResolvedDate as non-nil
// pointers AND populates CreatorUserId + ResolvedByUserId from the
// nested JSON:API user envelopes. Both users are emitted as User rows.
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

// TestExtractRootlyIncident_MissingOptionalTimestamps asserts that
// missing mitigated_at and resolved_at yield nil pointers rather than
// zero-time values (which would pollute downstream DORA math).
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

// TestExtractRootlyIncident_NullSeverity covers the common case where
// an incident has no severity set: the Severity field on the tool row
// is empty string, not a panic or a "null" literal.
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

// TestExtractRootlyIncident_NoRolesFilled verifies that an incident
// with every role-bearing user field null produces exactly one result
// (the incident row) with all role user-id fields empty and zero User
// rows.
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

// TestExtractRootlyIncident_SameUserInMultipleRoles verifies the
// dedupe invariant: if one person is both the creator and the
// resolver, only one User row is emitted but BOTH role id fields on
// the incident point to that user.
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

// TestExtractRootlyIncident_UserNamePreference verifies the name
// preference order: FullName > Name > Email > empty string. Three
// users exercise the three fallbacks in a single incident.
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

// TestExtractRootlyIncident_WrongServiceSkipped asserts the safety-net
// scope filter: if the incident's relationships.services.data doesn't
// include the configured ServiceId, the extractor returns an empty
// slice and no error. Defense in depth against multi-service
// incidents leaking into the wrong scope even if the API-side
// filter[service_ids] query failed.
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

// TestExtractRootlyIncident_EmptyServicesAccepted covers the case
// where an incident has no services relationship (empty array or
// missing block entirely). We accept the incident — the API-side
// filter[service_ids] query is the only scoping signal — and tag it
// with op.ServiceId.
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

// TestExtractRootlyIncident_MissingStartedAtReturnsError covers the
// single required-field validation. A missing started_at would write
// a zero-time row, breaking downstream MTTR math silently. Fail loud.
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

// TestExtractRootlyIncident_MissingSequentialId verifies graceful
// degradation when the Rootly response omits the incident number.
// We want the row to still land in the tool table so downstream
// conversion can fall back to the string id.
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
