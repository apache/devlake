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

// TaigaUserStory represents a user story in Taiga
type TaigaUserStory struct {
	common.NoPKModel
	ConnectionId   uint64     `gorm:"primaryKey"`
	ProjectId      uint64     `gorm:"index"`
	UserStoryId    uint64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Ref            int        `json:"ref"`
	Subject        string     `gorm:"type:varchar(255)" json:"subject"`
	Description    string     `gorm:"type:text" json:"description"`
	Status         string     `gorm:"type:varchar(100)" json:"status"`
	StatusColor    string     `gorm:"type:varchar(20)" json:"statusColor"`
	IsClosed       bool       `json:"isClosed"`
	CreatedDate    *time.Time `json:"createdDate"`
	ModifiedDate   *time.Time `json:"modifiedDate"`
	FinishedDate   *time.Time `json:"finishedDate"`
	AssignedTo     uint64     `json:"assignedTo"`
	AssignedToName string     `gorm:"type:varchar(255)" json:"assignedToName"`
	TotalPoints    float64    `json:"totalPoints"`
	MilestoneId    uint64     `json:"milestoneId"`
	MilestoneName  string     `gorm:"type:varchar(255)" json:"milestoneName"`
	Priority       int        `json:"priority"`
	IsBlocked      bool       `json:"isBlocked"`
	BlockedNote    string     `gorm:"type:text" json:"blockedNote"`
}

func (TaigaUserStory) TableName() string {
	return "_tool_taiga_user_stories"
}
