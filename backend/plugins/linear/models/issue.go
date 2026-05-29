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

// LinearIssue is the tool-layer representation of a Linear issue.
type LinearIssue struct {
	ConnectionId  uint64     `gorm:"primaryKey"`
	Id            string     `gorm:"primaryKey;type:varchar(255)" json:"id"`
	TeamId        string     `gorm:"index;type:varchar(255)" json:"teamId"`
	Identifier    string     `gorm:"type:varchar(255)" json:"identifier"`
	Number        int        `json:"number"`
	Title         string     `gorm:"type:varchar(255)" json:"title"`
	Description   string     `json:"description"`
	Url           string     `gorm:"type:varchar(255)" json:"url"`
	Priority      int        `json:"priority"`
	PriorityLabel string     `gorm:"type:varchar(100)" json:"priorityLabel"`
	Estimate      *float64   `json:"estimate"`
	StateId       string     `gorm:"index;type:varchar(255)" json:"stateId"`
	StateName     string     `gorm:"type:varchar(255)" json:"stateName"`
	StateType     string     `gorm:"type:varchar(100)" json:"stateType"`
	CreatorId     string     `gorm:"type:varchar(255)" json:"creatorId"`
	AssigneeId    string     `gorm:"type:varchar(255)" json:"assigneeId"`
	CycleId       string     `gorm:"index;type:varchar(255)" json:"cycleId"`
	ParentId      string     `gorm:"type:varchar(255)" json:"parentId"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `gorm:"index" json:"updatedAt"`
	StartedAt     *time.Time `json:"startedAt"`
	CompletedAt   *time.Time `json:"completedAt"`
	CanceledAt    *time.Time `json:"canceledAt"`
	common.NoPKModel
}

func (LinearIssue) TableName() string {
	return "_tool_linear_issues"
}
