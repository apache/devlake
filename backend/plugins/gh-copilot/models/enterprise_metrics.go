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

// CopilotActivityMetrics contains the common activity/LOC fields shared across all breakdown tables.
type CopilotActivityMetrics struct {
	UserInitiatedInteractionCount int `json:"userInitiatedInteractionCount" gorm:"comment:Chat messages and inline prompts initiated by user"`
	CodeGenerationActivityCount   int `json:"codeGenerationActivityCount" gorm:"comment:Number of code suggestions/generations made"`
	CodeAcceptanceActivityCount   int `json:"codeAcceptanceActivityCount" gorm:"comment:Number of suggestions accepted by user"`
	LocSuggestedToAddSum          int `json:"locSuggestedToAddSum" gorm:"comment:Lines of code suggested for addition"`
	LocSuggestedToDeleteSum       int `json:"locSuggestedToDeleteSum" gorm:"comment:Lines of code suggested for deletion"`
	LocAddedSum                   int `json:"locAddedSum" gorm:"comment:Lines of code actually added (accepted)"`
	LocDeletedSum                 int `json:"locDeletedSum" gorm:"comment:Lines of code actually deleted (accepted)"`
}

// CopilotCodeMetrics contains code generation/acceptance metrics without user interaction count.
type CopilotCodeMetrics struct {
	CodeGenerationActivityCount int `json:"codeGenerationActivityCount"`
	CodeAcceptanceActivityCount int `json:"codeAcceptanceActivityCount"`
	LocSuggestedToAddSum        int `json:"locSuggestedToAddSum"`
	LocSuggestedToDeleteSum     int `json:"locSuggestedToDeleteSum"`
	LocAddedSum                 int `json:"locAddedSum"`
	LocDeletedSum               int `json:"locDeletedSum"`
}

// CopilotCliMetrics contains CLI usage breakdown metrics.
type CopilotCliMetrics struct {
	CliSessionCount        int     `json:"cliSessionCount" gorm:"comment:Number of CLI sessions"`
	CliRequestCount        int     `json:"cliRequestCount" gorm:"comment:Number of CLI requests"`
	CliPromptCount         int     `json:"cliPromptCount" gorm:"comment:Number of CLI prompts"`
	CliOutputTokenSum      int     `json:"cliOutputTokenSum" gorm:"comment:Total output tokens from CLI"`
	CliPromptTokenSum      int     `json:"cliPromptTokenSum" gorm:"comment:Total prompt tokens from CLI"`
	CliLastKnownVersion    string  `json:"cliLastKnownVersion" gorm:"type:varchar(50);comment:Last known Copilot CLI version"`
	CliAvgTokensPerRequest float64 `json:"cliAvgTokensPerRequest" gorm:"comment:Average tokens consumed per CLI request"`
}

