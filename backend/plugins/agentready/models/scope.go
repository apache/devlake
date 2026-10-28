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

type AgentReadyScope struct {
	common.Scope `mapstructure:",squash"`
	FullName     string `gorm:"primaryKey;type:varchar(255)" json:"fullName" mapstructure:"fullName" validate:"required"`
	Name         string `gorm:"type:varchar(255)" json:"name" mapstructure:"name"`
	Id           string `gorm:"-" json:"id" mapstructure:"-"`
}

func (s AgentReadyScope) MarshalJSON() ([]byte, error) {
	type Alias AgentReadyScope
	alias := Alias(s)
	alias.Id = s.FullName
	return json.Marshal(alias)
}

func (AgentReadyScope) TableName() string {
	return "_tool_agentready_scopes"
}

func (s AgentReadyScope) ScopeId() string {
	return s.FullName
}

func (s AgentReadyScope) ScopeName() string {
	return s.Name
}

func (s AgentReadyScope) ScopeFullName() string {
	return s.FullName
}

func (s AgentReadyScope) ScopeParams() interface{} {
	return &AgentReadyApiParams{
		ConnectionId: s.ConnectionId,
		FullName:     s.FullName,
	}
}

type AgentReadyApiParams struct {
	ConnectionId uint64 `json:"connectionId"`
	FullName     string `json:"fullName"`
}

var _ plugin.ToolLayerScope = (*AgentReadyScope)(nil)
