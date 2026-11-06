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

package tasks

import (
	"encoding/json"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/gh-copilot/models"
)

// --- Enterprise report JSON structures ---

type enterpriseDayTotal struct {
	Day          string `json:"day"`
	EnterpriseId string `json:"enterprise_id"`
	// OrganizationId is only present on organization-level reports (org report
	// shares this same flat schema). Needed so ExtractOrgMetrics can stamp the
	// owning org onto rows instead of leaving them unidentifiable.
	OrganizationId          string `json:"organization_id"`
	DailyActiveUsers        int    `json:"daily_active_users"`
	WeeklyActiveUsers       int    `json:"weekly_active_users"`
	MonthlyActiveUsers      int    `json:"monthly_active_users"`
	MonthlyActiveChatUsers  int    `json:"monthly_active_chat_users"`
	MonthlyActiveAgentUsers int    `json:"monthly_active_agent_users"`
	DailyActiveCliUsers     int    `json:"daily_active_cli_users"`

	// Copilot cloud agent (coding agent) active user counts — added in 2026-03-10.
	DailyActiveCopilotCloudAgentUsers   int `json:"daily_active_copilot_cloud_agent_users"`
	WeeklyActiveCopilotCloudAgentUsers  int `json:"weekly_active_copilot_cloud_agent_users"`
	MonthlyActiveCopilotCloudAgentUsers int `json:"monthly_active_copilot_cloud_agent_users"`

	// Code review user counts
	DailyActiveCopilotCodeReviewUsers    int `json:"daily_active_copilot_code_review_users"`
	DailyPassiveCopilotCodeReviewUsers   int `json:"daily_passive_copilot_code_review_users"`
	WeeklyActiveCopilotCodeReviewUsers   int `json:"weekly_active_copilot_code_review_users"`
	WeeklyPassiveCopilotCodeReviewUsers  int `json:"weekly_passive_copilot_code_review_users"`
	MonthlyActiveCopilotCodeReviewUsers  int `json:"monthly_active_copilot_code_review_users"`
	MonthlyPassiveCopilotCodeReviewUsers int `json:"monthly_passive_copilot_code_review_users"`

	// Chat panel mode breakdown
	ChatPanelAgentMode   int `json:"chat_panel_agent_mode"`
	ChatPanelAskMode     int `json:"chat_panel_ask_mode"`
	ChatPanelCustomMode  int `json:"chat_panel_custom_mode"`
	ChatPanelEditMode    int `json:"chat_panel_edit_mode"`
	ChatPanelPlanMode    int `json:"chat_panel_plan_mode"`
	ChatPanelUnknownMode int `json:"chat_panel_unknown_mode"`

	UserInitiatedInteractionCount int                    `json:"user_initiated_interaction_count"`
	CodeGenerationActivityCount   int                    `json:"code_generation_activity_count"`
	CodeAcceptanceActivityCount   int                    `json:"code_acceptance_activity_count"`
	LocSuggestedToAddSum          int                    `json:"loc_suggested_to_add_sum"`
	LocSuggestedToDeleteSum       int                    `json:"loc_suggested_to_delete_sum"`
	LocAddedSum                   int                    `json:"loc_added_sum"`
	LocDeletedSum                 int                    `json:"loc_deleted_sum"`
	TotalsByIde                   []totalsByIde          `json:"totals_by_ide"`
	TotalsByFeature               []totalsByFeature      `json:"totals_by_feature"`
	TotalsByLanguageFeature       []totalsByLangFeature  `json:"totals_by_language_feature"`
	TotalsByLanguageModel         []totalsByLangModel    `json:"totals_by_language_model"`
	TotalsByModelFeature          []totalsByModelFeature `json:"totals_by_model_feature"`
	TotalsByCli                   *totalsByCli           `json:"totals_by_cli"`
	PullRequests                  *pullRequestStats      `json:"pull_requests"`

	// TotalsByAiAdoptionPhase groups engagement/activity metrics by AI adoption
	// cohort (phase 0-3). Added 2026-05-29, extended with review-cycle metrics
	// 2026-07-07. Only present on enterprise/org reports, not user reports.
	TotalsByAiAdoptionPhase []totalsByAiAdoptionPhase `json:"totals_by_ai_adoption_phase"`
}