// GhCopilotEnterpriseDailyMetrics captures daily enterprise-level aggregate Copilot metrics.
type GhCopilotEnterpriseDailyMetrics struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`

	EnterpriseId string `json:"enterpriseId" gorm:"type:varchar(100)"`
	// OrganizationId identifies the owning org for org-level connections/rows.
	// Previously always written as "" by ExtractOrgMetrics, making org-level
	// rows in this shared table unidentifiable without a join back to the
	// connection/scope config.
	OrganizationId          string `json:"organizationId" gorm:"type:varchar(100)"`
	DailyActiveUsers        int    `json:"dailyActiveUsers"`
	WeeklyActiveUsers       int    `json:"weeklyActiveUsers"`
	MonthlyActiveUsers      int    `json:"monthlyActiveUsers"`
	MonthlyActiveChatUsers  int    `json:"monthlyActiveChatUsers"`
	MonthlyActiveAgentUsers int    `json:"monthlyActiveAgentUsers"`

	// CLI active users
	DailyActiveCliUsers int `json:"dailyActiveCliUsers" gorm:"comment:Daily active CLI users"`

	// Copilot cloud agent (coding agent) active user counts
	DailyActiveCopilotCloudAgentUsers   int `json:"dailyActiveCopilotCloudAgentUsers" gorm:"comment:Daily active Copilot cloud/coding agent users"`
	WeeklyActiveCopilotCloudAgentUsers  int `json:"weeklyActiveCopilotCloudAgentUsers" gorm:"comment:Weekly active Copilot cloud/coding agent users"`
	MonthlyActiveCopilotCloudAgentUsers int `json:"monthlyActiveCopilotCloudAgentUsers" gorm:"comment:Monthly active Copilot cloud/coding agent users"`

	// Code review user counts
	DailyActiveCopilotCodeReviewUsers    int `json:"dailyActiveCopilotCodeReviewUsers"`
	DailyPassiveCopilotCodeReviewUsers   int `json:"dailyPassiveCopilotCodeReviewUsers"`
	WeeklyActiveCopilotCodeReviewUsers   int `json:"weeklyActiveCopilotCodeReviewUsers"`
	WeeklyPassiveCopilotCodeReviewUsers  int `json:"weeklyPassiveCopilotCodeReviewUsers"`
	MonthlyActiveCopilotCodeReviewUsers  int `json:"monthlyActiveCopilotCodeReviewUsers"`
	MonthlyPassiveCopilotCodeReviewUsers int `json:"monthlyPassiveCopilotCodeReviewUsers"`

	// Chat panel mode breakdown
	ChatPanelAgentMode   int `json:"chatPanelAgentMode" gorm:"comment:Chat panel agent mode interactions"`
	ChatPanelAskMode     int `json:"chatPanelAskMode" gorm:"comment:Chat panel ask mode interactions"`
	ChatPanelCustomMode  int `json:"chatPanelCustomMode" gorm:"comment:Chat panel custom mode interactions"`
	ChatPanelEditMode    int `json:"chatPanelEditMode" gorm:"comment:Chat panel edit mode interactions"`
	ChatPanelPlanMode    int `json:"chatPanelPlanMode" gorm:"comment:Chat panel plan mode interactions"`
	ChatPanelUnknownMode int `json:"chatPanelUnknownMode" gorm:"comment:Chat panel unknown mode interactions"`

	// Pull request metrics (expanded)
	PRTotalReviewed                   int     `json:"prTotalReviewed" gorm:"comment:Total PRs reviewed"`
	PRTotalCreated                    int     `json:"prTotalCreated" gorm:"comment:Total PRs created"`
	PRTotalMerged                     int     `json:"prTotalMerged" gorm:"comment:Total PRs merged"`
	PRMedianMinutesToMerge            float64 `json:"prMedianMinutesToMerge" gorm:"comment:Median minutes to merge PRs"`
	PRTotalSuggestions                int     `json:"prTotalSuggestions" gorm:"comment:Total PR review suggestions"`
	PRTotalAppliedSuggestions         int     `json:"prTotalAppliedSuggestions" gorm:"comment:Total applied PR suggestions"`
	PRTotalCreatedByCopilot           int     `json:"prTotalCreatedByCopilot" gorm:"comment:PRs created by Copilot"`
	PRTotalReviewedByCopilot          int     `json:"prTotalReviewedByCopilot" gorm:"comment:PRs reviewed by Copilot"`
	PRTotalMergedCreatedByCopilot     int     `json:"prTotalMergedCreatedByCopilot" gorm:"comment:Merged PRs created by Copilot"`
	PRTotalMergedReviewedByCopilot    int     `json:"prTotalMergedReviewedByCopilot" gorm:"comment:Merged PRs reviewed by Copilot"`
	PRMedianMinToMergeCopilotAuthored float64 `json:"prMedianMinToMergeCopilotAuthored" gorm:"comment:Median min to merge Copilot-authored PRs"`
	PRMedianMinToMergeCopilotReviewed float64 `json:"prMedianMinToMergeCopilotReviewed" gorm:"comment:Median min to merge Copilot-reviewed PRs"`
	PRTotalCopilotSuggestions         int     `json:"prTotalCopilotSuggestions" gorm:"comment:Total Copilot review suggestions"`
	PRTotalCopilotAppliedSuggestions  int     `json:"prTotalCopilotAppliedSuggestions" gorm:"comment:Total Copilot applied suggestions"`
	// PRCopilotSuggestionsByCommentType is a JSON-encoded array of
	// {comment_type, total_suggestions, total_applied_suggestions}, e.g. the
	// split between "suggestion" and "explanation" style review comments.
	PRCopilotSuggestionsByCommentType string `json:"prCopilotSuggestionsByCommentType" gorm:"type:text;comment:JSON breakdown of Copilot PR suggestions by comment type"`

	CopilotActivityMetrics `mapstructure:",squash"`
	CopilotCliMetrics      `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotEnterpriseDailyMetrics) TableName() string {
	return "_tool_copilot_enterprise_daily_metrics"
}

