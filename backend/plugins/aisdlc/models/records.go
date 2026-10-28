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

package models

import (
	"time"

	"github.com/apache/devlake/core/models/common"
)

// AisdlcRawFile is one artifact blob. It is not a station observation.
type AisdlcRawFile struct {
	common.NoPKModel
	Id           string `gorm:"primaryKey;type:varchar(64)"`
	ConnectionId uint64 `gorm:"primaryKey"`
	ProjectPath  string `gorm:"type:varchar(255);index"`
	FilePath     string `gorm:"type:varchar(1024)"`
	CommitSha    string `gorm:"type:varchar(64)"`
	BlobId       string `gorm:"type:varchar(64)"`
	Body         string `gorm:"type:longtext"`
	CollectedAt  time.Time
}

func (AisdlcRawFile) TableName() string { return "_tool_aisdlc_raw_files" }

// AisdlcCheckpoint records collection progress separately from station results.
type AisdlcCheckpoint struct {
	common.NoPKModel
	Id               string `gorm:"primaryKey;type:varchar(64)"`
	ConnectionId     uint64 `gorm:"primaryKey"`
	ProjectPath      string `gorm:"type:varchar(255)"`
	ExaminedRevision string `gorm:"type:varchar(64)"`
	Branch           string `gorm:"type:varchar(255)"`
	FileCount        int
	ErrorMessage     string `gorm:"type:text"`
	CollectedAt      time.Time
}

func (AisdlcCheckpoint) TableName() string { return "_tool_aisdlc_checkpoints" }

// AisdlcExecution is a batch the source file itself identified.
type AisdlcExecution struct {
	common.NoPKModel
	Id               string `gorm:"primaryKey;type:varchar(64)"`
	ConnectionId     uint64 `gorm:"primaryKey"`
	SourceId         string `gorm:"type:varchar(255)"`
	StationId        string `gorm:"type:varchar(255)"`
	SourceRunId      string `gorm:"type:varchar(255)"`
	RunIdType        string `gorm:"type:varchar(64)"`
	PipelineIdStatus string `gorm:"type:varchar(32)"`
	ProjectPath      string `gorm:"type:varchar(255)"`
	BatchPath        string `gorm:"type:varchar(1024)"`
}

func (AisdlcExecution) TableName() string { return "_tool_aisdlc_executions" }

// AisdlcWorkItem is a source-qualified reference. DomainIssueId is set only
// when exactly one existing issues row matches the key.
type AisdlcWorkItem struct {
	common.NoPKModel
	Id             string `gorm:"primaryKey;type:varchar(64)"`
	ConnectionId   uint64 `gorm:"primaryKey"`
	SourceSystem   string `gorm:"type:varchar(32)"`
	SourceInstance string `gorm:"type:varchar(255)"`
	EntityKind     string `gorm:"type:varchar(32)"`
	ItemKey        string `gorm:"type:varchar(255);index"`
	IdStatus       string `gorm:"type:varchar(32)"`
	DomainIssueId  string `gorm:"type:varchar(255)"`
	MatchStatus    string `gorm:"type:varchar(32)"`
}

func (AisdlcWorkItem) TableName() string { return "_tool_aisdlc_work_items" }

// AisdlcObservation is one source-supported result about one work item.
type AisdlcObservation struct {
	common.NoPKModel
	Id              string `gorm:"primaryKey;type:varchar(64)"`
	ConnectionId    uint64 `gorm:"primaryKey"`
	SchemaVersion   string `gorm:"type:varchar(16)"`
	TenantId        string `gorm:"type:varchar(255)"`
	SourceId        string `gorm:"type:varchar(255)"`
	StationId       string `gorm:"type:varchar(255)"`
	ObservationKind string `gorm:"type:varchar(64);index"`
	WorkItemId      string `gorm:"type:varchar(64);index"`
	ExecutionId     string `gorm:"type:varchar(64)"`
	AttemptStatus   string `gorm:"type:varchar(32)"`
	RawResult       string `gorm:"type:varchar(255)"`
	AssessmentTotal *float64
	AssessmentScale *float64
	RawVerdict      string `gorm:"type:varchar(64)"`
	ProjectPath     string `gorm:"type:varchar(255)"`
	FilePath        string `gorm:"type:varchar(1024)"`
	BlobId          string `gorm:"type:varchar(64)"`
	CommitSha       string `gorm:"type:varchar(64)"`
	FieldSelector   string `gorm:"type:varchar(255)"`
	AdapterId       string `gorm:"type:varchar(64)"`
	AdapterVersion  string `gorm:"type:varchar(32)"`
	MappingVersion  string `gorm:"type:varchar(32)"`
	Completeness    string `gorm:"type:varchar(64)"`
	Extensions      string `gorm:"type:text"`
	CollectedAt     time.Time
}

func (AisdlcObservation) TableName() string { return "_tool_aisdlc_observations" }

// AisdlcEvidence points at the Git blob entry that supports an observation.
type AisdlcEvidence struct {
	common.NoPKModel
	Id            string `gorm:"primaryKey;type:varchar(128)"`
	ConnectionId  uint64 `gorm:"primaryKey"`
	ObservationId string `gorm:"type:varchar(64);index"`
	SourceId      string `gorm:"type:varchar(255)"`
	URI           string `gorm:"type:text"`
	Revision      string `gorm:"type:varchar(64)"`
	FieldSelector string `gorm:"type:varchar(255)"`
}

func (AisdlcEvidence) TableName() string { return "_tool_aisdlc_evidence" }

// AisdlcItemLink is an evidenced input or output reference.
type AisdlcItemLink struct {
	common.NoPKModel
	Id             string `gorm:"primaryKey;type:varchar(128)"`
	ConnectionId   uint64 `gorm:"primaryKey"`
	ObservationId  string `gorm:"type:varchar(64);index"`
	Role           string `gorm:"type:varchar(32)"`
	Relation       string `gorm:"type:varchar(64)"`
	WorkItemId     string `gorm:"type:varchar(64)"`
	EvidenceStatus string `gorm:"type:varchar(32)"`
}

func (AisdlcItemLink) TableName() string { return "_tool_aisdlc_item_links" }

// AisdlcUnknown records a missing value and what would resolve it.
type AisdlcUnknown struct {
	common.NoPKModel
	Id             string `gorm:"primaryKey;type:varchar(128)"`
	ConnectionId   uint64 `gorm:"primaryKey"`
	ObservationId  string `gorm:"type:varchar(64);index"`
	Field          string `gorm:"type:varchar(255)"`
	Reason         string `gorm:"type:text"`
	EvidenceNeeded string `gorm:"type:text"`
}

func (AisdlcUnknown) TableName() string { return "_tool_aisdlc_unknowns" }

// AisdlcTime keeps the source timestamp's precision and timezone status.
type AisdlcTime struct {
	common.NoPKModel
	Id             string  `gorm:"primaryKey;type:varchar(128)"`
	ConnectionId   uint64  `gorm:"primaryKey"`
	ObservationId  string  `gorm:"type:varchar(64);index"`
	TimeType       string  `gorm:"column:time_type;type:varchar(64)"`
	RawValue       string  `gorm:"type:varchar(255)"`
	Normalized     *string `gorm:"type:varchar(255)"`
	Precision      string  `gorm:"type:varchar(32)"`
	TimezoneStatus string  `gorm:"type:varchar(32)"`
	AppliesTo      string  `gorm:"type:varchar(32)"`
}

func (AisdlcTime) TableName() string { return "_tool_aisdlc_times" }
