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

package migrationscripts

import (
	"encoding/json"
	"time"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

// replaceClaudeCodeAnalyticsTables creates the five analytics endpoint tables
// and their raw ingestion tables for the updated API surface.
type replaceClaudeCodeAnalyticsTables struct{}

func (*replaceClaudeCodeAnalyticsTables) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&ccUserActivity20260319{},
		&ccActivitySummary20260319{},
		&ccChatProject20260319{},
		&ccSkillUsage20260319{},
		&ccConnectorUsage20260319{},
		&ccRawUserActivity20260319{},
		&ccRawActivitySummary20260319{},
		&ccRawChatProject20260319{},
		&ccRawSkillUsage20260319{},
		&ccRawConnectorUsage20260319{},
	)
}

func (*replaceClaudeCodeAnalyticsTables) Version() uint64 { return 20260319000001 }
func (*replaceClaudeCodeAnalyticsTables) Name() string {
	return "claude-code replace deprecated analytics tables"
}

// ── Tool-layer table snapshots ────────────────────────────────────────────────

type ccUserActivity20260319 struct {
	ConnectionId uint64    `gorm:"primaryKey"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)"`
	Date         time.Time `gorm:"primaryKey;type:date"`
	UserId       string    `gorm:"primaryKey;type:varchar(255)"`
	UserEmail    string    `gorm:"type:varchar(255);index"`

	ChatConversationCount     int
	ChatMessageCount          int
	ChatProjectsCreatedCount  int
	ChatProjectsUsedCount     int
	ChatFilesUploadedCount    int
	ChatArtifactsCreatedCount int
	ChatThinkingMessageCount  int
	ChatSkillsUsedCount       int
	ChatConnectorsUsedCount   int

	CCCommitCount      int
	CCPullRequestCount int
	CCLinesAdded       int
	CCLinesRemoved     int
	CCSessionCount     int

	EditToolAccepted         int
	EditToolRejected         int
	MultiEditToolAccepted    int
	MultiEditToolRejected    int
	WriteToolAccepted        int
	WriteToolRejected        int
	NotebookEditToolAccepted int
	NotebookEditToolRejected int

	WebSearchCount int
	archived.NoPKModel
}

func (ccUserActivity20260319) TableName() string { return "_tool_claude_code_user_activity" }

type ccActivitySummary20260319 struct {
	ConnectionId           uint64    `gorm:"primaryKey"`
	ScopeId                string    `gorm:"primaryKey;type:varchar(255)"`
	Date                   time.Time `gorm:"primaryKey;type:date"`
	DailyActiveUserCount   int
	WeeklyActiveUserCount  int
	MonthlyActiveUserCount int
	AssignedSeatCount      int
	PendingInviteCount     int
	archived.NoPKModel
}

func (ccActivitySummary20260319) TableName() string { return "_tool_claude_code_activity_summary" }

type ccChatProject20260319 struct {
	ConnectionId      uint64    `gorm:"primaryKey"`
	ScopeId           string    `gorm:"primaryKey;type:varchar(255)"`
	Date              time.Time `gorm:"primaryKey;type:date"`
	ProjectId         string    `gorm:"primaryKey;type:varchar(255)"`
	ProjectName       string    `gorm:"type:varchar(255)"`
	DistinctUserCount int
	ConversationCount int
	MessageCount      int
	CreatedAt         time.Time
	CreatedById       string `gorm:"type:varchar(255)"`
	CreatedByEmail    string `gorm:"type:varchar(255);index"`
	archived.NoPKModel
}

func (ccChatProject20260319) TableName() string { return "_tool_claude_code_chat_project" }

type ccSkillUsage20260319 struct {
	ConnectionId          uint64    `gorm:"primaryKey"`
	ScopeId               string    `gorm:"primaryKey;type:varchar(255)"`
	Date                  time.Time `gorm:"primaryKey;type:date"`
	SkillName             string    `gorm:"primaryKey;type:varchar(255)"`
	DistinctUserCount     int
	ChatConversationCount int
	CCSessionCount        int
	archived.NoPKModel
}

func (ccSkillUsage20260319) TableName() string { return "_tool_claude_code_skill_usage" }

type ccConnectorUsage20260319 struct {
	ConnectionId          uint64    `gorm:"primaryKey"`
	ScopeId               string    `gorm:"primaryKey;type:varchar(255)"`
	Date                  time.Time `gorm:"primaryKey;type:date"`
	ConnectorName         string    `gorm:"primaryKey;type:varchar(255)"`
	DistinctUserCount     int
	ChatConversationCount int
	CCSessionCount        int
	archived.NoPKModel
}

func (ccConnectorUsage20260319) TableName() string { return "_tool_claude_code_connector_usage" }

// ── Raw table snapshots ──────────────────────────────────────────────────────

type ccRawUserActivity20260319 struct {
	ID        uint64 `gorm:"primaryKey"`
	Params    string `gorm:"type:varchar(255);index"`
	Data      []byte
	Url       string
	Input     json.RawMessage `gorm:"type:json"`
	CreatedAt time.Time       `gorm:"index"`
}

func (ccRawUserActivity20260319) TableName() string { return "_raw_claude_code_user_activity" }

type ccRawActivitySummary20260319 struct {
	ID        uint64 `gorm:"primaryKey"`
	Params    string `gorm:"type:varchar(255);index"`
	Data      []byte
	Url       string
	Input     json.RawMessage `gorm:"type:json"`
	CreatedAt time.Time       `gorm:"index"`
}

func (ccRawActivitySummary20260319) TableName() string { return "_raw_claude_code_activity_summary" }

type ccRawChatProject20260319 struct {
	ID        uint64 `gorm:"primaryKey"`
	Params    string `gorm:"type:varchar(255);index"`
	Data      []byte
	Url       string
	Input     json.RawMessage `gorm:"type:json"`
	CreatedAt time.Time       `gorm:"index"`
}

func (ccRawChatProject20260319) TableName() string { return "_raw_claude_code_chat_project" }

type ccRawSkillUsage20260319 struct {
	ID        uint64 `gorm:"primaryKey"`
	Params    string `gorm:"type:varchar(255);index"`
	Data      []byte
	Url       string
	Input     json.RawMessage `gorm:"type:json"`
	CreatedAt time.Time       `gorm:"index"`
}

func (ccRawSkillUsage20260319) TableName() string { return "_raw_claude_code_skill_usage" }

type ccRawConnectorUsage20260319 struct {
	ID        uint64 `gorm:"primaryKey"`
	Params    string `gorm:"type:varchar(255);index"`
	Data      []byte
	Url       string
	Input     json.RawMessage `gorm:"type:json"`
	CreatedAt time.Time       `gorm:"index"`
}

func (ccRawConnectorUsage20260319) TableName() string { return "_raw_claude_code_connector_usage" }
