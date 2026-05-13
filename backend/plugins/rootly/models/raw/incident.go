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

type Incident struct {
	Id            string                `json:"id"`
	Type          string                `json:"type"`
	Attributes    IncidentAttributes    `json:"attributes"`
	Relationships IncidentRelationships `json:"relationships"`
}

type IncidentRelationships struct {
	Services struct {
		Data []struct {
			Id   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	} `json:"services"`
}

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

	Severity *SeverityEnvelope `json:"severity"`

	User        *UserEnvelope `json:"user"`
	StartedBy   *UserEnvelope `json:"started_by"`
	MitigatedBy *UserEnvelope `json:"mitigated_by"`
	ResolvedBy  *UserEnvelope `json:"resolved_by"`
	ClosedBy    *UserEnvelope `json:"closed_by"`
}

type SeverityEnvelope struct {
	Data struct {
		Id         string             `json:"id"`
		Type       string             `json:"type"`
		Attributes SeverityAttributes `json:"attributes"`
	} `json:"data"`
}

type SeverityAttributes struct {
	Slug     string `json:"slug"`     // org-defined (sev0, sev1, ...)
	Name     string `json:"name"`     // display name
	Severity string `json:"severity"` // critical, high, medium, low
}

type UserEnvelope struct {
	Data struct {
		Id         string         `json:"id"`
		Type       string         `json:"type"`
		Attributes UserAttributes `json:"attributes"`
	} `json:"data"`
}

type UserAttributes struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}
