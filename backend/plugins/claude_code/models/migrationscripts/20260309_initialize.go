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
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

// addClaudeCodeInitialTables creates the initial Claude Code tool-layer tables.
type addClaudeCodeInitialTables struct{}

func (script *addClaudeCodeInitialTables) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&claudeCodeConnection20260309{},
		&claudeCodeScope20260309{},
		&claudeCodeScopeConfig20260309{},
	)
}

type claudeCodeConnection20260309 struct {
	archived.Model
	Name             string `gorm:"type:varchar(100);uniqueIndex" json:"name"`
	Endpoint         string `gorm:"type:varchar(255)" json:"endpoint"`
	Proxy            string `gorm:"type:varchar(255)" json:"proxy"`
	RateLimitPerHour int    `json:"rateLimitPerHour"`
	Token            string `json:"token"`
	Organization     string `gorm:"type:varchar(255)" json:"organization"`
}

func (claudeCodeConnection20260309) TableName() string {
	return "_tool_claude_code_connections"
}

type claudeCodeScope20260309 struct {
	archived.NoPKModel
	ConnectionId  uint64 `json:"connectionId" gorm:"primaryKey"`
	ScopeConfigId uint64 `json:"scopeConfigId,omitempty"`
	Id            string `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Organization  string `json:"organization" gorm:"type:varchar(255)"`
	Name          string `json:"name" gorm:"type:varchar(255)"`
	FullName      string `json:"fullName" gorm:"type:varchar(255)"`
}

func (claudeCodeScope20260309) TableName() string {
	return "_tool_claude_code_scopes"
}

type claudeCodeScopeConfig20260309 struct {
	archived.Model
	Entities     []string `gorm:"type:json;serializer:json" json:"entities" mapstructure:"entities"`
	ConnectionId uint64   `json:"connectionId" gorm:"index" validate:"required" mapstructure:"connectionId,omitempty"`
	Name         string   `mapstructure:"name" json:"name" gorm:"type:varchar(255);uniqueIndex" validate:"required"`
}

func (claudeCodeScopeConfig20260309) TableName() string {
	return "_tool_claude_code_scope_configs"
}

func (*addClaudeCodeInitialTables) Version() uint64 {
	return 20260309000000
}

func (*addClaudeCodeInitialTables) Name() string {
	return "claude-code init tables"
}
