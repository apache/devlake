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

type AsanaTask struct {
	ConnectionId    uint64     `gorm:"primaryKey"`
	Gid             string     `gorm:"primaryKey;type:varchar(255)"`
	Name            string     `gorm:"type:varchar(512)"`
	Notes           string     `gorm:"type:text"`
	ResourceType    string     `gorm:"type:varchar(32)"`
	ResourceSubtype string     `gorm:"type:varchar(32)"` // default_task, milestone, section, approval
	Completed       bool       `json:"completed"`
	CompletedAt     *time.Time `json:"completedAt"`
	DueOn           *time.Time `gorm:"type:date" json:"dueOn"`
	CreatedAt       time.Time  `json:"createdAt"`
	ModifiedAt      *time.Time `json:"modifiedAt"`
	PermalinkUrl    string     `gorm:"type:varchar(512)"`
	ProjectGid      string     `gorm:"type:varchar(255);index"`
	SectionGid      string     `gorm:"type:varchar(255);index"`
	SectionName     string     `gorm:"type:varchar(255)"` // For status mapping
	AssigneeGid     string     `gorm:"type:varchar(255)"`
	AssigneeName    string     `gorm:"type:varchar(255)"`
	CreatorGid      string     `gorm:"type:varchar(255)"`
	CreatorName     string     `gorm:"type:varchar(255)"`
	ParentGid       string     `gorm:"type:varchar(255);index"`
	NumSubtasks     int        `json:"numSubtasks"`

	// Transformed fields for domain layer
	StdType   string `gorm:"type:varchar(255)"` // Standard type: REQUIREMENT, BUG, INCIDENT, EPIC, TASK, SUBTASK
	StdStatus string `gorm:"type:varchar(255)"` // Standard status: TODO, IN_PROGRESS, DONE

	// Custom field values (extracted during transformation)
	Priority   string   `gorm:"type:varchar(255)"` // Priority from custom field
	StoryPoint *float64 `json:"storyPoint"`        // Story points from custom field
	Severity   string   `gorm:"type:varchar(255)"` // Severity from custom field

	// Lead time tracking
	LeadTimeMinutes *uint `json:"leadTimeMinutes"`

	common.NoPKModel
}

func (AsanaTask) TableName() string {
	return "_tool_asana_tasks"
}
