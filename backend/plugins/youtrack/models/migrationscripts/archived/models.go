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

// Package archived holds frozen snapshots of the tool-layer models as they
// existed at each migration. The live models in plugins/youtrack/models may
// evolve; these snapshots keep historical migrations stable. They are built
// on core/models/migrationscripts/archived helpers and never import the live
// models package.
//
// ⚠ The archived helpers are NOT the live embeds: archived.ScopeConfig lacks
// ConnectionId and Name, and there is no archived.Scope at all — both
// shortfalls are unrolled by hand below (a missing hand-added column surfaces
// only as a TestMigrationSchemaMatchesModels failure).
package archived

import (
	"time"

	"github.com/apache/devlake/core/models/migrationscripts/archived"
)

// YoutrackConnection unrolls BaseConnection/RestConnection/AccessToken into
// plain fields (Token keeps the encdec serializer).
type YoutrackConnection struct {
	Name string `gorm:"type:varchar(100);uniqueIndex" json:"name"`
	archived.Model
	Endpoint         string `mapstructure:"endpoint" json:"endpoint"`
	Proxy            string `mapstructure:"proxy" json:"proxy"`
	RateLimitPerHour int    `json:"rateLimitPerHour"`
	Token            string `mapstructure:"token" json:"token" gorm:"serializer:encdec"`
}

func (YoutrackConnection) TableName() string { return "_tool_youtrack_connections" }

// YoutrackProject unrolls common.Scope by hand (there is no archived.Scope):
// archived.NoPKModel + ConnectionId primary key + ScopeConfigId.
type YoutrackProject struct {
	archived.NoPKModel
	ConnectionId  uint64 `json:"connectionId" gorm:"primaryKey"`
	ScopeConfigId uint64 `json:"scopeConfigId,omitempty"`
	Id            string `json:"id" gorm:"primaryKey;type:varchar(255)"`
	ShortName     string `json:"shortName" gorm:"type:varchar(255);index"`
	Name          string `json:"name" gorm:"type:varchar(255)"`
	Description   string `json:"description"`
	Archived      bool   `json:"archived"`
}

func (YoutrackProject) TableName() string { return "_tool_youtrack_projects" }

// YoutrackScopeConfig hand-adds ConnectionId and Name, which the live
// common.ScopeConfig carries but archived.ScopeConfig does not.
type YoutrackScopeConfig struct {
	archived.ScopeConfig
	ConnectionId    uint64            `json:"connectionId" gorm:"index"`
	Name            string            `gorm:"type:varchar(255);uniqueIndex" json:"name"`
	TypeField       string            `json:"typeField" gorm:"type:varchar(255)"`
	StateField      string            `json:"stateField" gorm:"type:varchar(255)"`
	PriorityField   string            `json:"priorityField" gorm:"type:varchar(255)"`
	AssigneeField   string            `json:"assigneeField" gorm:"type:varchar(255)"`
	StoryPointField string            `json:"storyPointField" gorm:"type:varchar(255)"`
	DueDateField    string            `json:"dueDateField" gorm:"type:varchar(255)"`
	TypeMappings    map[string]string `json:"typeMappings" gorm:"type:json;serializer:json"`
	StatusMappings  map[string]string `json:"statusMappings" gorm:"type:json;serializer:json"`
}

func (YoutrackScopeConfig) TableName() string { return "_tool_youtrack_scope_configs" }

type YoutrackAccount struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Id           string `gorm:"primaryKey;type:varchar(255)"`
	Login        string `gorm:"type:varchar(255)"`
	FullName     string `gorm:"type:varchar(255)"`
	Email        string `gorm:"type:varchar(255)"`
	AvatarUrl    string `gorm:"type:varchar(255)"`
	Guest        bool
	archived.NoPKModel
}

func (YoutrackAccount) TableName() string { return "_tool_youtrack_accounts" }

type YoutrackIssue struct {
	ConnectionId     uint64 `gorm:"primaryKey"`
	Id               string `gorm:"primaryKey;type:varchar(255)"`
	IdReadable       string `gorm:"type:varchar(255)"`
	NumberInProject  int64
	ProjectId        string `gorm:"index;type:varchar(255)"`
	Summary          string
	Description      string
	Created          time.Time
	Updated          time.Time `gorm:"index"`
	Resolved         *time.Time
	CommentsCount    int
	ReporterId       string `gorm:"type:varchar(255)"`
	ReporterName     string `gorm:"type:varchar(255)"`
	StateName        string `gorm:"type:varchar(255)"`
	StateIsResolved  bool
	TypeName         string `gorm:"type:varchar(255)"`
	PriorityName     string `gorm:"type:varchar(255)"`
	AssigneeId       string `gorm:"type:varchar(255)"`
	AssigneeName     string `gorm:"type:varchar(255)"`
	StoryPoint       *float64
	DueDate          *time.Time
	StdType          string `gorm:"type:varchar(255)"`
	StdStatus        string `gorm:"type:varchar(255)"`
	LeadTimeMinutes  *uint
	CustomFieldsJson string `gorm:"type:text"`
	archived.NoPKModel
}

func (YoutrackIssue) TableName() string { return "_tool_youtrack_issues" }

type YoutrackIssueComment struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Id           string `gorm:"primaryKey;type:varchar(255)"`
	IssueId      string `gorm:"index;type:varchar(255)"`
	Body         string
	AuthorId     string `gorm:"type:varchar(255)"`
	Created      time.Time
	Updated      *time.Time
	Deleted      bool
	archived.NoPKModel
}

func (YoutrackIssueComment) TableName() string { return "_tool_youtrack_issue_comments" }

type YoutrackIssueLabel struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	IssueId      string `gorm:"primaryKey;type:varchar(255)"`
	LabelName    string `gorm:"primaryKey;type:varchar(255)"`
	archived.NoPKModel
}

func (YoutrackIssueLabel) TableName() string { return "_tool_youtrack_issue_labels" }

type YoutrackWorkflowState struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	ProjectId    string `gorm:"primaryKey;type:varchar(255)"`
	Id           string `gorm:"primaryKey;type:varchar(255)"`
	Name         string `gorm:"type:varchar(255)"`
	IsResolved   bool
	Ordinal      int
	Archived     bool
	archived.NoPKModel
}

func (YoutrackWorkflowState) TableName() string { return "_tool_youtrack_workflow_states" }

type YoutrackIssueChangelog struct {
	ConnectionId      uint64 `gorm:"primaryKey"`
	Id                string `gorm:"primaryKey;type:varchar(255)"`
	IssueId           string `gorm:"index;type:varchar(255)"`
	Category          string `gorm:"type:varchar(100)"`
	FieldName         string `gorm:"type:varchar(255)"`
	AuthorId          string `gorm:"type:varchar(255)"`
	AuthorName        string `gorm:"type:varchar(255)"`
	FromValue         string
	ToValue           string
	OriginalFromValue string
	OriginalToValue   string
	Created           time.Time `gorm:"index"`
	archived.NoPKModel
}

func (YoutrackIssueChangelog) TableName() string { return "_tool_youtrack_issue_changelogs" }
