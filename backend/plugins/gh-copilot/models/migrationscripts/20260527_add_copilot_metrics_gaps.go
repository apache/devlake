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

type addCopilotMetricsGaps struct{}

// --- Enterprise daily metrics: new columns ---

type enterpriseDailyMetrics20260527 struct {
	// CLI
	DailyActiveCliUsers int

	// Code review user counts
	DailyActiveCopilotCodeReviewUsers    int
	DailyPassiveCopilotCodeReviewUsers   int
	WeeklyActiveCopilotCodeReviewUsers   int
	WeeklyPassiveCopilotCodeReviewUsers  int
	MonthlyActiveCopilotCodeReviewUsers  int
	MonthlyPassiveCopilotCodeReviewUsers int

	// Chat panel mode breakdown
	ChatPanelAgentMode   int
	ChatPanelAskMode     int
	ChatPanelCustomMode  int
	ChatPanelEditMode    int
	ChatPanelPlanMode    int
	ChatPanelUnknownMode int

	// Expanded PR metrics
	PRTotalMerged                       int
	PRMedianMinutesToMerge              float64
	PRTotalSuggestions                  int
	PRTotalAppliedSuggestions           int
	PRTotalMergedCreatedByCopilot       int
	PRTotalMergedReviewedByCopilot      int
	PRMedianMinToMergeCopilotAuthored   float64
	PRMedianMinToMergeCopilotReviewed   float64
	PRTotalCopilotSuggestions           int
	PRTotalCopilotAppliedSuggestions    int

	// CLI breakdown
	CliSessionCount   int
	CliRequestCount   int
	CliPromptCount    int
	CliOutputTokenSum int
	CliPromptTokenSum int
}

func (enterpriseDailyMetrics20260527) TableName() string {
	return "_tool_copilot_enterprise_daily_metrics"
}

// --- User daily metrics: new columns ---

type userDailyMetrics20260527 struct {
	UsedCli                       bool
	UsedCopilotCodeReviewActive   bool
	UsedCopilotCodeReviewPassive  bool

	// CLI breakdown
	CliSessionCount   int
	CliRequestCount   int
	CliPromptCount    int
	CliOutputTokenSum int
	CliPromptTokenSum int
}

func (userDailyMetrics20260527) TableName() string {
	return "_tool_copilot_user_daily_metrics"
}

// --- Seat: new columns ---

type seat20260527 struct {
	UserName          string `gorm:"type:varchar(255)"`
	UserEmail         string `gorm:"type:varchar(255)"`
	AssigningTeamId   int64
	AssigningTeamName string `gorm:"type:varchar(255)"`
	AssigningTeamSlug string `gorm:"type:varchar(255)"`
}

func (seat20260527) TableName() string {
	return "_tool_copilot_seats"
}

// --- User-teams: new table ---

type userTeam20260527 struct {
	ConnectionId   uint64    `gorm:"primaryKey"`
	ScopeId        string    `gorm:"primaryKey;type:varchar(255)"`
	Day            time.Time `gorm:"primaryKey;type:date"`
	UserId         int64     `gorm:"primaryKey"`
	TeamId         int64     `gorm:"primaryKey"`

	UserLogin      string `gorm:"type:varchar(255);index"`
	OrganizationId string `gorm:"type:varchar(100)"`
	EnterpriseId   string `gorm:"type:varchar(100)"`
	TeamSlug       string `gorm:"type:varchar(255)"`

	archived.NoPKModel
}

func (userTeam20260527) TableName() string {
	return "_tool_copilot_user_teams"
}

func (script *addCopilotMetricsGaps) Up(basicRes context.BasicRes) errors.Error {
	// Add new columns to existing tables
	if err := migrationhelper.AutoMigrateTables(basicRes,
		&enterpriseDailyMetrics20260527{},
		&userDailyMetrics20260527{},
		&seat20260527{},
	); err != nil {
		return err
	}

	// Create new user-teams table
	return migrationhelper.AutoMigrateTables(basicRes,
		&userTeam20260527{},
	)
}

func (*addCopilotMetricsGaps) Version() uint64 {
	return 20260527000000
}

func (*addCopilotMetricsGaps) Name() string {
	return "Add Copilot metrics gaps: CLI, code review, chat modes, PR expansion, user-teams"
}
