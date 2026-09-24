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

// Incident is the wire shape of a single object in
// IncidentsService.QueryIncidents'/GetIncident's `incident`/`incidents[]`
// field, verified live against a real Grafana Cloud stack (see
// grafana_irm_plan.md §3.2/§3.4/§9). Only fields the extractor actually maps
// are declared here; CreatedTime/ModifiedTime/ClosedTime are plain strings,
// not time.Time, because ClosedTime comes back as a literal empty string
// `""` on an open incident (verified live) rather than null or omitted.
type Incident struct {
	IncidentID         string             `json:"incidentID"`
	Title              string             `json:"title"`
	Status             string             `json:"status"`
	Severity           string             `json:"severity"`
	Labels             []IncidentLabel    `json:"labels"`
	CreatedTime        string             `json:"createdTime"`
	ModifiedTime       string             `json:"modifiedTime"`
	ClosedTime         string             `json:"closedTime"`
	OverviewURL        string             `json:"overviewURL"`
	IncidentMembership IncidentMembership `json:"incidentMembership"`
}

type IncidentLabel struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type IncidentMembership struct {
	Assignments []Assignment `json:"assignments"`
}

// Assignment is one entry of IncidentMembership.Assignments. On the wire
// this array is a fixed-size list of role *slots*, most of them empty
// placeholders with User.UserID == "" (verified live, see
// grafana_irm_plan.md §3.4/§9) — the extractor filters on that.
type Assignment struct {
	User UserPreview `json:"user"`
	Role Role        `json:"role"`
}

type UserPreview struct {
	UserID string `json:"userID"`
	Name   string `json:"name"`
}

type Role struct {
	Name string `json:"name"`
}
