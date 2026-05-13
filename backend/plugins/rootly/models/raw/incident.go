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
	"time"
)

// Incident is the JSON:API envelope for a Rootly incident as returned by
// GET /incidents. The top-level Id is the incident id; display fields
// live under Attributes. Role-bearing fields (user, *_by) and the
// severity field on Attributes are themselves JSON:API response
// envelopes nested on the incident's attributes. Service membership
// lives on the sibling Relationships block (JSON:API relationship
// data, id+type pointers only) and is used by the extractor's
// safety-net service-scope filter.
type Incident struct {
	Id            string                `json:"id"`
	Type          string                `json:"type"`
	Attributes    IncidentAttributes    `json:"attributes"`
	Relationships IncidentRelationships `json:"relationships"`
}

// IncidentRelationships is a narrow view of the JSON:API relationships
// block used only for the safety-net service-scope check in the
// extractor. Every relationship type other than services is ignored.
type IncidentRelationships struct {
	Services struct {
		Data []struct {
			Id   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	} `json:"services"`
}

// IncidentAttributes carries the display fields for a Rootly incident.
// Shapes here match an actual GET /v1/incidents response. Notable:
//
//   - `severity`, `user`, `started_by`, `mitigated_by`, `resolved_by`,
//     `closed_by` are each nullable JSON:API-envelope objects — the
//     inner record lives at `<field>.data.id` /
//     `<field>.data.attributes.*`.
//   - Service membership is NOT on attributes; it lives on the
//     Incident.Relationships.Services block as JSON:API id+type
//     pointers. Without `?include=services` the full service records
//     are not returned — but the relationship pointers alone are
//     enough for the extractor's safety-net scope filter.
//   - No `urgency` field exists on the incident resource.
type IncidentAttributes struct {
	SequentialId   *int       `json:"sequential_id"`
	Title          string     `json:"title"`
	Summary        *string    `json:"summary"`
	Url            *string    `json:"url"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	MitigatedAt    *time.Time `json:"mitigated_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Severity is a JSON:API-envelope nested object. Inner attributes
	// include `slug` (e.g. sev0, sev1) and `severity` (the domain-
	// normalized value: critical, high, medium, low).
	Severity *SeverityEnvelope `json:"severity"`

	// Role-bearing users. Each is a JSON:API-envelope nested object,
	// nullable. User is the incident creator.
	User        *UserEnvelope `json:"user"`
	StartedBy   *UserEnvelope `json:"started_by"`
	MitigatedBy *UserEnvelope `json:"mitigated_by"`
	ResolvedBy  *UserEnvelope `json:"resolved_by"`
	ClosedBy    *UserEnvelope `json:"closed_by"`
}

// SeverityEnvelope is a JSON:API response envelope for a severity
// resource as it appears nested on an incident's attributes.
type SeverityEnvelope struct {
	Data struct {
		Id         string             `json:"id"`
		Type       string             `json:"type"`
		Attributes SeverityAttributes `json:"attributes"`
	} `json:"data"`
}

// SeverityAttributes carries the inner severity display fields.
// `Slug` is the org-defined identifier (e.g. sev0, sev1). `Severity`
// is the domain-normalized bucket (critical, high, medium, low) that
// DevLake maps straight onto ticket.Issue.Priority.
type SeverityAttributes struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
}

// UserEnvelope is a JSON:API response envelope for a user resource as
// it appears nested on an incident's attributes.
type UserEnvelope struct {
	Data struct {
		Id         string         `json:"id"`
		Type       string         `json:"type"`
		Attributes UserAttributes `json:"attributes"`
	} `json:"data"`
}

// UserAttributes is the subset of the user resource DevLake cares
// about for incident role tracking.
type UserAttributes struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