type totalsByIde struct {
	Ide                           string `json:"ide"`
	UserInitiatedInteractionCount int    `json:"user_initiated_interaction_count"`
	CodeGenerationActivityCount   int    `json:"code_generation_activity_count"`
	CodeAcceptanceActivityCount   int    `json:"code_acceptance_activity_count"`
	LocSuggestedToAddSum          int    `json:"loc_suggested_to_add_sum"`
	LocSuggestedToDeleteSum       int    `json:"loc_suggested_to_delete_sum"`
	LocAddedSum                   int    `json:"loc_added_sum"`
	LocDeletedSum                 int    `json:"loc_deleted_sum"`
}

type totalsByFeature struct {
	Feature                       string `json:"feature"`
	UserInitiatedInteractionCount int    `json:"user_initiated_interaction_count"`
	CodeGenerationActivityCount   int    `json:"code_generation_activity_count"`
	CodeAcceptanceActivityCount   int    `json:"code_acceptance_activity_count"`
	LocSuggestedToAddSum          int    `json:"loc_suggested_to_add_sum"`
	LocSuggestedToDeleteSum       int    `json:"loc_suggested_to_delete_sum"`
	LocAddedSum                   int    `json:"loc_added_sum"`
	LocDeletedSum                 int    `json:"loc_deleted_sum"`
}

type totalsByLangFeature struct {
	Language                    string `json:"language"`
	Feature                     string `json:"feature"`
	CodeGenerationActivityCount int    `json:"code_generation_activity_count"`
	CodeAcceptanceActivityCount int    `json:"code_acceptance_activity_count"`
	LocSuggestedToAddSum        int    `json:"loc_suggested_to_add_sum"`
	LocSuggestedToDeleteSum     int    `json:"loc_suggested_to_delete_sum"`
	LocAddedSum                 int    `json:"loc_added_sum"`
	LocDeletedSum               int    `json:"loc_deleted_sum"`
}

type totalsByLangModel struct {
	Language                    string `json:"language"`
	Model                       string `json:"model"`
	CodeGenerationActivityCount int    `json:"code_generation_activity_count"`
	CodeAcceptanceActivityCount int    `json:"code_acceptance_activity_count"`
	LocSuggestedToAddSum        int    `json:"loc_suggested_to_add_sum"`
	LocSuggestedToDeleteSum     int    `json:"loc_suggested_to_delete_sum"`
	LocAddedSum                 int    `json:"loc_added_sum"`
	LocDeletedSum               int    `json:"loc_deleted_sum"`
}

type pullRequestStats struct {
	TotalReviewed                   int     `json:"total_reviewed"`
	TotalCreated                    int     `json:"total_created"`
	TotalMerged                     int     `json:"total_merged"`
	MedianMinutesToMerge            float64 `json:"median_minutes_to_merge"`
	TotalSuggestions                int     `json:"total_suggestions"`
	TotalAppliedSuggestions         int     `json:"total_applied_suggestions"`
	TotalCreatedByCopilot           int     `json:"total_created_by_copilot"`
	TotalReviewedByCopilot          int     `json:"total_reviewed_by_copilot"`
	TotalMergedCreatedByCopilot     int     `json:"total_merged_created_by_copilot"`
	TotalMergedReviewedByCopilot    int     `json:"total_merged_reviewed_by_copilot"`
	MedianMinToMergeCopilotAuthored float64 `json:"median_minutes_to_merge_copilot_authored"`
	MedianMinToMergeCopilotReviewed float64 `json:"median_minutes_to_merge_copilot_reviewed"`
	TotalCopilotSuggestions         int     `json:"total_copilot_suggestions"`
	TotalCopilotAppliedSuggestions  int     `json:"total_copilot_applied_suggestions"`

	// CopilotSuggestionsByCommentType breaks total_copilot_suggestions down by
	// the kind of PR review comment Copilot generated (e.g. "suggestion", "explanation").
	CopilotSuggestionsByCommentType []prSuggestionsByCommentType `json:"copilot_suggestions_by_comment_type"`
}

