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

// ClaudeCodeActivitySummary captures daily organisation-level engagement and seat
// utilisation from the /v1/organizations/analytics/summaries endpoint.
type ClaudeCodeActivitySummary struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Date         time.Time `gorm:"primaryKey;type:date" json:"date"` // = starting_date

	DailyActiveUserCount   int `json:"dailyActiveUserCount"`
	WeeklyActiveUserCount  int `json:"weeklyActiveUserCount"`
	MonthlyActiveUserCount int `json:"monthlyActiveUserCount"`
	AssignedSeatCount      int `json:"assignedSeatCount"`
	PendingInviteCount     int `json:"pendingInviteCount"`

	common.NoPKModel
}

func (ClaudeCodeActivitySummary) TableName() string {
	return "_tool_claude_code_activity_summary"
}
