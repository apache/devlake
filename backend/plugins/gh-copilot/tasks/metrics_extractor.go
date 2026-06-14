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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/gh-copilot/models"
)

// Seat response structs (used by seat_extractor.go)

type copilotSeatResponse struct {
	CreatedAt               string          `json:"created_at"`
	UpdatedAt               string          `json:"updated_at"`
	PlanType                string          `json:"plan_type"`
	PendingCancellationDate *string         `json:"pending_cancellation_date"`
	LastAuthenticatedAt     *string         `json:"last_authenticated_at"`
	LastActivityAt          *string         `json:"last_activity_at"`
	LastActivityEditor      string          `json:"last_activity_editor"`
	Assignee                copilotAssignee `json:"assignee"`
	AssigningTeam           *copilotTeam    `json:"assigning_team"`
}

type copilotAssignee struct {
	Login string `json:"login"`
	Id    int64  `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type copilotTeam struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ExtractOrgMetrics parses org report data from the new report download API.
// The org report uses the same flat format as enterprise reports (day, totals_by_*).
// It writes to the same unified tables as ExtractEnterpriseMetrics so the
// Grafana dashboard works identically for org-only and enterprise connections.
func ExtractOrgMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	if connection.Organization == "" {
		taskCtx.GetLogger().Info("No organization configured, skipping org metrics extraction")
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
			Table:   rawOrgMetricsTable,
			Options: params,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			var dt enterpriseDayTotal
			if err := errors.Convert(json.Unmarshal(row.Data, &dt)); err != nil {
				return nil, err
			}

			day, parseErr := time.Parse("2006-01-02", dt.Day)
			if parseErr != nil {
				return nil, errors.BadInput.Wrap(parseErr, "invalid day in org report")
			}

			var results []interface{}

			// Main daily metrics — same model as enterprise extractor
			dailyMetrics := &models.GhCopilotEnterpriseDailyMetrics{
				ConnectionId:            data.Options.ConnectionId,
				ScopeId:                 data.Options.ScopeId,
				Day:                     day,
				EnterpriseId:            "", // org-level, no enterprise
				DailyActiveUsers:        dt.DailyActiveUsers,
				WeeklyActiveUsers:       dt.WeeklyActiveUsers,
				MonthlyActiveUsers:      dt.MonthlyActiveUsers,
				MonthlyActiveChatUsers:  dt.MonthlyActiveChatUsers,
				MonthlyActiveAgentUsers: dt.MonthlyActiveAgentUsers,
				DailyActiveCliUsers:     dt.DailyActiveCliUsers,

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
					CliSessionCount: dt.TotalsByCli.SessionCount,
					CliRequestCount: dt.TotalsByCli.RequestCount,
					CliPromptCount:  dt.TotalsByCli.PromptCount,
				}
				if dt.TotalsByCli.TokenUsage != nil {
					dailyMetrics.CopilotCliMetrics.CliOutputTokenSum = dt.TotalsByCli.TokenUsage.OutputTokensSum
					dailyMetrics.CopilotCliMetrics.CliPromptTokenSum = dt.TotalsByCli.TokenUsage.PromptTokensSum
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
			}
			results = append(results, dailyMetrics)

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