type prSuggestionsByCommentType struct {
	CommentType      string `json:"comment_type"`
	TotalSuggestions int    `json:"total_suggestions"`
	TotalApplied     int    `json:"total_applied_suggestions"`
}

type totalsByCli struct {
	SessionCount        int        `json:"session_count"`
	RequestCount        int        `json:"request_count"`
	PromptCount         int        `json:"prompt_count"`
	TokenUsage          *cliTokens `json:"token_usage"`
	LastKnownCliVersion string     `json:"last_known_cli_version"`
}

type cliTokens struct {
	OutputTokensSum     int     `json:"output_tokens_sum"`
	PromptTokensSum     int     `json:"prompt_tokens_sum"`
	AvgTokensPerRequest float64 `json:"avg_tokens_per_request"`
}

// totalsByAiAdoptionPhase is one entry of the totals_by_ai_adoption_phase
// breakdown on enterprise/org reports. Per-user fields are averages within the
// phase, not sums (per GitHub's docs) — the *_total fields are true sums.
type totalsByAiAdoptionPhase struct {
	Phase   int `json:"phase"`
	Version int `json:"version"`

	EngagedUsers int `json:"engaged_users"`

	AvgUserInitiatedInteractionCount float64 `json:"avg_user_initiated_interaction_count"`
	AvgCodeGenerationActivityCount   float64 `json:"avg_code_generation_activity_count"`
	AvgCodeAcceptanceActivityCount   float64 `json:"avg_code_acceptance_activity_count"`
	AvgLocAddedSum                   float64 `json:"avg_loc_added_sum"`
	AvgLocDeletedSum                 float64 `json:"avg_loc_deleted_sum"`

	AvgPullRequestsCreated  float64 `json:"avg_pull_requests_created"`
	AvgPullRequestsMerged   float64 `json:"avg_pull_requests_merged"`
	AvgPullRequestsReviewed float64 `json:"avg_pull_requests_reviewed"`
	AvgMedianMinutesToMerge float64 `json:"avg_median_minutes_to_merge"`

	// Added 2026-07-07.
	AvgPullRequestsMinutesToReview float64 `json:"avg_pull_requests_minutes_to_review"`
	AvgPullRequestsReviewCycles    float64 `json:"avg_pull_requests_review_cycles"`

	// Added 2026-06-26 — true total rather than a per-user average.
	TotalPullRequestsMerged int `json:"total_pull_requests_merged"`
}

type totalsByModelFeature struct {
	Model                         string `json:"model"`
	Feature                       string `json:"feature"`
	UserInitiatedInteractionCount int    `json:"user_initiated_interaction_count"`
	CodeGenerationActivityCount   int    `json:"code_generation_activity_count"`
	CodeAcceptanceActivityCount   int    `json:"code_acceptance_activity_count"`
	LocSuggestedToAddSum          int    `json:"loc_suggested_to_add_sum"`
	LocSuggestedToDeleteSum       int    `json:"loc_suggested_to_delete_sum"`
	LocAddedSum                   int    `json:"loc_added_sum"`
	LocDeletedSum                 int    `json:"loc_deleted_sum"`
}

