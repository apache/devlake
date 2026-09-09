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

// CursorAiCodeChange stores granular accepted AI changes from
// GET /analytics/ai-code/changes (Enterprise API).
type CursorAiCodeChange struct {
	ConnectionId uint64 `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	ChangeId     string `gorm:"primaryKey;type:varchar(255)" json:"changeId"`

	UserId          string     `gorm:"type:varchar(255);index" json:"userId"`
	UserEmail       string     `gorm:"type:varchar(255);index" json:"userEmail"`
	Source          string     `gorm:"type:varchar(20)" json:"source"`
	Model           string     `gorm:"type:varchar(255)" json:"model"`
	TotalLinesAdded   int        `json:"totalLinesAdded"`
	TotalLinesDeleted int        `json:"totalLinesDeleted"`
	IngestedAt        *time.Time `gorm:"column:ingested_at" json:"createdAt"`
	Metadata          string     `gorm:"type:text" json:"metadata"`

	common.NoPKModel
}

func (CursorAiCodeChange) TableName() string {
	return "_tool_cursor_ai_code_changes"
}
