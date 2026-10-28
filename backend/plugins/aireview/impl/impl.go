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

package impl

import (
	"encoding/json"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/aireview/api"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/apache/devlake/plugins/aireview/models/migrationscripts"
	"github.com/apache/devlake/plugins/aireview/tasks"
)

// Verify interface implementation
var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginModel
	plugin.PluginMetric
	plugin.PluginMigration
	plugin.PluginApi
	plugin.MetricPluginBlueprintV200
} = (*AiReview)(nil)

// AiReview is the main plugin struct
type AiReview struct{}

// Init initializes the plugin
func (p AiReview) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p AiReview) Description() string {
	return "Extract and analyze AI-generated code reviews from pull requests to calculate AI predicted failure metrics"
}

func (p AiReview) Name() string {
	return "aireview"
}

func (p AiReview) RequiredDataEntities() (data []map[string]interface{}, err errors.Error) {
	return []map[string]interface{}{
		{
			"model": "pull_requests",
			"requiredFields": map[string]string{
				"id":     "string",
				"status": "string",
			},
		},
		{
			"model": "pull_request_comments",
			"requiredFields": map[string]string{
				"id":              "string",
				"pull_request_id": "string",
				"body":            "string",
			},
		},
		{
			"model": "commits",
			"requiredFields": map[string]string{
				"sha":     "string",
				"message": "string",
			},
		},
	}, nil
}

func (p AiReview) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.AiReview{},
		&models.AiCommit{},
		&models.AiReviewFinding{},
		&models.AiFailurePrediction{},
		&models.AiPredictionMetrics{},
		&models.AiReviewScopeConfig{},
	}
}

func (p AiReview) IsProjectMetric() bool {
	return true
}

func (p AiReview) RunAfter() ([]string, errors.Error) {
	// Run after GitHub or GitLab plugins have collected PR data
	return []string{"github", "gitlab", "gitextractor"}, nil
}

func (p AiReview) Settings() interface{} {
	return nil
}

func (p AiReview) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.ExtractAiReviewsMeta,
		tasks.ExtractAiCommitsMeta,
		tasks.EnrichGithubReviewReactionsMeta,
		tasks.EnrichGitlabReviewReactionsMeta,
		tasks.ExtractAiReviewFindingsMeta,
		tasks.ConvertAiReviewsMeta,
		tasks.ConvertAiCommitsMeta,
		tasks.MatchSuggestionDiffsMeta,
		tasks.FetchMissingCiJobsMeta,
		tasks.CalculateFailurePredictionsMeta,
		tasks.ConvertFailurePredictionsMeta,
		tasks.CalculatePredictionMetricsMeta,
		tasks.ConvertPredictionMetricsMeta,
	}
}

func (p AiReview) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	logger := taskCtx.GetLogger()
	logger.Debug("Preparing AI Review task data: %v", options)

	op, err := tasks.DecodeTaskOptions(options)
	if err != nil {
		return nil, err
	}

	err = tasks.ValidateTaskOptions(op)
	if err != nil {
		return nil, err
	}

	// Load scope config if ID is provided
	if op.ScopeConfig == nil && op.ScopeConfigId != 0 {
		var scopeConfig models.AiReviewScopeConfig
		db := taskCtx.GetDal()
		dbErr := db.First(&scopeConfig, dal.Where("id = ?", op.ScopeConfigId))
		if dbErr != nil && !db.IsErrorNotFound(dbErr) {
			return nil, errors.BadInput.Wrap(dbErr, "failed to get scopeConfig")
		}
		op.ScopeConfig = &scopeConfig
	}

	// Use default scope config if none provided
	if op.ScopeConfig == nil {
		op.ScopeConfig = models.GetDefaultScopeConfig()
	}

	taskData := &tasks.AiReviewTaskData{
		Options: op,
	}

	// Compile regex patterns
	err = tasks.CompilePatterns(taskData)
	if err != nil {
		return nil, err
	}

	return taskData, nil
}

func (p AiReview) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/aireview"
}

func (p AiReview) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"reviews": {
			"GET": api.GetReviews,
		},
		"reviews/:id": {
			"GET": api.GetReview,
		},
		"stats": {
			"GET": api.GetReviewStats,
		},
		"findings": {
			"GET": api.GetFindings,
		},
		"scope-configs": {
			"GET":  api.GetScopeConfigs,
			"POST": api.CreateScopeConfig,
		},
		"scope-configs/default": {
			"GET": api.GetDefaultScopeConfig,
		},
		"scope-configs/:id": {
			"GET":    api.GetScopeConfig,
			"PATCH":  api.UpdateScopeConfig,
			"DELETE": api.DeleteScopeConfig,
		},
		"analyze": {
			"POST": api.GenerateAnalysisPipeline,
		},
	}
}

func (p AiReview) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

// DecodeAndValidateTaskOptions is a helper for the API
func DecodeAndValidateTaskOptions(options map[string]interface{}) (*tasks.AiReviewOptions, errors.Error) {
	op, err := tasks.DecodeTaskOptions(options)
	if err != nil {
		return nil, err
	}
	err = helper.DecodeMapStruct(options, op, true)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// MakeMetricPluginPipelinePlanV200 generates pipeline plan for project metrics
func (p AiReview) MakeMetricPluginPipelinePlanV200(projectName string, options json.RawMessage) (coreModels.PipelinePlan, errors.Error) {
	op := &tasks.AiReviewOptions{}
	if options != nil && string(options) != "\"\"" {
		err := json.Unmarshal(options, op)
		if err != nil {
			return nil, errors.Default.WrapRaw(err)
		}
	}

	// Build options map with projectName
	opts := map[string]interface{}{
		"projectName": projectName,
	}
	if op.ScopeConfigId != 0 {
		opts["scopeConfigId"] = op.ScopeConfigId
	}

	plan := coreModels.PipelinePlan{
		{
			{
				Plugin:  "aireview",
				Options: opts,
				Subtasks: []string{
					tasks.ExtractAiReviewsMeta.Name,
					tasks.ExtractAiCommitsMeta.Name,
					tasks.EnrichGithubReviewReactionsMeta.Name,
					tasks.EnrichGitlabReviewReactionsMeta.Name,
					tasks.ExtractAiReviewFindingsMeta.Name,
					tasks.ConvertAiReviewsMeta.Name,
					tasks.ConvertAiCommitsMeta.Name,
					tasks.MatchSuggestionDiffsMeta.Name,
					tasks.FetchMissingCiJobsMeta.Name,
					tasks.CalculateFailurePredictionsMeta.Name,
					tasks.ConvertFailurePredictionsMeta.Name,
					tasks.CalculatePredictionMetricsMeta.Name,
					tasks.ConvertPredictionMetricsMeta.Name,
				},
			},
		},
	}
	return plan, nil
}
