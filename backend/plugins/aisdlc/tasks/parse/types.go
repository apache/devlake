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

package parse

import "time"

// Config is the station registration supplied by scope configuration.
type Config struct {
	TenantID    string
	SourceID    string
	StationID   string
	DisplayName string
	Parser      string
	ProjectPath string
	WebBase     string
}

// File is one collected artifact.
// CollectedAt is when collect last saw this blob in the tree, not when it was parsed.
type File struct {
	Path        string
	BlobID      string
	CommitSHA   string
	Body        string
	CollectedAt time.Time
}

// WorkItem is a source-qualified reference.
type WorkItem struct {
	ID             string
	SourceSystem   string
	SourceInstance string
	EntityKind     string
	ItemKey        string
	IdStatus       string
	MatchStatus    string
	DomainIssueID  string
}

// Execution is a batch the source file identified.
type Execution struct {
	ID               string
	SourceID         string
	StationID        string
	SourceRunID      string
	RunIDType        string
	PipelineIDStatus string
	BatchPath        string
}

// Evidence points at the blob entry behind a claim.
type Evidence struct {
	ID            string
	ObservationID string
	SourceID      string
	URI           string
	Revision      string
	FieldSelector string
}

// Link is an evidenced item relationship. It is not a causal handoff.
type Link struct {
	ID             string
	ObservationID  string
	Role           string
	Relation       string
	WorkItemID     string
	EvidenceStatus string
}

// Unknown explains a missing field.
type Unknown struct {
	ID             string
	ObservationID  string
	Field          string
	Reason         string
	EvidenceNeeded string
}

// Observation is one converted record.
type Observation struct {
	ID              string
	Kind            string
	WorkItemID      string
	SubjectKey      string
	ExecutionID     string
	RawResult       string
	AssessmentTotal *float64
	AssessmentScale *float64
	RawVerdict      string
	Path            string
	BlobID          string
	CommitSHA       string
	FieldSelector   string
	AdapterID       string
	Completeness    string
	Extensions      string
	CollectedAt     time.Time
	AttemptStatus   string
	Evidence        []Evidence
	Links           []Link
	Unknowns        []Unknown
	Times           []TimeEntry
}

// DraftSet is the parser output for one collection.
type DraftSet struct {
	Observations []Observation
	WorkItems    []WorkItem
	Executions   []Execution
	FileErrors   []string
}
