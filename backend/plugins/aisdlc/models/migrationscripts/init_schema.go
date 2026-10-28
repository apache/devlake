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

package migrationscripts

import (
	"time"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/migrationhelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

var _ plugin.MigrationScript = (*initSchema)(nil)

type initSchema struct{}

// Snapshot of the tool tables at version 20261020000001.
// These types stay here so a later model change cannot rewrite this migration.

// AisdlcNoPK20261020 is exported so GORM migrates its fields. An unexported
// embed is ignored, and created_at plus the _raw_data_* columns are never created.
type AisdlcNoPK20261020 struct {
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	RawDataParams string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
}

type aisdlcConnection20261020 struct {
	helper.BaseConnection `mapstructure:",squash"`
	GitlabConnectionId    uint64 `gorm:"column:gitlab_connection_id;not null"`
}

func (aisdlcConnection20261020) TableName() string {
	return "_tool_aisdlc_connections"
}

type aisdlcScope20261020 struct {
	AisdlcNoPK20261020
	ConnectionId  uint64 `json:"connectionId" gorm:"primaryKey"`
	ScopeConfigId uint64 `json:"scopeConfigId,omitempty"`
	FullName      string `gorm:"primaryKey;type:varchar(255)"`
	Name          string `gorm:"type:varchar(255)"`
}

func (aisdlcScope20261020) TableName() string {
	return "_tool_aisdlc_scopes"
}

type aisdlcScopeConfig20261020 struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Entities     []string  `gorm:"type:json;serializer:json" json:"entities"`
	ConnectionId uint64    `json:"connectionId" gorm:"index"`
	Name         string    `json:"name" gorm:"type:varchar(255);uniqueIndex"`
	TenantId     string    `gorm:"type:varchar(255)"`
	SourceId     string    `gorm:"type:varchar(255)"`
	StationId    string    `gorm:"type:varchar(255)"`
	DisplayName  string    `gorm:"type:varchar(255)"`
	Parser       string    `gorm:"type:varchar(64)"`
	Branch       string    `gorm:"type:varchar(255)"`
	PathPrefix   string    `gorm:"type:varchar(1024)"`
}

func (aisdlcScopeConfig20261020) TableName() string {
	return "_tool_aisdlc_scope_configs"
}

type aisdlcRawFile20261020 struct {
	AisdlcNoPK20261020
	Id           string `gorm:"primaryKey;type:varchar(64)"`
	ConnectionId uint64 `gorm:"primaryKey"`
	ProjectPath  string `gorm:"type:varchar(255);index"`
	FilePath     string `gorm:"type:varchar(1024)"`
	CommitSha    string `gorm:"type:varchar(64)"`
	BlobId       string `gorm:"type:varchar(64)"`
	Body         string `gorm:"type:longtext"`
	CollectedAt  time.Time
}

func (aisdlcRawFile20261020) TableName() string {
	return "_tool_aisdlc_raw_files"
}

type aisdlcCheckpoint20261020 struct {
	AisdlcNoPK20261020
	Id               string `gorm:"primaryKey;type:varchar(64)"`
	ConnectionId     uint64 `gorm:"primaryKey"`
	ProjectPath      string `gorm:"type:varchar(255)"`
	ExaminedRevision string `gorm:"type:varchar(64)"`
	Branch           string `gorm:"type:varchar(255)"`
	FileCount        int
	ErrorMessage     string `gorm:"type:text"`
	CollectedAt      time.Time
}

func (aisdlcCheckpoint20261020) TableName() string {
	return "_tool_aisdlc_checkpoints"
}

type aisdlcExecution20261020 struct {
	AisdlcNoPK20261020
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

func (aisdlcExecution20261020) TableName() string {
	return "_tool_aisdlc_executions"
}

type aisdlcWorkItem20261020 struct {
	AisdlcNoPK20261020
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

func (aisdlcWorkItem20261020) TableName() string {
	return "_tool_aisdlc_work_items"
}

type aisdlcObservation20261020 struct {
	AisdlcNoPK20261020
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

func (aisdlcObservation20261020) TableName() string {
	return "_tool_aisdlc_observations"
}

type aisdlcEvidence20261020 struct {
	AisdlcNoPK20261020
	Id            string `gorm:"primaryKey;type:varchar(128)"`
	ConnectionId  uint64 `gorm:"primaryKey"`
	ObservationId string `gorm:"type:varchar(64);index"`
	SourceId      string `gorm:"type:varchar(255)"`
	URI           string `gorm:"type:text"`
	Revision      string `gorm:"type:varchar(64)"`
	FieldSelector string `gorm:"type:varchar(255)"`
}

func (aisdlcEvidence20261020) TableName() string {
	return "_tool_aisdlc_evidence"
}

type aisdlcItemLink20261020 struct {
	AisdlcNoPK20261020
	Id             string `gorm:"primaryKey;type:varchar(128)"`
	ConnectionId   uint64 `gorm:"primaryKey"`
	ObservationId  string `gorm:"type:varchar(64);index"`
	Role           string `gorm:"type:varchar(32)"`
	Relation       string `gorm:"type:varchar(64)"`
	WorkItemId     string `gorm:"type:varchar(64)"`
	EvidenceStatus string `gorm:"type:varchar(32)"`
}

func (aisdlcItemLink20261020) TableName() string {
	return "_tool_aisdlc_item_links"
}

type aisdlcUnknown20261020 struct {
	AisdlcNoPK20261020
	Id             string `gorm:"primaryKey;type:varchar(128)"`
	ConnectionId   uint64 `gorm:"primaryKey"`
	ObservationId  string `gorm:"type:varchar(64);index"`
	Field          string `gorm:"type:varchar(255)"`
	Reason         string `gorm:"type:text"`
	EvidenceNeeded string `gorm:"type:text"`
}

func (aisdlcUnknown20261020) TableName() string {
	return "_tool_aisdlc_unknowns"
}

type aisdlcTime20261020 struct {
	AisdlcNoPK20261020
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

func (aisdlcTime20261020) TableName() string {
	return "_tool_aisdlc_times"
}

func (script *initSchema) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&aisdlcConnection20261020{},
		&aisdlcScope20261020{},
		&aisdlcScopeConfig20261020{},
		&aisdlcRawFile20261020{},
		&aisdlcCheckpoint20261020{},
		&aisdlcExecution20261020{},
		&aisdlcWorkItem20261020{},
		&aisdlcObservation20261020{},
		&aisdlcEvidence20261020{},
		&aisdlcItemLink20261020{},
		&aisdlcUnknown20261020{},
		&aisdlcTime20261020{},
	)
}

func (script *initSchema) Version() uint64 {
	return 20261020000001
}

func (script *initSchema) Name() string {
	return "aisdlc init schema"
}
