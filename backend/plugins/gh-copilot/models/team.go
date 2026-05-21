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

// GhCopilotTeam stores a GitHub organization team in the tool layer.
type GhCopilotTeam struct {
	ConnectionId        uint64 `gorm:"primaryKey"`
	Id                  int    `json:"id" gorm:"primaryKey;autoIncrement:false"`
	OrgId               *int   `json:"organization_id" gorm:"index"`
	OrgLogin            string `json:"org_login" gorm:"type:varchar(255);index"`
	Name                string `json:"name" gorm:"type:varchar(255)"`
	Slug                string `json:"slug" gorm:"type:varchar(255);index"`
	Description         string `json:"description"`
	Privacy             string `json:"privacy" gorm:"type:varchar(100)"`
	Permission          string `json:"permission" gorm:"type:varchar(100)"`
	NotificationSetting string `json:"notification_setting" gorm:"type:varchar(100)"`
	ParentTeamId        *int   `json:"parent_team_id" gorm:"index"`
	ParentTeamSlug      string `json:"parent_team_slug" gorm:"type:varchar(255);index"`
	GithubCreatedAt     *time.Time
	GithubUpdatedAt     *time.Time
	common.NoPKModel
}

func (GhCopilotTeam) TableName() string {
	return "_tool_copilot_teams"
}
