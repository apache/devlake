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
	"testing"
	"time"

	"github.com/apache/devlake/plugins/agentready/models"
)

func TestParseAssessmentJSON(t *testing.T) {
	rawJSON := `{
		"schema_version": "1.0.0",
		"repository": {
			"name": "my-repo",
			"branch": "main",
			"commit_hash": "abc123def456abc123def456abc123def456abc1"
		},
		"timestamp": "2026-05-10T14:30:00Z",
		"overall_score": 85.5,
		"certification_level": "Gold",
		"attributes_assessed": 20,
		"attributes_total": 25,
		"duration_seconds": 12.3,
		"findings": [
			{
				"attribute": {
					"id": "doc-readme",
					"name": "README Quality",
					"category": "Documentation Standards",
					"tier": 1,
					"default_weight": 0.8
				},
				"status": "pass",
				"score": 100.0,
				"measured_value": "README exists with 500 words",
				"threshold": "README with >100 words",
				"evidence": ["README.md found", "500 words detected"]
			},
			{
				"attribute": {
					"id": "sec-secrets",
					"name": "No Secrets",
					"category": "Security",
					"tier": 2,
					"default_weight": 0.9
				},
				"status": "fail",
				"score": 30.0,
				"measured_value": "2 potential secrets found",
				"threshold": "0 secrets",
				"evidence": [".env file contains API_KEY"],
				"remediation": {
					"summary": "Remove secrets from codebase",
					"steps": ["Add .env to .gitignore", "Rotate exposed keys"]
				}
			},
			{
				"attribute": {
					"id": "na-attr",
					"name": "N/A Attribute",
					"category": "Other",
					"tier": 4,
					"default_weight": 0.1
				},
				"status": "not_applicable"
			}
		]
	}`

	assessment := &models.AgentReadyAssessment{
		RepoId:       "github:GithubRepo:1:123",
		ConnectionId: 1,
		Provider:     "github",
		RepoName:     "myorg/my-repo",
		CollectedAt:  time.Now(),
		RawJSON:      rawJSON,
	}

	parsed, parseErr := parseRawAssessment(assessment.RawJSON)
	if parseErr != nil {
		t.Fatalf("unexpected error: %v", parseErr)
	}
	result, err := parseAssessmentJSON(assessment, parsed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OverallScore != 85.5 {
		t.Errorf("OverallScore = %v, want 85.5", result.OverallScore)
	}
	if result.CertificationLevel != "Gold" {
		t.Errorf("CertificationLevel = %v, want Gold", result.CertificationLevel)
	}
	if result.CommitHash != "abc123def456abc123def456abc123def456abc1" {
		t.Errorf("CommitHash = %v, want abc123...", result.CommitHash)
	}
	if result.Id != "github:GithubRepo:1:123:abc123def456abc123def456abc123def456abc1" {
		t.Errorf("Id = %v, want composite key with repo:commit", result.Id)
	}
	if result.SchemaVersion != "1.0.0" {
		t.Errorf("SchemaVersion = %v, want 1.0.0", result.SchemaVersion)
	}
	if result.AttributesAssessed != 20 {
		t.Errorf("AttributesAssessed = %v, want 20", result.AttributesAssessed)
	}
	if result.Branch != "main" {
		t.Errorf("Branch = %v, want main", result.Branch)
	}
}

func TestParseFindings(t *testing.T) {
	rawJSON := `{
		"schema_version": "1.0.0",
		"repository": {"name": "r", "branch": "main", "commit_hash": "aaa"},
		"timestamp": "2026-05-10T14:30:00Z",
		"overall_score": 50,
		"certification_level": "Bronze",
		"attributes_assessed": 2,
		"attributes_total": 3,
		"duration_seconds": 1,
		"findings": [
			{
				"attribute": {"id": "a1", "name": "A1", "category": "Cat1", "tier": 1, "default_weight": 0.5},
				"status": "pass",
				"score": 100.0,
				"evidence": ["ok"]
			},
			{
				"attribute": {"id": "a2", "name": "A2", "category": "Cat2", "tier": 2, "default_weight": 0.7},
				"status": "fail",
				"score": 30.0,
				"remediation": {"summary": "Fix it", "steps": ["step1", "step2"]}
			},
			{
				"attribute": {"id": "a3", "name": "A3", "category": "Cat3", "tier": 3, "default_weight": 0.1},
				"status": "not_applicable"
			}
		]
	}`

	assessmentId := "repo1:aaa"
	repoId := "repo1"

	parsed, parseErr := parseRawAssessment(rawJSON)
	if parseErr != nil {
		t.Fatalf("unexpected error: %v", parseErr)
	}
	findings, err := parseFindings(parsed, assessmentId, repoId, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (not_applicable filtered), got %d", len(findings))
	}

	f1 := findings[0]
	if f1.AttributeId != "a1" {
		t.Errorf("finding[0].AttributeId = %v, want a1", f1.AttributeId)
	}
	if f1.Status != "pass" {
		t.Errorf("finding[0].Status = %v, want pass", f1.Status)
	}
	if f1.Score == nil || *f1.Score != 100.0 {
		t.Errorf("finding[0].Score = %v, want 100.0", f1.Score)
	}

	f2 := findings[1]
	if f2.RemediationSummary != "Fix it" {
		t.Errorf("finding[1].RemediationSummary = %v, want 'Fix it'", f2.RemediationSummary)
	}
	if f2.Tier != 2 {
		t.Errorf("finding[1].Tier = %v, want 2", f2.Tier)
	}
}
