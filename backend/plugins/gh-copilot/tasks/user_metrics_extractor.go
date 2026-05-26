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

// --- User report JSONL structures (one line per user) ---

type userDailyReport struct {
	ReportStartDay                string                 `json:"report_start_day"`
	ReportEndDay                  string                 `json:"report_end_day"`
	Day                           string                 `json:"day"`
	OrganizationId                string                 `json:"organization_id"`
	EnterpriseId                  string                 `json:"enterprise_id"`
	UserId                        int64                  `json:"user_id"`
	UserLogin                     string                 `json:"user_login"`
	UserInitiatedInteractionCount int                    `json:"user_initiated_interaction_count"`
	CodeGenerationActivityCount   int                    `json:"code_generation_activity_count"`
	CodeAcceptanceActivityCount   int                    `json:"code_acceptance_activity_count"`
	LocSuggestedToAddSum          int                    `json:"loc_suggested_to_add_sum"`
	LocSuggestedToDeleteSum       int                    `json:"loc_suggested_to_delete_sum"`
	LocAddedSum                   int                    `json:"loc_added_sum"`
	LocDeletedSum                 int                    `json:"loc_deleted_sum"`
	UsedAgent                     bool                   `json:"used_agent"`
	UsedChat                      bool                   `json:"used_chat"`
	TotalsByIde                   []userTotalsByIde      `json:"totals_by_ide"`
	TotalsByFeature               []totalsByFeature      `json:"totals_by_feature"`
	TotalsByLanguageFeature       []totalsByLangFeature  `json:"totals_by_language_feature"`
	TotalsByLanguageModel         []totalsByLangModel    `json:"totals_by_language_model"`
	TotalsByModelFeature          []totalsByModelFeature `json:"totals_by_model_feature"`
}

type userTotalsByIde struct {
	totalsByIde
	LastKnownPluginVersion *pluginVersion `json:"last_known_plugin_version"`
	LastKnownIdeVersion    *ideVersion    `json:"last_known_ide_version"`
}

type pluginVersion struct {
	SampledAt     string `json:"sampled_at"`
	Plugin        string `json:"plugin"`
	PluginVersion string `json:"plugin_version"`
}

type ideVersion struct {
	SampledAt  string `json:"sampled_at"`
	IdeVersion string `json:"ide_version"`
}

// ExtractUserMetrics parses user report JSONL records and extracts to tool-layer tables.
func ExtractUserMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	params := copilotRawParams{
		ConnectionId: data.Options.ConnectionId,
		ScopeId:      data.Options.ScopeId,
		Organization: connection.Organization,
		Endpoint:     connection.Endpoint,
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Table:   rawUserMetricsTable,
			Options: params,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			var u userDailyReport
			if err := errors.Convert(json.Unmarshal(row.Data, &u)); err != nil {
				return nil, err
			}

			day, parseErr := time.Parse("2006-01-02", u.Day)
			if parseErr != nil {
				return nil, errors.BadInput.Wrap(parseErr, "invalid day in user report")
			}

			var results []interface{}

			// Main user daily metrics
			results = append(results, &models.GhCopilotUserDailyMetrics{
				ConnectionId:   data.Options.ConnectionId,
				ScopeId:        data.Options.ScopeId,
				Day:            day,
				UserId:         u.UserId,
				OrganizationId: u.OrganizationId,
				EnterpriseId:   u.EnterpriseId,
				UserLogin:      u.UserLogin,
				UsedAgent:      u.UsedAgent,
				UsedChat:       u.UsedChat,
				CopilotActivityMetrics: models.CopilotActivityMetrics{
					UserInitiatedInteractionCount: u.UserInitiatedInteractionCount,
					CodeGenerationActivityCount:   u.CodeGenerationActivityCount,
					CodeAcceptanceActivityCount:   u.CodeAcceptanceActivityCount,
					LocSuggestedToAddSum:          u.LocSuggestedToAddSum,
					LocSuggestedToDeleteSum:       u.LocSuggestedToDeleteSum,
					LocAddedSum:                   u.LocAddedSum,
					LocDeletedSum:                 u.LocDeletedSum,
				},
			})

			// User by IDE
			for _, ide := range u.TotalsByIde {
				rec := &models.GhCopilotUserMetricsByIde{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					UserId:       u.UserId,
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
				}
				if ide.LastKnownPluginVersion != nil {
					rec.LastKnownPluginName = ide.LastKnownPluginVersion.Plugin
					rec.LastKnownPluginVersion = ide.LastKnownPluginVersion.PluginVersion
				}
				if ide.LastKnownIdeVersion != nil {
					rec.LastKnownIdeVersion = ide.LastKnownIdeVersion.IdeVersion
				}
				results = append(results, rec)
			}

			// User by Feature
			for _, f := range u.TotalsByFeature {
				results = append(results, &models.GhCopilotUserMetricsByFeature{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					UserId:       u.UserId,
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

			// User by Language+Feature
			for _, lf := range u.TotalsByLanguageFeature {
				results = append(results, &models.GhCopilotUserMetricsByLanguageFeature{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					UserId:       u.UserId,
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

			// User by Language+Model
			for _, lm := range u.TotalsByLanguageModel {
				results = append(results, &models.GhCopilotUserMetricsByLanguageModel{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					UserId:       u.UserId,
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

			// User by Model+Feature
			for _, mf := range u.TotalsByModelFeature {
				results = append(results, &models.GhCopilotUserMetricsByModelFeature{
					ConnectionId: data.Options.ConnectionId,
					ScopeId:      data.Options.ScopeId,
					Day:          day,
					UserId:       u.UserId,
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