// ExtractEnterpriseMetrics parses enterprise report JSON and extracts to tool-layer tables.
func ExtractEnterpriseMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	if !connection.HasEnterprise() {
		taskCtx.GetLogger().Info("No enterprise configured, skipping enterprise metrics extraction")
		return nil
	}

	params := copilotRawParams{
		ConnectionId: data.Options.ConnectionId,
		ScopeId:      data.Options.ScopeId,
		Organization: connection.Organization,
		Endpoint:     connection.Endpoint,
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Table:   rawEnterpriseMetricsTable,
			Options: params,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			// The API returns a flat enterpriseDayTotal object per raw row, not a wrapper.
			var dt enterpriseDayTotal
			if err := errors.Convert(json.Unmarshal(row.Data, &dt)); err != nil {
				return nil, err
			}

			day, parseErr := time.Parse("2006-01-02", dt.Day)
			if parseErr != nil {
				return nil, errors.BadInput.Wrap(parseErr, "invalid day in enterprise report")
			}

			var results []interface{}

			// Main daily metrics
			dailyMetrics := &models.GhCopilotEnterpriseDailyMetrics{
				ConnectionId:            data.Options.ConnectionId,
				ScopeId:                 data.Options.ScopeId,
				Day:                     day,
				EnterpriseId:            dt.EnterpriseId,
				OrganizationId:          dt.OrganizationId,
				DailyActiveUsers:        dt.DailyActiveUsers,
				WeeklyActiveUsers:       dt.WeeklyActiveUsers,
				MonthlyActiveUsers:      dt.MonthlyActiveUsers,
				MonthlyActiveChatUsers:  dt.MonthlyActiveChatUsers,
				MonthlyActiveAgentUsers: dt.MonthlyActiveAgentUsers,
				DailyActiveCliUsers:     dt.DailyActiveCliUsers,

				DailyActiveCopilotCloudAgentUsers:   dt.DailyActiveCopilotCloudAgentUsers,
				WeeklyActiveCopilotCloudAgentUsers:  dt.WeeklyActiveCopilotCloudAgentUsers,
				MonthlyActiveCopilotCloudAgentUsers: dt.MonthlyActiveCopilotCloudAgentUsers,

				DailyActiveCopilotCodeReviewUsers:    dt.DailyActiveCopilotCodeReviewUsers,
				DailyPassiveCopilotCodeReviewUsers:   dt.DailyPassiveCopilotCodeReviewUsers,
				WeeklyActiveCopilotCodeReviewUsers:   dt.WeeklyActiveCopilotCodeReviewUsers,
				WeeklyPassiveCopilotCodeReviewUsers:  dt.WeeklyPassiveCopilotCodeReviewUsers,
				MonthlyActiveCopilotCodeReviewUsers:  dt.MonthlyActiveCopilotCodeReviewUsers,
				MonthlyPassiveCopilotCodeReviewUsers: dt.MonthlyPassiveCopilotCodeReviewUsers,

				ChatPanelAgentMode:   dt.ChatPanelAgentMode,
				ChatPanelAskMode:     dt.ChatPanelAskMode,
				ChatPanelCustomMode:  dt.ChatPanelCustomMode,
				ChatPanelEditMode:    dt.ChatPanelEditMode,
				ChatPanelPlanMode:    dt.ChatPanelPlanMode,
				ChatPanelUnknownMode: dt.ChatPanelUnknownMode,

				CopilotActivityMetrics: models.CopilotActivityMetrics{
					UserInitiatedInteractionCount: dt.UserInitiatedInteractionCount,
					CodeGenerationActivityCount:   dt.CodeGenerationActivityCount,
					CodeAcceptanceActivityCount:   dt.CodeAcceptanceActivityCount,
					LocSuggestedToAddSum:          dt.LocSuggestedToAddSum,
					LocSuggestedToDeleteSum:       dt.LocSuggestedToDeleteSum,
					LocAddedSum:                   dt.LocAddedSum,
					LocDeletedSum:                 dt.LocDeletedSum,
				},
			}
			if dt.TotalsByCli != nil {
				dailyMetrics.CopilotCliMetrics = models.CopilotCliMetrics{
					CliSessionCount:     dt.TotalsByCli.SessionCount,
					CliRequestCount:     dt.TotalsByCli.RequestCount,
					CliPromptCount:      dt.TotalsByCli.PromptCount,
					CliLastKnownVersion: dt.TotalsByCli.LastKnownCliVersion,
				}
				if dt.TotalsByCli.TokenUsage != nil {
					dailyMetrics.CopilotCliMetrics.CliOutputTokenSum = dt.TotalsByCli.TokenUsage.OutputTokensSum
					dailyMetrics.CopilotCliMetrics.CliPromptTokenSum = dt.TotalsByCli.TokenUsage.PromptTokensSum
					dailyMetrics.CopilotCliMetrics.CliAvgTokensPerRequest = dt.TotalsByCli.TokenUsage.AvgTokensPerRequest
				}
			}
			if dt.PullRequests != nil {
				dailyMetrics.PRTotalReviewed = dt.PullRequests.TotalReviewed
				dailyMetrics.PRTotalCreated = dt.PullRequests.TotalCreated
				dailyMetrics.PRTotalMerged = dt.PullRequests.TotalMerged
				dailyMetrics.PRMedianMinutesToMerge = dt.PullRequests.MedianMinutesToMerge
				dailyMetrics.PRTotalSuggestions = dt.PullRequests.TotalSuggestions
				dailyMetrics.PRTotalAppliedSuggestions = dt.PullRequests.TotalAppliedSuggestions
				dailyMetrics.PRTotalCreatedByCopilot = dt.PullRequests.TotalCreatedByCopilot
				dailyMetrics.PRTotalReviewedByCopilot = dt.PullRequests.TotalReviewedByCopilot
				dailyMetrics.PRTotalMergedCreatedByCopilot = dt.PullRequests.TotalMergedCreatedByCopilot
				dailyMetrics.PRTotalMergedReviewedByCopilot = dt.PullRequests.TotalMergedReviewedByCopilot
				dailyMetrics.PRMedianMinToMergeCopilotAuthored = dt.PullRequests.MedianMinToMergeCopilotAuthored
				dailyMetrics.PRMedianMinToMergeCopilotReviewed = dt.PullRequests.MedianMinToMergeCopilotReviewed
				dailyMetrics.PRTotalCopilotSuggestions = dt.PullRequests.TotalCopilotSuggestions
				dailyMetrics.PRTotalCopilotAppliedSuggestions = dt.PullRequests.TotalCopilotAppliedSuggestions

				// Stored as a JSON blob rather than a normalized breakdown table for
				// now — comment-type cardinality is small (suggestion/explanation)
				// and doesn't warrant its own table + primary key the way IDE/feature
				// breakdowns do. Revisit if GitHub adds more comment types.
				if len(dt.PullRequests.CopilotSuggestionsByCommentType) > 0 {
					if b, jsonErr := json.Marshal(dt.PullRequests.CopilotSuggestionsByCommentType); jsonErr == nil {
						dailyMetrics.PRCopilotSuggestionsByCommentType = string(b)
					}
				}
			}
			results = append(results, dailyMetrics)
			// By AI adoption phase
			for _, phase := range dt.TotalsByAiAdoptionPhase {
				results = append(results, &models.GhCopilotMetricsByAiAdoptionPhase{
					ConnectionId:                     data.Options.ConnectionId,
					ScopeId:                          data.Options.ScopeId,
					Day:                              day,
					Phase:                            phase.Phase,
					PhaseVersion:                     phase.Version,
					EngagedUsers:                     phase.EngagedUsers,
					AvgUserInitiatedInteractionCount: phase.AvgUserInitiatedInteractionCount,
					AvgCodeGenerationActivityCount:   phase.AvgCodeGenerationActivityCount,
					AvgCodeAcceptanceActivityCount:   phase.AvgCodeAcceptanceActivityCount,
					AvgLocAddedSum:                   phase.AvgLocAddedSum,
					AvgLocDeletedSum:                 phase.AvgLocDeletedSum,
					AvgPullRequestsCreated:           phase.AvgPullRequestsCreated,
					AvgPullRequestsMerged:            phase.AvgPullRequestsMerged,
					AvgPullRequestsReviewed:          phase.AvgPullRequestsReviewed,
					AvgMedianMinutesToMerge:          phase.AvgMedianMinutesToMerge,
					AvgPullRequestsMinutesToReview:   phase.AvgPullRequestsMinutesToReview,
					AvgPullRequestsReviewCycles:      phase.AvgPullRequestsReviewCycles,
					TotalPullRequestsMerged:          phase.TotalPullRequestsMerged,
				})
			}

			// By IDE
			for _, ide := range dt.TotalsByIde {
				results = append(results, &models.GhCopilotMetricsByIde{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					Ide:          ide.Ide,
					CopilotActivityMetrics: models.CopilotActivityMetrics{
						UserInitiatedInteractionCount: ide.UserInitiatedInteractionCount,
						CodeGenerationActivityCount:   ide.CodeGenerationActivityCount,
						CodeAcceptanceActivityCount:   ide.CodeAcceptanceActivityCount,
						LocSuggestedToAddSum:          ide.LocSuggestedToAddSum,
						LocSuggestedToDeleteSum:       ide.LocSuggestedToDeleteSum,
						LocAddedSum:                   ide.LocAddedSum,
						LocDeletedSum:                 ide.LocDeletedSum,
					},
				})
			}

			// By Feature
			for _, f := range dt.TotalsByFeature {
				results = append(results, &models.GhCopilotMetricsByFeature{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					Feature:      f.Feature,
					CopilotActivityMetrics: models.CopilotActivityMetrics{
						UserInitiatedInteractionCount: f.UserInitiatedInteractionCount,
						CodeGenerationActivityCount:   f.CodeGenerationActivityCount,
						CodeAcceptanceActivityCount:   f.CodeAcceptanceActivityCount,
						LocSuggestedToAddSum:          f.LocSuggestedToAddSum,
						LocSuggestedToDeleteSum:       f.LocSuggestedToDeleteSum,
						LocAddedSum:                   f.LocAddedSum,
						LocDeletedSum:                 f.LocDeletedSum,
					},
				})
			}

			// By Language+Feature
			for _, lf := range dt.TotalsByLanguageFeature {
				results = append(results, &models.GhCopilotMetricsByLanguageFeature{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					Language:     lf.Language,
					Feature:      lf.Feature,
					CopilotCodeMetrics: models.CopilotCodeMetrics{
						CodeGenerationActivityCount: lf.CodeGenerationActivityCount,
						CodeAcceptanceActivityCount: lf.CodeAcceptanceActivityCount,
						LocSuggestedToAddSum:        lf.LocSuggestedToAddSum,
						LocSuggestedToDeleteSum:     lf.LocSuggestedToDeleteSum,
						LocAddedSum:                 lf.LocAddedSum,
						LocDeletedSum:               lf.LocDeletedSum,
					},
				})
			}

			// By Language+Model
			for _, lm := range dt.TotalsByLanguageModel {
				results = append(results, &models.GhCopilotMetricsByLanguageModel{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					Language:     lm.Language,
					Model:        lm.Model,
					CopilotCodeMetrics: models.CopilotCodeMetrics{
						CodeGenerationActivityCount: lm.CodeGenerationActivityCount,
						CodeAcceptanceActivityCount: lm.CodeAcceptanceActivityCount,
						LocSuggestedToAddSum:        lm.LocSuggestedToAddSum,
						LocSuggestedToDeleteSum:     lm.LocSuggestedToDeleteSum,
						LocAddedSum:                 lm.LocAddedSum,
						LocDeletedSum:               lm.LocDeletedSum,
					},
				})
			}

			// By Model+Feature
			for _, mf := range dt.TotalsByModelFeature {
				results = append(results, &models.GhCopilotMetricsByModelFeature{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					Model:        mf.Model,
					Feature:      mf.Feature,
					CopilotActivityMetrics: models.CopilotActivityMetrics{
						UserInitiatedInteractionCount: mf.UserInitiatedInteractionCount,
						CodeGenerationActivityCount:   mf.CodeGenerationActivityCount,
						CodeAcceptanceActivityCount:   mf.CodeAcceptanceActivityCount,
						LocSuggestedToAddSum:          mf.LocSuggestedToAddSum,
						LocSuggestedToDeleteSum:       mf.LocSuggestedToDeleteSum,
						LocAddedSum:                   mf.LocAddedSum,
						LocDeletedSum:                 mf.LocDeletedSum,
					},
				})
			}

			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
