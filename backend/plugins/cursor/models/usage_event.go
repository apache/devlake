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

// CursorUsageEvent stores one usage event from /teams/filtered-usage-events.
type CursorUsageEvent struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	EventId      string    `gorm:"primaryKey;type:varchar(64)" json:"eventId"`
	EventTime    time.Time `gorm:"index" json:"eventTime"`

	UserEmail          string  `json:"userEmail" gorm:"type:varchar(255);index"`
	Model              string  `json:"model" gorm:"type:varchar(255);index"`
	Kind               string  `json:"kind" gorm:"type:varchar(100)"`
	ConversationId     string  `json:"conversationId" gorm:"type:varchar(64);index"`
	ChargedCents       float64 `json:"chargedCents"`
	RequestsCosts      float64 `json:"requestsCosts"`
	IsTokenBasedCall   bool    `json:"isTokenBasedCall"`
	IsChargeable       bool    `json:"isChargeable"`
	MaxMode            bool    `json:"maxMode"`
	IsHeadless         bool    `json:"isHeadless"`
	InputTokens        int     `json:"inputTokens"`
	OutputTokens       int     `json:"outputTokens"`
	CacheReadTokens    int     `json:"cacheReadTokens"`
	CacheWriteTokens   int     `json:"cacheWriteTokens"`
	TotalCents         float64 `json:"totalCents"`
	CursorTokenFee     float64 `json:"cursorTokenFee"`
	HostingType        string  `json:"hostingType" gorm:"type:varchar(50)"`
	ServiceAccountId   string  `json:"serviceAccountId" gorm:"type:varchar(255);index"`
	ServiceAccountName string  `json:"serviceAccountName" gorm:"type:varchar(255)"`

	common.NoPKModel
}

func (CursorUsageEvent) TableName() string {
	return "_tool_cursor_usage_events"
}
