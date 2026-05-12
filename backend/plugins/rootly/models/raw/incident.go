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

package raw

import (
	"encoding/json"
	"time"
)

// Incident is the JSON:API envelope for a Rootly incident as returned by
// GET /incidents. The top-level Id is the incident id; display fields live
// under Attributes, and Relationships carries cross-entity references
// (services, users) that we may or may not consult during extraction.
type Incident struct {
	Id            string             `json:"id"`
	Type          string             `json:"type"`
	Attributes    IncidentAttributes `json:"attributes"`
	Relationships json.RawMessage    `json:"relationships"`
}

// IncidentAttributes carries the display fields for a Rootly incident.
//
// The severity response shape is defensive: Rootly may return severity
// as a flat slug string or as a nested object; we accept either. The
// extractor prefers SeverityObj.Slug when non-nil, otherwise falls back
// to SeveritySlug, otherwise empty string.
//
// Role-bearing user objects (User, StartedBy, MitigatedBy, ResolvedBy,
// ClosedBy) are nested user records inlined directly on the incident —
// NOT JSON:API-wrapped and NOT surfaced through a relationships
// `included` array. Any of them may be nil if the role was not filled.
type IncidentAttributes struct {
	SequentialId   *int           `json:"sequential_id"`
	Title          string         `json:"title"`
	Summary        *string        `json:"summary"`
	Url            *string        `json:"url"`
	Status         string         `json:"status"`
	SeveritySlug   *string        `json:"severity"`
	SeverityObj    *SeverityAttrs `json:"severity_attributes"`
	Urgency        *string        `json:"urgency"`
	StartedAt      time.Time      `json:"started_at"`
	AcknowledgedAt *time.Time     `json:"acknowledged_at"`
	MitigatedAt    *time.Time     `json:"mitigated_at"`
	ResolvedAt     *time.Time     `json:"resolved_at"`
	UpdatedAt      time.Time      `json:"updated_at"`

	// Role-bearing user objects. Each is a nullable nested user record.
	// User is the incident creator; the others track lifecycle actors.
	User        *NestedUser `json:"user"`
	StartedBy   *NestedUser `json:"started_by"`
	MitigatedBy *NestedUser `json:"mitigated_by"`
	ResolvedBy  *NestedUser `json:"resolved_by"`
	ClosedBy    *NestedUser `json:"closed_by"`
}

type SeverityAttrs struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// NestedUser is the shape of a user object as inlined on an incident's
// attributes. Plain JSON — NOT a JSON:API envelope. Rootly exposes the
// display name under either `name` or `full_name` depending on endpoint
// shape; the extractor prefers FullName when non-empty.
type NestedUser struct {
	Id       string  `json:"id"`
	Email    *string `json:"email"`
	Name     *string `json:"name"`
	FullName *string `json:"full_name"`
	Url      *string `json:"url"`
}

// IncidentRelationships is a narrow view of the JSON:API relationships
// envelope used only for the safety-net service-scope check. It ignores
// every relationship type except services.
type IncidentRelationships struct {
	Services struct {
		Data []struct {
			Id   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	} `json:"services"`
}
