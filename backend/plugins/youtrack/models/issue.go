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

// YoutrackIssue is a YouTrack issue (tool layer), converted to ticket.Issue.
//
// YouTrack's State/Type/Priority/Assignee are custom fields, not top-level
// properties, and instances rename them (e.g. `Client type` instead of
// `Type`). The well-known fields flatten to dedicated columns, extracted by
// configured field name + `$type` (never by array index); the whole
// `customFields` array is serialized into CustomFieldsJson as a catch-all so
// dropped fields stay recoverable without re-collection.
//
// Source timestamps are named Created/Updated/Resolved (Jira style) so they
// don't shadow NoPKModel.CreatedAt/UpdatedAt. Url is not stored — it is
// computed at conversion time as `{endpoint}/issue/{IdReadable}`. There are
// deliberately no ParentId/links columns: links are out of scope and the raw
// JSON retains them for a future re-extract.
type YoutrackIssue struct {
	ConnectionId    uint64 `gorm:"primaryKey"`
	Id              string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	IdReadable      string `gorm:"type:varchar(255)" json:"idReadable"` // e.g. PROJ-123 → domain IssueKey
	NumberInProject int64  `json:"numberInProject"`
	ProjectId       string `gorm:"index;type:varchar(255)" json:"projectId"`
	Summary         string `json:"summary"`
	Description     string `json:"description"`
	Created         time.Time
	Updated         time.Time `gorm:"index"`
	Resolved        *time.Time
	CommentsCount   int
	ReporterId      string `gorm:"type:varchar(255)" json:"reporterId"`
	ReporterName    string `gorm:"type:varchar(255)" json:"reporterName"`
	// custom-field columns, extracted by configured field name + $type
	StateName       string `gorm:"type:varchar(255)" json:"stateName"`
	StateIsResolved bool   `json:"stateIsResolved"` // zero-config DONE signal
	TypeName        string `gorm:"type:varchar(255)" json:"typeName"`
	PriorityName    string `gorm:"type:varchar(255)" json:"priorityName"`
	AssigneeId      string `gorm:"type:varchar(255)" json:"assigneeId"`
	AssigneeName    string `gorm:"type:varchar(255)" json:"assigneeName"`
	// optional slots: populated only when the scope config names a field
	StoryPoint *float64   `json:"storyPoint"`
	DueDate    *time.Time `json:"dueDate"`
	// derived at extraction (mappings applied Jira-style)
	StdType         string `gorm:"type:varchar(255)" json:"stdType"`
	StdStatus       string `gorm:"type:varchar(255)" json:"stdStatus"`
	LeadTimeMinutes *uint  `json:"leadTimeMinutes"`
	// CustomFieldsJson is the whole customFields array, serialized — the
	// catch-all that keeps dropped fields recoverable.
	CustomFieldsJson string `gorm:"type:text" json:"customFieldsJson"`
	common.NoPKModel
}

func (YoutrackIssue) TableName() string {
	return "_tool_youtrack_issues"
}