// GhCopilotMetricsByIde stores enterprise/org metrics broken down by IDE.
type GhCopilotMetricsByIde struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	Ide          string    `gorm:"primaryKey;type:varchar(50)" json:"ide"`

	CopilotActivityMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotMetricsByIde) TableName() string {
	return "_tool_copilot_metrics_by_ide"
}

// GhCopilotMetricsByFeature stores enterprise/org metrics broken down by feature.
type GhCopilotMetricsByFeature struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	Feature      string    `gorm:"primaryKey;type:varchar(100)" json:"feature"`

	CopilotActivityMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotMetricsByFeature) TableName() string {
	return "_tool_copilot_metrics_by_feature"
}

// GhCopilotMetricsByLanguageFeature stores metrics broken down by language and feature.
type GhCopilotMetricsByLanguageFeature struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	Language     string    `gorm:"primaryKey;type:varchar(50)" json:"language"`
	Feature      string    `gorm:"primaryKey;type:varchar(100)" json:"feature"`

	CopilotCodeMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotMetricsByLanguageFeature) TableName() string {
	return "_tool_copilot_metrics_by_language_feature"
}

// GhCopilotMetricsByLanguageModel stores metrics broken down by language and model.
type GhCopilotMetricsByLanguageModel struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	Language     string    `gorm:"primaryKey;type:varchar(50)" json:"language"`
	Model        string    `gorm:"primaryKey;type:varchar(100)" json:"model"`

	CopilotCodeMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotMetricsByLanguageModel) TableName() string {
	return "_tool_copilot_metrics_by_language_model"
}

// GhCopilotMetricsByModelFeature stores metrics broken down by model and feature.
type GhCopilotMetricsByModelFeature struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	Model        string    `gorm:"primaryKey;type:varchar(100)" json:"model"`
	Feature      string    `gorm:"primaryKey;type:varchar(100)" json:"feature"`

	CopilotActivityMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotMetricsByModelFeature) TableName() string {
	return "_tool_copilot_metrics_by_model_feature"
}

// GhCopilotMetricsByAiAdoptionPhase stores enterprise/org metrics broken down
// by AI adoption cohort (phase 0-3, see GitHub's totals_by_ai_adoption_phase).
// Unlike CopilotActivityMetrics elsewhere in this file, most of these fields
// are per-user averages within the phase rather than sums.
type GhCopilotMetricsByAiAdoptionPhase struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	Phase        int       `gorm:"primaryKey" json:"phase"`

	PhaseVersion int `json:"phaseVersion" gorm:"comment:Adoption-phase classification version, starts at 1"`
	EngagedUsers int `json:"engagedUsers" gorm:"comment:Users engaged in this phase (2-day-in-28 window)"`

	AvgUserInitiatedInteractionCount float64 `json:"avgUserInitiatedInteractionCount"`
	AvgCodeGenerationActivityCount   float64 `json:"avgCodeGenerationActivityCount"`
	AvgCodeAcceptanceActivityCount   float64 `json:"avgCodeAcceptanceActivityCount"`
	AvgLocAddedSum                   float64 `json:"avgLocAddedSum"`
	AvgLocDeletedSum                 float64 `json:"avgLocDeletedSum"`

	AvgPullRequestsCreated  float64 `json:"avgPullRequestsCreated"`
	AvgPullRequestsMerged   float64 `json:"avgPullRequestsMerged"`
	AvgPullRequestsReviewed float64 `json:"avgPullRequestsReviewed"`
	AvgMedianMinutesToMerge float64 `json:"avgMedianMinutesToMerge"`

	AvgPullRequestsMinutesToReview float64 `json:"avgPullRequestsMinutesToReview" gorm:"comment:Added 2026-07-07"`
	AvgPullRequestsReviewCycles    float64 `json:"avgPullRequestsReviewCycles" gorm:"comment:Added 2026-07-07"`

	TotalPullRequestsMerged int `json:"totalPullRequestsMerged" gorm:"comment:True sum (not average), added 2026-06-26"`

	common.NoPKModel
}

func (GhCopilotMetricsByAiAdoptionPhase) TableName() string {
	return "_tool_copilot_metrics_by_ai_adoption_phase"
}
