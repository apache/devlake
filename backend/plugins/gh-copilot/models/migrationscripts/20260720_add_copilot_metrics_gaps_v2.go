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

type addCopilotMetricsGapsV2 struct{}

// --- Enterprise daily metrics: columns added in the latest Copilot API pass ---

type enterpriseDailyMetrics20260720 struct {
	// totals_by_cli additions
	CliLastKnownVersion    string `gorm:"type:varchar(50)"`
	CliAvgTokensPerRequest float64

	// cloud/coding agent active user counts
	DailyActiveCopilotCloudAgentUsers   int
	WeeklyActiveCopilotCloudAgentUsers  int
	MonthlyActiveCopilotCloudAgentUsers int

	// pull_requests.copilot_suggestions_by_comment_type (JSON blob)
	PRCopilotSuggestionsByCommentType string `gorm:"type:text"`
}

func (enterpriseDailyMetrics20260720) TableName() string {
	return "_tool_copilot_enterprise_daily_metrics"
}

// --- User daily metrics: columns added in the latest Copilot API pass ---

type userDailyMetrics20260720 struct {
	UsedCopilotCodingAgent bool
	UsedCopilotCloudAgent  bool
	AiAdoptionPhase        int
	AiAdoptionPhaseVersion int

	// totals_by_cli additions (CopilotCliMetrics is squashed into this table too)
	CliLastKnownVersion    string `gorm:"type:varchar(50)"`
	CliAvgTokensPerRequest float64
}

func (userDailyMetrics20260720) TableName() string {
	return "_tool_copilot_user_daily_metrics"
}

// --- New table: enterprise/org metrics broken down by AI adoption phase ---

type metricsByAiAdoptionPhase20260720 struct {
	ConnectionId uint64    `gorm:"primaryKey"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)"`
	Day          time.Time `gorm:"primaryKey;type:date"`
	Phase        int       `gorm:"primaryKey"`

	PhaseVersion int
	EngagedUsers int

	AvgUserInitiatedInteractionCount float64
	AvgCodeGenerationActivityCount   float64
	AvgCodeAcceptanceActivityCount   float64
	AvgLocAddedSum                   float64
	AvgLocDeletedSum                 float64

	AvgPullRequestsCreated  float64
	AvgPullRequestsMerged   float64
	AvgPullRequestsReviewed float64
	AvgMedianMinutesToMerge float64

	AvgPullRequestsMinutesToReview float64
	AvgPullRequestsReviewCycles    float64

	TotalPullRequestsMerged int

	archived.NoPKModel
}

func (metricsByAiAdoptionPhase20260720) TableName() string {
	return "_tool_copilot_metrics_by_ai_adoption_phase"
}

func (script *addCopilotMetricsGapsV2) Up(basicRes context.BasicRes) errors.Error {
	// Add the new columns to the existing daily-metrics tables
	if err := migrationhelper.AutoMigrateTables(basicRes,
		&enterpriseDailyMetrics20260720{},
		&userDailyMetrics20260720{},
	); err != nil {
		return err
	}

	// Create the new AI-adoption-phase breakdown table
	return migrationhelper.AutoMigrateTables(basicRes,
		&metricsByAiAdoptionPhase20260720{},
	)
}

func (*addCopilotMetricsGapsV2) Version() uint64 {
	return 20260720000000
}

func (*addCopilotMetricsGapsV2) Name() string {
	return "Add Copilot metrics gaps v2: CLI version/tokens, cloud-agent users, PR suggestion-type blob, AI adoption phase"
}
