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

// GhCopilotSeat represents a seat assignment snapshot for Copilot.
type GhCopilotSeat struct {
	ConnectionId            uint64 `gorm:"primaryKey"`
	Organization            string `gorm:"primaryKey;type:varchar(255)"`
	UserLogin               string `gorm:"primaryKey;type:varchar(255)"`
	UserId                  int64  `gorm:"index"`
	UserName                string `gorm:"type:varchar(255)" json:"userName"`
	UserEmail               string `gorm:"type:varchar(255)" json:"userEmail"`
	PlanType                string `gorm:"type:varchar(32)"`
	AssigningTeamId         int64  `json:"assigningTeamId" gorm:"comment:Team that assigned the seat"`
	AssigningTeamName       string `json:"assigningTeamName" gorm:"type:varchar(255)"`
	AssigningTeamSlug       string `json:"assigningTeamSlug" gorm:"type:varchar(255)"`
	CreatedAt               time.Time
	LastActivityAt          *time.Time
	LastActivityEditor      string
	LastAuthenticatedAt     *time.Time
	PendingCancellationDate *time.Time
	UpdatedAt               time.Time

	common.RawDataOrigin
}

func (GhCopilotSeat) TableName() string {
	return "_tool_copilot_seats"
}
