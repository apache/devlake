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
// existed at each migration. The live models in plugins/linear/models may
// evolve; these snapshots keep historical migrations stable.
package archived

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
)

type LinearConnection struct {
	Name string `gorm:"type:varchar(100);uniqueIndex" json:"name"`
	archived.Model
	Endpoint         string `mapstructure:"endpoint" json:"endpoint"`
	Proxy            string `mapstructure:"proxy" json:"proxy"`
	RateLimitPerHour int    `json:"rateLimitPerHour"`
	Token            string `mapstructure:"token" json:"token" gorm:"serializer:encdec"`
}

func (LinearConnection) TableName() string { return "_tool_linear_connections" }

type LinearTeam struct {
	archived.NoPKModel
	ConnectionId  uint64 `json:"connectionId" gorm:"primaryKey"`
	ScopeConfigId uint64 `json:"scopeConfigId,omitempty"`
	TeamId        string `json:"teamId" gorm:"primaryKey;type:varchar(255)"`
	Name          string `json:"name" gorm:"type:varchar(255)"`
	Key           string `json:"key" gorm:"type:varchar(255)"`
	Description   string `json:"description"`
}

func (LinearTeam) TableName() string { return "_tool_linear_teams" }

type LinearScopeConfig struct {
	archived.ScopeConfig
	ConnectionId uint64 `json:"connectionId" gorm:"index"`
	Name         string `gorm:"type:varchar(255);uniqueIndex" json:"name"`
}

func (LinearScopeConfig) TableName() string { return "_tool_linear_scope_configs" }

type LinearAccount struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Id           string `gorm:"primaryKey;type:varchar(255)"`
	Name         string `gorm:"type:varchar(255)"`
	DisplayName  string `gorm:"type:varchar(255)"`
	Email        string `gorm:"type:varchar(255)"`
	AvatarUrl    string `gorm:"type:varchar(255)"`
	Active       bool
	archived.NoPKModel
}

func (LinearAccount) TableName() string { return "_tool_linear_accounts" }

type LinearIssue struct {
	ConnectionId  uint64 `gorm:"primaryKey"`
	Id            string `gorm:"primaryKey;type:varchar(255)"`
	TeamId        string `gorm:"index;type:varchar(255)"`
	Identifier    string `gorm:"type:varchar(255)"`
	Number        int
	Title         string `gorm:"type:varchar(255)"`
	Description   string
	Url           string `gorm:"type:varchar(255)"`
	Priority      int
	PriorityLabel string `gorm:"type:varchar(100)"`
	Estimate      *float64
	StateId       string `gorm:"index;type:varchar(255)"`
	StateName     string `gorm:"type:varchar(255)"`
	StateType     string `gorm:"type:varchar(100)"`
	CreatorId     string `gorm:"type:varchar(255)"`
	AssigneeId    string `gorm:"type:varchar(255)"`
	CycleId       string `gorm:"index;type:varchar(255)"`
	ParentId      string `gorm:"type:varchar(255)"`
	CreatedAt     time.Time
	UpdatedAt     time.Time `gorm:"index"`
	StartedAt     *time.Time
	CompletedAt   *time.Time
	CanceledAt    *time.Time
	archived.NoPKModel
}

func (LinearIssue) TableName() string { return "_tool_linear_issues" }

type LinearComment struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Id           string `gorm:"primaryKey;type:varchar(255)"`
	IssueId      string `gorm:"index;type:varchar(255)"`
	Body         string
	AuthorId     string `gorm:"type:varchar(255)"`
	CreatedAt    time.Time
	UpdatedAt    time.Time `gorm:"index"`
	archived.NoPKModel
}

func (LinearComment) TableName() string { return "_tool_linear_comments" }

type LinearIssueLabel struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	IssueId      string `gorm:"primaryKey;type:varchar(255)"`
	LabelName    string `gorm:"primaryKey;type:varchar(255)"`
	archived.NoPKModel
}

func (LinearIssueLabel) TableName() string { return "_tool_linear_issue_labels" }

type LinearWorkflowState struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Id           string `gorm:"primaryKey;type:varchar(255)"`
	TeamId       string `gorm:"index;type:varchar(255)"`
	Name         string `gorm:"type:varchar(255)"`
	Type         string `gorm:"type:varchar(100)"`
	Color        string `gorm:"type:varchar(50)"`
	Position     float64
	archived.NoPKModel
}

func (LinearWorkflowState) TableName() string { return "_tool_linear_workflow_states" }

type LinearCycle struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Id           string `gorm:"primaryKey;type:varchar(255)"`
	TeamId       string `gorm:"index;type:varchar(255)"`
	Number       int
	Name         string `gorm:"type:varchar(255)"`
	StartsAt     *time.Time
	EndsAt       *time.Time
	CompletedAt  *time.Time
	archived.NoPKModel
}

func (LinearCycle) TableName() string { return "_tool_linear_cycles" }

type LinearIssueHistory struct {
	ConnectionId  uint64    `gorm:"primaryKey"`
	Id            string    `gorm:"primaryKey;type:varchar(255)"`
	IssueId       string    `gorm:"index;type:varchar(255)"`
	ActorId       string    `gorm:"type:varchar(255)"`
	FromStateId   string    `gorm:"type:varchar(255)"`
	FromStateName string    `gorm:"type:varchar(255)"`
	FromStateType string    `gorm:"type:varchar(100)"`
	ToStateId     string    `gorm:"type:varchar(255)"`
	ToStateName   string    `gorm:"type:varchar(255)"`
	ToStateType   string    `gorm:"type:varchar(100)"`
	CreatedAt     time.Time `gorm:"index"`
	archived.NoPKModel
}

func (LinearIssueHistory) TableName() string { return "_tool_linear_issue_history" }
