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

package api

import (
	"net/http"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/aireview/tasks"
)

// AnalyzeRequest represents the request body for generating analysis pipeline
type AnalyzeRequest struct {
	ProjectName   string `json:"projectName"`
	RepoId        string `json:"repoId"`
	ScopeConfigId uint64 `json:"scopeConfigId"`
	TimeAfter     string `json:"timeAfter"`
}

// GenerateAnalysisPipeline generates a pipeline configuration for AI review analysis
// @Summary Generate analysis pipeline
// @Description Generate a pipeline configuration to analyze PR comments for AI reviews.
// @Description Submit the returned pipeline to POST /pipelines to execute.
// @Description Use this to re-analyze data after changing scope config patterns.
// @Tags plugins/aireview
// @Accept json
// @Param body body AnalyzeRequest true "Analysis parameters"
// @Success 200 {object} map[string]any
// @Router /plugins/aireview/analyze [post]
func GenerateAnalysisPipeline(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var request AnalyzeRequest

	// Decode request body
	err := api.DecodeMapStruct(input.Body, &request, true)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode request body")
	}

	// Validate: either projectName or repoId is required
	if request.ProjectName == "" && request.RepoId == "" {
		return nil, errors.BadInput.New("either projectName or repoId is required")
	}

	// Build pipeline options
	opts := map[string]any{}
	if request.ProjectName != "" {
		opts["projectName"] = request.ProjectName
	}
	if request.RepoId != "" {
		opts["repoId"] = request.RepoId
	}
	if request.ScopeConfigId != 0 {
		opts["scopeConfigId"] = request.ScopeConfigId
	}
	if request.TimeAfter != "" {
		opts["timeAfter"] = request.TimeAfter
	}

	// Create pipeline plan
	plan := models.PipelinePlan{
		{
			{
				Plugin:  "aireview",
				Options: opts,
				Subtasks: []string{
					tasks.ExtractAiReviewsMeta.Name,
					tasks.ExtractAiReviewFindingsMeta.Name,
					tasks.CalculateFailurePredictionsMeta.Name,
					tasks.CalculatePredictionMetricsMeta.Name,
				},
			},
		},
	}

	// Generate pipeline name
	pipelineName := "AI Review Analysis"
	if request.ProjectName != "" {
		pipelineName = "AI Review Analysis - " + request.ProjectName
	} else if request.RepoId != "" {
		pipelineName = "AI Review Analysis - " + request.RepoId
	}

	// Return the pipeline configuration that can be submitted to /pipelines
	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"name": pipelineName,
			"plan": plan,
			"message": "Submit this configuration to POST /pipelines to execute the analysis. " +
				"Example: curl -X POST http://localhost:8080/pipelines -H 'Content-Type: application/json' -d '<this response>'",
		},
		Status: http.StatusOK,
	}, nil
}
