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

// ClaudeCodeChatProject captures per-project daily usage from the
// /v1/organizations/analytics/apps/chat/projects endpoint.
type ClaudeCodeChatProject struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Date         time.Time `gorm:"primaryKey;type:date" json:"date"`
	ProjectId    string    `gorm:"primaryKey;type:varchar(255)" json:"projectId"`

	ProjectName       string    `json:"projectName" gorm:"type:varchar(255)"`
	DistinctUserCount int       `json:"distinctUserCount"`
	ConversationCount int       `json:"conversationCount"`
	MessageCount      int       `json:"messageCount"`
	CreatedAt         time.Time `json:"createdAt"`
	CreatedById       string    `json:"createdById" gorm:"type:varchar(255)"`
	CreatedByEmail    string    `json:"createdByEmail" gorm:"type:varchar(255);index"`

	common.NoPKModel
}

func (ClaudeCodeChatProject) TableName() string {
	return "_tool_claude_code_chat_project"
}
