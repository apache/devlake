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
	"encoding/json"

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
)

// AisdlcScope is one GitLab project path, such as redhat/rhel-ai/agentic-ci/rfe-assess-data.
type AisdlcScope struct {
	common.Scope `mapstructure:",squash"`
	FullName     string `gorm:"primaryKey;type:varchar(255)" json:"fullName" mapstructure:"fullName" validate:"required"`
	Name         string `gorm:"type:varchar(255)" json:"name" mapstructure:"name"`
	Id           string `gorm:"-" json:"id" mapstructure:"-"`
}

func (s AisdlcScope) MarshalJSON() ([]byte, error) {
	type Alias AisdlcScope
	alias := Alias(s)
	alias.Id = s.FullName
	return json.Marshal(alias)
}

func (AisdlcScope) TableName() string {
	return "_tool_aisdlc_scopes"
}

func (s AisdlcScope) ScopeId() string {
	return s.FullName
}

func (s AisdlcScope) ScopeName() string {
	if s.Name != "" {
		return s.Name
	}
	return s.FullName
}

func (s AisdlcScope) ScopeFullName() string {
	return s.FullName
}

func (s AisdlcScope) ScopeParams() interface{} {
	return &AisdlcApiParams{
		ConnectionId: s.ConnectionId,
		FullName:     s.FullName,
	}
}

// AisdlcApiParams identifies a scope inside a pipeline.
type AisdlcApiParams struct {
	ConnectionId uint64 `json:"connectionId"`
	FullName     string `json:"fullName"`
}

var _ plugin.ToolLayerScope = (*AisdlcScope)(nil)
