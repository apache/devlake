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

type agentReadyConnection20260604 struct {
	helper.BaseConnection `mapstructure:",squash"`
	Project               string `gorm:"column:project;type:varchar(200)"`
	GitHubConnectionId    uint64 `gorm:"column:github_connection_id;not null"`
	SubmissionsRepo       string `gorm:"column:submissions_repo;type:varchar(255);not null"`
	SubmissionsPath       string `gorm:"column:submissions_path;type:varchar(255);default:submissions"`
	Branch                string `gorm:"column:branch;type:varchar(255)"`
}

func (agentReadyConnection20260604) TableName() string {
	return "_tool_agentready_connections"
}

type agentReadyScope20260604 struct {
	// common.NoPKModel fields
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// common.RawDataOrigin fields
	RawDataParams string `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64 `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// common.Scope fields
	ConnectionId  uint64 `json:"connectionId" gorm:"primaryKey"`
	ScopeConfigId uint64 `json:"scopeConfigId,omitempty"`
	// agentready-specific fields
	FullName string `gorm:"primaryKey;type:varchar(255)"`
	Name     string `gorm:"type:varchar(255)"`
}

func (agentReadyScope20260604) TableName() string {
	return "_tool_agentready_scopes"
}

type agentReadyScopeConfig20260604 struct {
	// common.Model fields
	ID        uint64    `gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// common.ScopeConfig fields
	Entities     []string `gorm:"type:json;serializer:json" json:"entities"`
	ConnectionId uint64   `json:"connectionId" gorm:"index"`
	Name         string   `gorm:"type:varchar(255);uniqueIndex" json:"name"`
}

func (agentReadyScopeConfig20260604) TableName() string {
	return "_tool_agentready_scope_configs"
}

type agentReadyAssessment20260604 struct {
	// common.NoPKModel fields
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	RawDataParams      string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable       string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId          uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark      string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	Id                 string    `gorm:"primaryKey;type:varchar(255)"`
	RepoId             string    `gorm:"index;type:varchar(255)"`
	RepoName           string    `gorm:"type:varchar(255)"`
	ConnectionId       uint64    `gorm:"primaryKey"`
	Provider           string    `gorm:"type:varchar(50)"`
	SchemaVersion      string    `gorm:"type:varchar(20)"`
	OverallScore       float64   `gorm:"type:float"`
	CertificationLevel string    `gorm:"type:varchar(50)"`
	AttributesAssessed int       `gorm:"type:int"`
	AttributesTotal    int       `gorm:"type:int"`
	Branch             string    `gorm:"type:varchar(255)"`
	CommitHash         string    `gorm:"type:varchar(40)"`
	DurationSeconds    float64   `gorm:"type:float"`
	AssessedAt         time.Time `gorm:"index"`
	CollectedAt        time.Time
	RawJSON            string `gorm:"type:longtext"`
}

func (agentReadyAssessment20260604) TableName() string {
	return "_tool_agentready_assessments"
}

type agentReadyFinding20260604 struct {
	// common.NoPKModel fields
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	RawDataParams      string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable       string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId          uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark      string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	Id                 string    `gorm:"primaryKey;type:varchar(255)"`
	ConnectionId       uint64    `gorm:"primaryKey"`
	AssessmentId       string    `gorm:"index;type:varchar(255)"`
	RepoId             string    `gorm:"index;type:varchar(255)"`
	AttributeId        string    `gorm:"type:varchar(255)"`
	AttributeName      string    `gorm:"type:varchar(255)"`
	Category           string    `gorm:"type:varchar(255)"`
	Tier               int       `gorm:"type:int"`
	Status             string    `gorm:"type:varchar(50)"`
	Score              *float64  `gorm:"type:float"`
	MeasuredValue      string    `gorm:"type:text"`
	Threshold          string    `gorm:"type:text"`
	Evidence           string    `gorm:"type:text"`
	RemediationSummary string    `gorm:"type:text"`
	RemediationSteps   string    `gorm:"type:text"`
	DefaultWeight      float64   `gorm:"type:float"`
}

func (agentReadyFinding20260604) TableName() string {
	return "_tool_agentready_findings"
}

type agentReadyMetric20260604 struct {
	// common.NoPKModel fields
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	RawDataParams  string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable   string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId      uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark  string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	Id             string    `gorm:"primaryKey;type:varchar(255)"`
	ConnectionId   uint64    `gorm:"primaryKey"`
	RepoId         string    `gorm:"index;type:varchar(255)"`
	AssessedAt     time.Time `gorm:"index"`
	PassCount      int       `gorm:"type:int"`
	FailCount      int       `gorm:"type:int"`
	SkipCount      int       `gorm:"type:int"`
	Tier1PassRate  float64   `gorm:"type:float"`
	Tier2PassRate  float64   `gorm:"type:float"`
	Tier3PassRate  float64   `gorm:"type:float"`
	Tier4PassRate  float64   `gorm:"type:float"`
	CategoryScores string    `gorm:"type:text"`
}

func (agentReadyMetric20260604) TableName() string {
	return "_tool_agentready_metrics"
}

func (script *initSchema) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&agentReadyConnection20260604{},
		&agentReadyScope20260604{},
		&agentReadyScopeConfig20260604{},
		&agentReadyAssessment20260604{},
		&agentReadyFinding20260604{},
		&agentReadyMetric20260604{},
	)
}

func (script *initSchema) Version() uint64 {
	return 20260604000001
}

func (script *initSchema) Name() string {
	return "agentready init schema"
}
