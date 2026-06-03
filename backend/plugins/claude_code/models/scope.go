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
	"strings"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"gorm.io/gorm"
)

// ClaudeCodeScope represents an organization-level collection scope.
type ClaudeCodeScope struct {
	common.Scope `mapstructure:",squash"`
	Id           string `json:"id" mapstructure:"id" gorm:"primaryKey;type:varchar(255)"`
	Organization string `json:"organization" mapstructure:"organization" gorm:"type:varchar(255)"`
	Name         string `json:"name" mapstructure:"name" gorm:"type:varchar(255)"`
	FullName     string `json:"fullName" mapstructure:"fullName" gorm:"type:varchar(255)"`
}

func (ClaudeCodeScope) TableName() string {
	return "_tool_claude_code_scopes"
}

func (s *ClaudeCodeScope) BeforeSave(tx *gorm.DB) error {
	if s == nil {
		return nil
	}

	s.Id = strings.TrimSpace(s.Id)
	s.Organization = strings.TrimSpace(s.Organization)
	s.Name = strings.TrimSpace(s.Name)
	s.FullName = strings.TrimSpace(s.FullName)

	if s.Organization == "" {
		s.Organization = s.Id
	}
	if s.Id == "" {
		s.Id = s.Organization
	}
	if s.Name == "" {
		s.Name = s.ScopeName()
	}
	if s.FullName == "" {
		s.FullName = s.ScopeFullName()
	}

	return nil

}

func (s ClaudeCodeScope) ScopeId() string {
	return s.Id
}

func (s ClaudeCodeScope) ScopeName() string {
	if s.Name != "" {
		return s.Name
	}
	if s.Organization != "" {
		return s.Organization
	}
	return s.Id
}

func (s ClaudeCodeScope) ScopeFullName() string {
	if s.FullName != "" {
		return s.FullName
	}
	return s.ScopeName()
}

func (s ClaudeCodeScope) ScopeParams() interface{} {
	return &ClaudeCodeScopeParams{
		ConnectionId: s.ConnectionId,
		ScopeId:      s.Id,
	}
}

// ClaudeCodeScopeParams is returned for blueprint configuration.
type ClaudeCodeScopeParams struct {
	ConnectionId uint64 `json:"connectionId"`
	ScopeId      string `json:"scopeId"`
}

var _ plugin.ToolLayerScope = (*ClaudeCodeScope)(nil)
