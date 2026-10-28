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
	"strconv"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
)

// GetReviews returns a list of AI reviews with optional filtering
// @Summary Get AI reviews
// @Description Get a list of AI-generated code reviews
// @Tags plugins/aireview
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(50)
// @Param repoId query string false "Filter by repository ID"
// @Param projectName query string false "Filter by project name"
// @Param riskLevel query string false "Filter by risk level (high, medium, low)"
// @Param aiTool query string false "Filter by AI tool (coderabbit, cursor-bugbot)"
// @Success 200 {object} map[string]any
// @Router /plugins/aireview/reviews [get]
func GetReviews(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// Parse pagination
	page, _ := strconv.Atoi(input.Query.Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(input.Query.Get("pageSize"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	// Build base query clauses
	var clauses []dal.Clause

	// Project filter requires join with project_mapping
	if projectName := input.Query.Get("projectName"); projectName != "" {
		clauses = []dal.Clause{
			dal.Select("r.*"),
			dal.From("_tool_aireview_reviews r"),
			dal.Join("JOIN project_mapping pm ON r.repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ?", projectName, "repos"),
		}
	} else {
		clauses = []dal.Clause{
			dal.From(&models.AiReview{}),
		}
		// Apply filters
		if repoId := input.Query.Get("repoId"); repoId != "" {
			clauses = append(clauses, dal.Where("repo_id = ?", repoId))
		}
	}

	if riskLevel := input.Query.Get("riskLevel"); riskLevel != "" {
		clauses = append(clauses, dal.Where("risk_level = ?", riskLevel))
	}
	if aiTool := input.Query.Get("aiTool"); aiTool != "" {
		clauses = append(clauses, dal.Where("ai_tool = ?", aiTool))
	}

	// Get total count
	countClauses := make([]dal.Clause, len(clauses))
	copy(countClauses, clauses)
	total, err := db.Count(countClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to count reviews")
	}

	// Get paginated results
	clauses = append(clauses,
		dal.Orderby("created_date DESC"),
		dal.Limit(pageSize),
		dal.Offset(offset),
	)

	var reviews []models.AiReview
	err = db.All(&reviews, clauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query reviews")
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"reviews":  reviews,
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
		Status: http.StatusOK,
	}, nil
}

// GetReview returns a single AI review by ID
// @Summary Get AI review by ID
// @Description Get a single AI-generated code review
// @Tags plugins/aireview
// @Param id path string true "Review ID"
// @Success 200 {object} models.AiReview
// @Router /plugins/aireview/reviews/{id} [get]
func GetReview(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	reviewId := input.Params["id"]
	if reviewId == "" {
		return nil, errors.BadInput.New("review id is required")
	}

	var review models.AiReview
	err := db.First(&review, dal.Where("id = ?", reviewId))
	if err != nil {
		if db.IsErrorNotFound(err) {
			return nil, errors.NotFound.Wrap(err, "review not found")
		}
		return nil, errors.Default.Wrap(err, "failed to get review")
	}

	return &plugin.ApiResourceOutput{
		Body:   review,
		Status: http.StatusOK,
	}, nil
}

// GetReviewStats returns aggregated statistics for AI reviews
// @Summary Get AI review statistics
// @Description Get aggregated statistics for AI-generated code reviews
// @Tags plugins/aireview
// @Param repoId query string false "Filter by repository ID"
// @Param projectName query string false "Filter by project name"
// @Success 200 {object} map[string]any
// @Router /plugins/aireview/stats [get]
func GetReviewStats(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// Build base clauses for filtering
	var baseClauses []dal.Clause

	if projectName := input.Query.Get("projectName"); projectName != "" {
		baseClauses = []dal.Clause{
			dal.From("_tool_aireview_reviews r"),
			dal.Join("JOIN project_mapping pm ON r.repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ?", projectName, "repos"),
		}
	} else {
		baseClauses = []dal.Clause{
			dal.From(&models.AiReview{}),
		}
		if repoId := input.Query.Get("repoId"); repoId != "" {
			baseClauses = append(baseClauses, dal.Where("repo_id = ?", repoId))
		}
	}

	// Get total count
	total, err := db.Count(baseClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to get total count")
	}

	// Risk level counts
	type RiskCount struct {
		RiskLevel string `gorm:"column:risk_level" json:"riskLevel"`
		Count     int64  `gorm:"column:count" json:"count"`
	}
	var riskCounts []RiskCount
	riskClauses := append(baseClauses,
		dal.Select("risk_level, COUNT(*) as count"),
		dal.Groupby("risk_level"),
	)
	err = db.All(&riskCounts, riskClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to get risk counts")
	}

	// AI tool counts
	type ToolCount struct {
		AiTool string `gorm:"column:ai_tool" json:"aiTool"`
		Count  int64  `gorm:"column:count" json:"count"`
	}
	var toolCounts []ToolCount
	toolClauses := append(baseClauses,
		dal.Select("ai_tool, COUNT(*) as count"),
		dal.Groupby("ai_tool"),
	)
	err = db.All(&toolCounts, toolClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to get tool counts")
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"total":       total,
			"byRiskLevel": riskCounts,
			"byAiTool":    toolCounts,
		},
		Status: http.StatusOK,
	}, nil
}

// GetFindings returns a list of AI review findings
// @Summary Get AI review findings
// @Description Get a list of individual findings from AI reviews
// @Tags plugins/aireview
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(50)
// @Param reviewId query string false "Filter by review ID"
// @Param category query string false "Filter by category (security, bug, performance, etc.)"
// @Param severity query string false "Filter by severity (critical, major, minor, info)"
// @Success 200 {object} map[string]any
// @Router /plugins/aireview/findings [get]
func GetFindings(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// Parse pagination
	page, _ := strconv.Atoi(input.Query.Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(input.Query.Get("pageSize"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	// Build query clauses
	clauses := []dal.Clause{
		dal.From(&models.AiReviewFinding{}),
	}

	// Apply filters
	if reviewId := input.Query.Get("reviewId"); reviewId != "" {
		clauses = append(clauses, dal.Where("review_id = ?", reviewId))
	}
	if category := input.Query.Get("category"); category != "" {
		clauses = append(clauses, dal.Where("category = ?", category))
	}
	if severity := input.Query.Get("severity"); severity != "" {
		clauses = append(clauses, dal.Where("severity = ?", severity))
	}

	// Get total count
	total, err := db.Count(clauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to count findings")
	}

	// Get paginated results
	clauses = append(clauses,
		dal.Orderby("id DESC"),
		dal.Limit(pageSize),
		dal.Offset(offset),
	)

	var findings []models.AiReviewFinding
	err = db.All(&findings, clauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query findings")
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"findings": findings,
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
		Status: http.StatusOK,
	}, nil
}
