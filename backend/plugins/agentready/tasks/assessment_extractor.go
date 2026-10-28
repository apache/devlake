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
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/agentready/models"
)

var ExtractAssessmentsMeta = plugin.SubTaskMeta{
	Name:             "extractAssessments",
	EntryPoint:       ExtractAssessments,
	EnabledByDefault: true,
	Description:      "Parse raw assessment JSON into structured assessment and finding rows",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&CollectSubmissionsMeta},
}

type assessmentJSON struct {
	SchemaVersion      string         `json:"schema_version"`
	Repository         repositoryJSON `json:"repository"`
	Timestamp          string         `json:"timestamp"`
	OverallScore       float64        `json:"overall_score"`
	CertificationLevel string         `json:"certification_level"`
	AttributesAssessed int            `json:"attributes_assessed"`
	AttributesTotal    int            `json:"attributes_total"`
	DurationSeconds    float64        `json:"duration_seconds"`
	Findings           []findingJSON  `json:"findings"`
}

type repositoryJSON struct {
	Name       string `json:"name"`
	Branch     string `json:"branch"`
	CommitHash string `json:"commit_hash"`
}

type findingJSON struct {
	Attribute   attributeJSON    `json:"attribute"`
	Status      string           `json:"status"`
	Score       *float64         `json:"score"`
	MeasuredVal string           `json:"measured_value"`
	Threshold   string           `json:"threshold"`
	Evidence    []string         `json:"evidence"`
	Remediation *remediationJSON `json:"remediation"`
}

type attributeJSON struct {
	Id            string  `json:"id"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	Tier          int     `json:"tier"`
	DefaultWeight float64 `json:"default_weight"`
}

type remediationJSON struct {
	Summary string   `json:"summary"`
	Steps   []string `json:"steps"`
}

func ExtractAssessments(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AgentReadyTaskData)

	connectionId := data.Connection.ID
	repoId := data.Options.FullName

	var rawAssessments []models.AgentReadyAssessment
	err := db.All(&rawAssessments,
		dal.From(&models.AgentReadyAssessment{}),
		dal.Where("connection_id = ? AND repo_id = ? AND raw_json != ''", connectionId, repoId),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query raw assessments")
	}

	logger.Info("Extracting %d assessments for scope %s", len(rawAssessments), repoId)
	taskCtx.SetProgress(0, len(rawAssessments))

	for i := range rawAssessments {
		parsed, parseErr := parseRawAssessment(rawAssessments[i].RawJSON)
		if parseErr != nil {
			logger.Warn(nil, "Failed to parse assessment for %s: %v", repoId, parseErr)
			taskCtx.IncProgress(1)
			continue
		}
		assessment, assessErr := parseAssessmentJSON(&rawAssessments[i], parsed)
		if assessErr != nil {
			logger.Warn(nil, "Failed to extract assessment for %s: %v", repoId, assessErr)
			taskCtx.IncProgress(1)
			continue
		}

		if dbErr := db.CreateOrUpdate(assessment); dbErr != nil {
			logger.Warn(dbErr, "Failed to save parsed assessment %s", assessment.Id)
		}

		findings, findErr := parseFindings(parsed, assessment.Id, assessment.RepoId, connectionId)
		if findErr != nil {
			logger.Warn(nil, "Failed to parse findings for %s: %v", repoId, findErr)
		}
		for _, f := range findings {
			if dbErr := db.CreateOrUpdate(f); dbErr != nil {
				logger.Warn(dbErr, "Failed to save finding %s", f.Id)
			}
		}

		taskCtx.IncProgress(1)
	}

	return nil
}

func parseAssessmentJSON(assessment *models.AgentReadyAssessment, parsed *assessmentJSON) (*models.AgentReadyAssessment, error) {
	assessedAt, err := time.Parse(time.RFC3339, parsed.Timestamp)
	if err != nil {
		assessedAt = assessment.CollectedAt
	}

	assessment.Id = fmt.Sprintf("%s:%s", assessment.RepoId, parsed.Repository.CommitHash)
	assessment.SchemaVersion = parsed.SchemaVersion
	assessment.OverallScore = parsed.OverallScore
	assessment.CertificationLevel = parsed.CertificationLevel
	assessment.AttributesAssessed = parsed.AttributesAssessed
	assessment.AttributesTotal = parsed.AttributesTotal
	assessment.Branch = parsed.Repository.Branch
	assessment.CommitHash = parsed.Repository.CommitHash
	assessment.DurationSeconds = parsed.DurationSeconds
	assessment.AssessedAt = assessedAt

	return assessment, nil
}

func parseFindings(parsed *assessmentJSON, assessmentId, repoId string, connectionId uint64) ([]*models.AgentReadyFinding, error) {
	var findings []*models.AgentReadyFinding
	for _, f := range parsed.Findings {
		if f.Status == models.FindingStatusNotApplicable {
			continue
		}

		finding := &models.AgentReadyFinding{
			Id:            fmt.Sprintf("%s:%s", assessmentId, f.Attribute.Id),
			ConnectionId:  connectionId,
			AssessmentId:  assessmentId,
			RepoId:        repoId,
			AttributeId:   f.Attribute.Id,
			AttributeName: f.Attribute.Name,
			Category:      f.Attribute.Category,
			Tier:          f.Attribute.Tier,
			Status:        f.Status,
			Score:         f.Score,
			MeasuredValue: f.MeasuredVal,
			Threshold:     f.Threshold,
			DefaultWeight: f.Attribute.DefaultWeight,
		}

		if len(f.Evidence) > 0 {
			evidenceJSON, marshalErr := json.Marshal(f.Evidence)
			if marshalErr == nil {
				finding.Evidence = string(evidenceJSON)
			}
		}

		if f.Remediation != nil {
			finding.RemediationSummary = f.Remediation.Summary
			if len(f.Remediation.Steps) > 0 {
				stepsJSON, marshalErr := json.Marshal(f.Remediation.Steps)
				if marshalErr == nil {
					finding.RemediationSteps = string(stepsJSON)
				}
			}
		}

		findings = append(findings, finding)
	}

	return findings, nil
}

func parseRawAssessment(rawJSON string) (*assessmentJSON, error) {
	var parsed assessmentJSON
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		return nil, fmt.Errorf("parsing assessment JSON: %w", err)
	}
	return &parsed, nil
}
