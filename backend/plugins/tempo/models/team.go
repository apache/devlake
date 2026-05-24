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
	"fmt"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"gorm.io/gorm"
)

var _ plugin.ToolLayerScope = (*TempoTeam)(nil)

// TempoTeam represents a team in Jira Tempo
type TempoTeam struct {
	common.Scope `mapstructure:",squash" gorm:"embedded"`
	TeamId       int64  `json:"teamId" mapstructure:"teamId" validate:"required" gorm:"primaryKey"`
	Id           int64  `json:"id" gorm:"-" mapstructure:"-"` // JS scope selector compatibility (mirrors TeamId)
	Key          string `json:"key" mapstructure:"key" gorm:"type:varchar(255)"`
	Name         string `json:"name" mapstructure:"name" gorm:"type:varchar(255)"`
	Summary      string `json:"summary" mapstructure:"summary" gorm:"type:varchar(255)"`
}

// AfterFind populates the virtual Id field after reading from DB
func (t *TempoTeam) AfterFind(_ *gorm.DB) error {
	t.Id = t.TeamId
	return nil
}

func (t TempoTeam) ScopeId() string {
	return fmt.Sprintf("%d", t.TeamId)
}

func (t TempoTeam) ScopeName() string {
	return t.Name
}

func (t TempoTeam) ScopeFullName() string {
	return fmt.Sprintf("%s - %s", t.Key, t.Name)
}

func (t TempoTeam) ScopeParams() interface{} {
	return &TempoApiParams{
		ConnectionId: t.ConnectionId,
		TeamId:       t.TeamId,
	}
}

func (TempoTeam) TableName() string {
	return "_tool_tempo_teams"
}

// TempoApiParams holds the API parameters for Tempo teams
type TempoApiParams struct {
	ConnectionId uint64
	TeamId       int64
}

// TempoTeamResponse represents the API response for a team from Tempo API
type TempoTeamResponse struct {
	Id      int64  `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// ConvertToToolLayer converts the API response to the tool layer model
func (r TempoTeamResponse) ConvertToToolLayer(connectionId uint64) *TempoTeam {
	return &TempoTeam{
		Scope: common.Scope{
			ConnectionId: connectionId,
		},
		TeamId:  r.Id,
		Id:      r.Id,
		Key:     r.Key,
		Name:    r.Name,
		Summary: r.Summary,
	}
}
