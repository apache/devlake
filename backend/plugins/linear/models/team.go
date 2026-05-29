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
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.ToolLayerScope = (*LinearTeam)(nil)

// LinearTeam is the data-source scope for the Linear plugin. A Linear Team
// owns issues, cycles, workflow states and labels, mapping cleanly to a
// DevLake domain-layer ticket.Board.
type LinearTeam struct {
	common.Scope `mapstructure:",squash"`
	TeamId       string `json:"teamId" mapstructure:"teamId" gorm:"primaryKey;type:varchar(255)"`
	Name         string `json:"name" mapstructure:"name" gorm:"type:varchar(255)"`
	Key          string `json:"key" mapstructure:"key" gorm:"type:varchar(255)"`
	Description  string `json:"description" mapstructure:"description"`
}

func (t LinearTeam) ScopeId() string {
	return t.TeamId
}

func (t LinearTeam) ScopeName() string {
	return t.Name
}

func (t LinearTeam) ScopeFullName() string {
	return t.Name
}

func (t LinearTeam) ScopeParams() interface{} {
	return &LinearApiParams{
		ConnectionId: t.ConnectionId,
		TeamId:       t.TeamId,
	}
}

func (LinearTeam) TableName() string {
	return "_tool_linear_teams"
}

// LinearApiParams identifies the scope a raw row belongs to. It is stored in
// the `params` column of every _raw_linear_* table.
type LinearApiParams struct {
	ConnectionId uint64
	TeamId       string
}
