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
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
)

var _ plugin.ToolLayerScope = (*YoutrackProject)(nil)

// YoutrackProject is the data-source scope for the YouTrack plugin. A YouTrack
// project maps to a domain-layer ticket.Board. The primary key is the
// internal project id (`0-12`) so domain board ids stay stable across
// renames; `shortName` (`PROJ`) is a stored, indexed column used for the
// `project: {}` query and all user-facing display.
type YoutrackProject struct {
	common.Scope `mapstructure:",squash"`
	Id           string `json:"id" mapstructure:"id" gorm:"primaryKey;type:varchar(255)"`
	ShortName    string `json:"shortName" mapstructure:"shortName" gorm:"type:varchar(255);index"`
	Name         string `json:"name" mapstructure:"name" gorm:"type:varchar(255)"`
	Description  string `json:"description" mapstructure:"description"`
	Archived     bool   `json:"archived" mapstructure:"archived"`
}

func (p YoutrackProject) ScopeId() string {
	return p.Id
}

func (p YoutrackProject) ScopeName() string {
	return p.Name
}

func (p YoutrackProject) ScopeFullName() string {
	return p.Name
}

func (p YoutrackProject) ScopeParams() interface{} {
	return &YoutrackApiParams{
		ConnectionId: p.ConnectionId,
		ProjectId:    p.Id,
	}
}

func (YoutrackProject) TableName() string {
	return "_tool_youtrack_projects"
}

// YoutrackApiParams identifies the scope a raw row belongs to. It is stored in
// the `params` column of every _raw_youtrack_* table.
type YoutrackApiParams struct {
	ConnectionId uint64
	ProjectId    string
}
