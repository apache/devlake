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

// ClaudeCodeUserActivity captures per-user daily engagement metrics from the
// /v1/organizations/analytics/users endpoint.
type ClaudeCodeUserActivity struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Date         time.Time `gorm:"primaryKey;type:date" json:"date"`
	UserId       string    `gorm:"primaryKey;type:varchar(255)" json:"userId"`

	UserEmail string `json:"userEmail" gorm:"type:varchar(255);index"`

	// Claude.ai (chat) metrics
	ChatConversationCount     int `json:"chatConversationCount"`
	ChatMessageCount          int `json:"chatMessageCount"`
	ChatProjectsCreatedCount  int `json:"chatProjectsCreatedCount"`
	ChatProjectsUsedCount     int `json:"chatProjectsUsedCount"`
	ChatFilesUploadedCount    int `json:"chatFilesUploadedCount"`
	ChatArtifactsCreatedCount int `json:"chatArtifactsCreatedCount"`
	ChatThinkingMessageCount  int `json:"chatThinkingMessageCount"`
	ChatSkillsUsedCount       int `json:"chatSkillsUsedCount"`
	ChatConnectorsUsedCount   int `json:"chatConnectorsUsedCount"`

	// Claude Code core metrics
	CCCommitCount      int `json:"ccCommitCount"`
	CCPullRequestCount int `json:"ccPullRequestCount"`
	CCLinesAdded       int `json:"ccLinesAdded"`
	CCLinesRemoved     int `json:"ccLinesRemoved"`
	CCSessionCount     int `json:"ccSessionCount"`

	// Claude Code tool actions
	EditToolAccepted         int `json:"editToolAccepted"`
	EditToolRejected         int `json:"editToolRejected"`
	MultiEditToolAccepted    int `json:"multiEditToolAccepted"`
	MultiEditToolRejected    int `json:"multiEditToolRejected"`
	WriteToolAccepted        int `json:"writeToolAccepted"`
	WriteToolRejected        int `json:"writeToolRejected"`
	NotebookEditToolAccepted int `json:"notebookEditToolAccepted"`
	NotebookEditToolRejected int `json:"notebookEditToolRejected"`

	WebSearchCount int `json:"webSearchCount"`

	common.NoPKModel
}

func (ClaudeCodeUserActivity) TableName() string {
	return "_tool_claude_code_user_activity"
}
