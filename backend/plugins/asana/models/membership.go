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
)

// AsanaProjectMembership links users to projects with their role
type AsanaProjectMembership struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	ProjectGid   string `gorm:"primaryKey;type:varchar(255)"`
	UserGid      string `gorm:"primaryKey;type:varchar(255)"`
	Role         string `gorm:"type:varchar(32)"`
	common.NoPKModel
}

func (AsanaProjectMembership) TableName() string {
	return "_tool_asana_project_memberships"
}

// AsanaTeamMembership links users to teams
type AsanaTeamMembership struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	TeamGid      string `gorm:"primaryKey;type:varchar(255)"`
	UserGid      string `gorm:"primaryKey;type:varchar(255)"`
	IsGuest      bool   `json:"isGuest"`
	common.NoPKModel
}

func (AsanaTeamMembership) TableName() string {
	return "_tool_asana_team_memberships"
}
