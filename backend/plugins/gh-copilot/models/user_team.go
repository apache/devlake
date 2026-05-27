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
	"time"

	"github.com/apache/incubator-devlake/core/models/common"
)

// GhCopilotUserTeam maps users to teams per day from the user-teams-1-day report.
// This enables team-level metrics aggregation by joining with per-user daily metrics.
type GhCopilotUserTeam struct {
	ConnectionId   uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId        string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day            time.Time `gorm:"primaryKey;type:date" json:"day"`
	UserId         int64     `gorm:"primaryKey" json:"userId"`
	TeamId         int64     `gorm:"primaryKey" json:"teamId"`

	UserLogin      string `json:"userLogin" gorm:"type:varchar(255);index"`
	OrganizationId string `json:"organizationId" gorm:"type:varchar(100)"`
	EnterpriseId   string `json:"enterpriseId" gorm:"type:varchar(100)"`
	TeamSlug       string `json:"teamSlug" gorm:"type:varchar(255)"`

	common.NoPKModel
}

func (GhCopilotUserTeam) TableName() string {
	return "_tool_copilot_user_teams"
}
