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

// compile-time interface check
var _ plugin.ToolLayerScope = (*CheckmarxoneProject)(nil)

type CheckmarxoneProject struct {
	common.Scope `mapstructure:",squash"`
	ProjectId    string `json:"projectId" gorm:"primaryKey;type:varchar(255)" mapstructure:"projectId"`
	Name         string `json:"name" gorm:"type:varchar(255)"`
	Description  string `json:"description" gorm:"type:text"`
}

func (CheckmarxoneProject) TableName() string {
	return "_tool_checkmarxone_projects"
}

func (p CheckmarxoneProject) ScopeId() string {
	return p.ProjectId
}

func (p CheckmarxoneProject) ScopeName() string {
	return p.Name
}

func (p CheckmarxoneProject) ScopeFullName() string {
	return p.Name
}

func (p CheckmarxoneProject) ScopeParams() interface{} {
	return CheckmarxoneApiParams{
		ConnectionId: p.ConnectionId,
		ProjectId:    p.ProjectId,
	}
}

// CheckmarxoneApiParams identifies a unique set of data for raw data storage
type CheckmarxoneApiParams struct {
	ConnectionId uint64 `json:"connectionId"`
	ProjectId    string `json:"projectId"`
}