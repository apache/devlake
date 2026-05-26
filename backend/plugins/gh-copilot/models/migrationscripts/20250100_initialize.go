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
	"time"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

// addCopilotInitialTables creates the initial Copilot tool-layer tables.
type addCopilotInitialTables struct{}

func (script *addCopilotInitialTables) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&ghCopilotConnection20250100{},
		&ghCopilotScope20250100{},
		&ghCopilotOrgMetrics20250100{},
		&ghCopilotLanguageMetrics20250100{},
		&ghCopilotSeat20250100{},
	)
}

type noPKModel20250100 struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ghCopilotConnection20250100 struct {
	archived.Model
	Name             string `gorm:"type:varchar(100);uniqueIndex" json:"name"`
	Endpoint         string `gorm:"type:varchar(255)" json:"endpoint"`
	Proxy            string `gorm:"type:varchar(255)" json:"proxy"`
	RateLimitPerHour int    `json:"rateLimitPerHour"`
	Token            string `json:"token"`
	Organization     string `gorm:"type:varchar(255)" json:"organization"`
}

func (ghCopilotConnection20250100) TableName() string {
	return "_tool_copilot_connections"
}

type ghCopilotScope20250100 struct {
	archived.NoPKModel
	ConnectionId       uint64     `json:"connectionId" gorm:"primaryKey"`
	ScopeConfigId      uint64     `json:"scopeConfigId,omitempty"`
	Id                 string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Organization       string     `json:"organization" gorm:"type:varchar(255)"`
	ImplementationDate *time.Time `json:"implementationDate"`
	BaselinePeriodDays int        `json:"baselinePeriodDays" gorm:"default:90"`
	SeatsLastSyncedAt  *time.Time `json:"seatsLastSyncedAt"`
}

func (ghCopilotScope20250100) TableName() string {
	return "_tool_copilot_scopes"
}

type ghCopilotOrgMetrics20250100 struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Date         time.Time `gorm:"primaryKey;type:date" json:"date"`

	TotalActiveUsers         int `json:"totalActiveUsers"`
	TotalEngagedUsers        int `json:"totalEngagedUsers"`
	CompletionSuggestions    int `json:"completionSuggestions"`
	CompletionAcceptances    int `json:"completionAcceptances"`
	CompletionLinesSuggested int `json:"completionLinesSuggested"`
	CompletionLinesAccepted  int `json:"completionLinesAccepted"`
	IdeChats                 int `json:"ideChats"`
	IdeChatCopyEvents        int `json:"ideChatCopyEvents"`
	IdeChatInsertionEvents   int `json:"ideChatInsertionEvents"`
	IdeChatEngagedUsers      int `json:"ideChatEngagedUsers"`
	DotcomChats              int `json:"dotcomChats"`
	DotcomChatEngagedUsers   int `json:"dotcomChatEngagedUsers"`
	PRSummariesCreated       int `json:"prSummariesCreated"`
	PREngagedUsers           int `json:"prEngagedUsers"`
	SeatActiveCount          int `json:"seatActiveCount"`
	SeatTotal                int `json:"seatTotal"`

	archived.NoPKModel
}

func (ghCopilotOrgMetrics20250100) TableName() string {
	return "_tool_copilot_org_daily_metrics"
}

type ghCopilotLanguageMetrics20250100 struct {
	ConnectionId uint64    `gorm:"primaryKey"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)"`
	Date         time.Time `gorm:"primaryKey;type:date"`
	Editor       string    `gorm:"primaryKey;type:varchar(50)"`
	Language     string    `gorm:"primaryKey;type:varchar(50)"`

	EngagedUsers   int `json:"engagedUsers"`
	Suggestions    int `json:"suggestions"`
	Acceptances    int `json:"acceptances"`
	LinesSuggested int `json:"linesSuggested"`
	LinesAccepted  int `json:"linesAccepted"`

	noPKModel20250100
}

func (ghCopilotLanguageMetrics20250100) TableName() string {
	return "_tool_copilot_org_language_metrics"
}

type ghCopilotSeat20250100 struct {
	ConnectionId            uint64 `gorm:"primaryKey"`
	Organization            string `gorm:"primaryKey;type:varchar(255)"`
	UserLogin               string `gorm:"primaryKey;type:varchar(255)"`
	UserId                  int64  `gorm:"index"`
	PlanType                string `gorm:"type:varchar(32)"`
	CreatedAt               time.Time
	LastActivityAt          *time.Time
	LastActivityEditor      string
	LastAuthenticatedAt     *time.Time
	PendingCancellationDate *time.Time
	UpdatedAt               time.Time
}

func (ghCopilotSeat20250100) TableName() string {
	return "_tool_copilot_seats"
}

func (*addCopilotInitialTables) Version() uint64 {
	return 20250100000000
}

func (*addCopilotInitialTables) Name() string {
	return "copilot init tables"
}
