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

var _ plugin.ToolLayerScope = (*AsanaProject)(nil)

type AsanaProject struct {
	common.Scope `mapstructure:",squash"`
	Gid          string `json:"gid" mapstructure:"gid" gorm:"type:varchar(255);primaryKey"`
	Name         string `json:"name" mapstructure:"name" gorm:"type:varchar(255)"`
	ResourceType string `json:"resourceType" mapstructure:"resourceType" gorm:"type:varchar(32)"`
	Archived     bool   `json:"archived" mapstructure:"archived"`
	WorkspaceGid string `json:"workspaceGid" mapstructure:"workspaceGid" gorm:"type:varchar(255)"`
	PermalinkUrl string `json:"permalinkUrl" mapstructure:"permalinkUrl" gorm:"type:varchar(512)"`
}

func (p AsanaProject) ScopeId() string {
	return p.Gid
}

func (p AsanaProject) ScopeName() string {
	return p.Name
}

func (p AsanaProject) ScopeFullName() string {
	return p.Name
}

func (p AsanaProject) ScopeParams() interface{} {
	return &AsanaApiParams{
		ConnectionId: p.ConnectionId,
		ProjectId:    p.Gid,
	}
}

func (AsanaProject) TableName() string {
	return "_tool_asana_projects"
}

type AsanaApiParams struct {
	ConnectionId uint64
	ProjectId    string
}
