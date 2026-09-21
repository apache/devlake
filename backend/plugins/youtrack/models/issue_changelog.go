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

// YoutrackIssueChangelog is one activity item from an issue's activity feed
// (tool layer), converted to ticket.IssueChangelog. Single table, one row per
// activity item; multi-value added/removed payloads are JSON-encoded into the
// value columns. `Created` is epoch-ms UTC at the source, so unlike issue
// query dates there is no timezone hazard on this table.
type YoutrackIssueChangelog struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Id           string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	IssueId      string `gorm:"index;type:varchar(255)" json:"issueId"`
	Category     string `gorm:"type:varchar(100)" json:"category"` // e.g. CustomFieldCategory
	FieldName    string `gorm:"type:varchar(255)" json:"fieldName"`
	AuthorId     string `gorm:"type:varchar(255)" json:"authorId"`
	AuthorName   string `gorm:"type:varchar(255)" json:"authorName"`
	// value ids; a JSON array string when the change is multi-value
	FromValue string `json:"fromValue"`
	ToValue   string `json:"toValue"`
	// display strings (name/login/text); JSON when multi-value
	OriginalFromValue string    `json:"originalFromValue"`
	OriginalToValue   string    `json:"originalToValue"`
	Created           time.Time `gorm:"index"`
	common.NoPKModel
}

func (YoutrackIssueChangelog) TableName() string {
	return "_tool_youtrack_issue_changelogs"
}
