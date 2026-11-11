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

	"github.com/apache/devlake/core/models/common"
)

// Description, relations, custom fields, comments, and changelog values use a size past 16MiB
// so MySQL creates LONGTEXT. TEXT is 64KiB there, and Azure payloads exceed it. Postgres maps this size to text.
type AzuredevopsWorkItem struct {
	common.NoPKModel

	ConnectionId            uint64 `gorm:"primaryKey"`
	AzuredevopsId           int    `gorm:"primaryKey"`
	OrgId                   string `gorm:"type:varchar(255)"`
	ProjectId               string `gorm:"type:varchar(255)"`
	Title                   string
	WorkItemType            string `gorm:"type:varchar(255)"`
	State                   string `gorm:"type:varchar(255)"`
	Reason                  string `gorm:"type:varchar(255)"`
	Priority                string `gorm:"type:varchar(255)"`
	Severity                string `gorm:"type:varchar(255)"`
	Description             string `gorm:"size:16777217"`
	Url                     string `gorm:"type:varchar(500)"`
	CreatedDate             *time.Time
	UpdatedDate             *time.Time
	ClosedDate              *time.Time
	DueDate                 *time.Time
	OriginalEstimateMinutes *int64
	TimeSpentMinutes        *int64
	TimeRemainingMinutes    *int64
	CreatorId               string `gorm:"type:varchar(255)"`
	CreatorName             string `gorm:"type:varchar(255)"`
	AssigneeId              string `gorm:"type:varchar(255)"`
	AssigneeName            string `gorm:"type:varchar(255)"`
	ParentId                *int
	StoryPoints             *float64
	IterationPath           string `gorm:"type:varchar(500)"`
	AreaPath                string `gorm:"type:varchar(500)"`
	Tags                    string `gorm:"type:text"`
	RelationsJson           string `gorm:"size:16777217"`
	CustomFieldsJson        string `gorm:"size:16777217"`
}

func (AzuredevopsWorkItem) TableName() string {
	return "_tool_azuredevops_go_work_items"
}

type AzuredevopsWorkItemComment struct {
	common.NoPKModel

	ConnectionId  uint64 `gorm:"primaryKey"`
	WorkItemId    int    `gorm:"primaryKey"`
	AzuredevopsId int    `gorm:"primaryKey"`
	ProjectId     string `gorm:"type:varchar(255)"`
	Body          string `gorm:"size:16777217"`
	AuthorId      string `gorm:"type:varchar(255)"`
	AuthorName    string `gorm:"type:varchar(255)"`
	CreatedDate   *time.Time
	UpdatedDate   *time.Time
}

func (AzuredevopsWorkItemComment) TableName() string {
	return "_tool_azuredevops_go_work_item_comments"
}

type AzuredevopsWorkItemUpdate struct {
	common.NoPKModel

	ConnectionId uint64 `gorm:"primaryKey"`
	WorkItemId   int    `gorm:"primaryKey"`
	Revision     int    `gorm:"primaryKey"`
	FieldRef     string `gorm:"primaryKey;type:varchar(255)"`
	ProjectId    string `gorm:"type:varchar(255)"`
	WorkItemType string `gorm:"type:varchar(255)"`
	AuthorId     string `gorm:"type:varchar(255)"`
	AuthorName   string `gorm:"type:varchar(255)"`
	OriginalFrom string `gorm:"size:16777217"`
	OriginalTo   string `gorm:"size:16777217"`
	CreatedDate  *time.Time
}

func (AzuredevopsWorkItemUpdate) TableName() string {
	return "_tool_azuredevops_go_work_item_updates"
}
