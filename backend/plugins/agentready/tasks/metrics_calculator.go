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
	"fmt"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/agentready/models"
)

var CalculateMetricsMeta = plugin.SubTaskMeta{
	Name:             "calculateMetrics",
	EntryPoint:       CalculateMetrics,
	EnabledByDefault: true,
	Description:      "Compute aggregated pass rates and category scores per assessment",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAssessmentsMeta},
}

func CalculateMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AgentReadyTaskData)

	connectionId := data.Connection.ID
	repoId := data.Options.FullName

	var assessments []models.AgentReadyAssessment
	err := db.All(&assessments,
		dal.From(&models.AgentReadyAssessment{}),
		dal.Where("connection_id = ? AND repo_id = ?", connectionId, repoId),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query assessments")
	}

	if len(assessments) == 0 {
		logger.Info("No assessments found for scope %s, skipping metrics", repoId)
		return nil
	}

	logger.Info("Calculating metrics for %d assessments", len(assessments))
	taskCtx.SetProgress(0, len(assessments))

	assessmentIds := make([]string, 0, len(assessments))
	for _, a := range assessments {
		assessmentIds = append(assessmentIds, a.Id)
	}

	var allFindings []*models.AgentReadyFinding
	err = db.All(&allFindings,
		dal.From(&models.AgentReadyFinding{}),
		dal.Where("connection_id = ? AND assessment_id IN (?)", connectionId, assessmentIds),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query findings")
	}

	findingsByAssessment := map[string][]*models.AgentReadyFinding{}
	for _, f := range allFindings {
		findingsByAssessment[f.AssessmentId] = append(findingsByAssessment[f.AssessmentId], f)
	}

	for _, assessment := range assessments {
		findings := findingsByAssessment[assessment.Id]
		metric, calcErr := CalculateMetricsFromFindings(findings)
		if calcErr != nil {
			logger.Warn(nil, "Failed to calculate metrics for assessment %s: %v", assessment.Id, calcErr)
		}
		metric.Id = fmt.Sprintf("%s:%s:%s", assessment.RepoId, assessment.AssessedAt.Format("20060102T150405"), assessment.Id)
		metric.ConnectionId = connectionId
		metric.RepoId = assessment.RepoId
		metric.AssessedAt = assessment.AssessedAt

		if dbErr := db.CreateOrUpdate(metric); dbErr != nil {
			logger.Warn(dbErr, "Failed to save metric %s", metric.Id)
		}
		taskCtx.IncProgress(1)
	}

	return nil
}

func CalculateMetricsFromFindings(findings []*models.AgentReadyFinding) (*models.AgentReadyMetric, error) {
	metric := &models.AgentReadyMetric{}

	tierPass := map[int]int{1: 0, 2: 0, 3: 0, 4: 0}
	tierTotal := map[int]int{1: 0, 2: 0, 3: 0, 4: 0}
	catScoreSum := map[string]float64{}
	catCount := map[string]int{}

	for _, f := range findings {
		switch f.Status {
		case models.FindingStatusPass:
			metric.PassCount++
			tierPass[f.Tier]++
			tierTotal[f.Tier]++
		case models.FindingStatusFail:
			metric.FailCount++
			tierTotal[f.Tier]++
		default:
			metric.SkipCount++
		}

		if f.Score != nil {
			catScoreSum[f.Category] += *f.Score
			catCount[f.Category]++
		}
	}

	metric.Tier1PassRate = tierPassRate(tierPass[1], tierTotal[1])
	metric.Tier2PassRate = tierPassRate(tierPass[2], tierTotal[2])
	metric.Tier3PassRate = tierPassRate(tierPass[3], tierTotal[3])
	metric.Tier4PassRate = tierPassRate(tierPass[4], tierTotal[4])

	catAvg := map[string]float64{}
	for cat, sum := range catScoreSum {
		if catCount[cat] > 0 {
			catAvg[cat] = sum / float64(catCount[cat])
		}
	}
	catJSON, marshalErr := json.Marshal(catAvg)
	if marshalErr != nil {
		return metric, fmt.Errorf("marshaling category scores: %w", marshalErr)
	}
	metric.CategoryScores = string(catJSON)

	return metric, nil
}

func tierPassRate(pass, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(pass) / float64(total) * 100
}
