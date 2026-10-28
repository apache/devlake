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
	"github.com/apache/devlake/core/models/common"
)

// AiReviewScopeConfig contains configuration for AI review extraction
type AiReviewScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`

	// CodeRabbit detection patterns
	CodeRabbitEnabled  bool   `mapstructure:"codeRabbitEnabled" json:"codeRabbitEnabled" gorm:"type:boolean"`
	CodeRabbitUsername string `mapstructure:"codeRabbitUsername" json:"codeRabbitUsername" gorm:"type:varchar(255)"`
	CodeRabbitPattern  string `mapstructure:"codeRabbitPattern" json:"codeRabbitPattern" gorm:"type:varchar(500)"`

	// Cursor Bugbot detection patterns (future)
	CursorBugbotEnabled  bool   `mapstructure:"cursorBugbotEnabled" json:"cursorBugbotEnabled" gorm:"type:boolean"`
	CursorBugbotUsername string `mapstructure:"cursorBugbotUsername" json:"cursorBugbotUsername" gorm:"type:varchar(255)"`
	CursorBugbotPattern  string `mapstructure:"cursorBugbotPattern" json:"cursorBugbotPattern" gorm:"type:varchar(500)"`

	// Qodo (formerly Codium) detection patterns
	QodoEnabled  bool   `mapstructure:"qodoEnabled" json:"qodoEnabled" gorm:"type:boolean"`
	QodoUsername string `mapstructure:"qodoUsername" json:"qodoUsername" gorm:"type:varchar(255)"`
	QodoPattern  string `mapstructure:"qodoPattern" json:"qodoPattern" gorm:"type:varchar(500)"`

	// Gemini Code Assist detection patterns
	GeminiEnabled  bool   `mapstructure:"geminiEnabled" json:"geminiEnabled" gorm:"type:boolean"`
	GeminiUsername string `mapstructure:"geminiUsername" json:"geminiUsername" gorm:"type:varchar(255)"`
	GeminiPattern  string `mapstructure:"geminiPattern" json:"geminiPattern" gorm:"type:varchar(500)"`

	// Generic AI detection patterns (for commit messages, PR descriptions)
	// AiCommitPatterns is a single extra catch-all regex (not comma-separated)
	// applied after built-in Co-Authored-By / Assisted-by / Made-with trailer matchers.
	AiCommitPatterns string `mapstructure:"aiCommitPatterns" json:"aiCommitPatterns" gorm:"type:text"`
	AiPrLabelPattern string `mapstructure:"aiPrLabelPattern" json:"aiPrLabelPattern" gorm:"type:varchar(500)"`

	// Risk detection configuration
	RiskHighPattern   string `mapstructure:"riskHighPattern" json:"riskHighPattern" gorm:"type:varchar(500)"`
	RiskMediumPattern string `mapstructure:"riskMediumPattern" json:"riskMediumPattern" gorm:"type:varchar(500)"`
	RiskLowPattern    string `mapstructure:"riskLowPattern" json:"riskLowPattern" gorm:"type:varchar(500)"`

	// Failure tracking configuration
	ObservationWindowDays int `mapstructure:"observationWindowDays" json:"observationWindowDays"` // Default 14 days

	// CI failure prediction threshold: PRs with risk_score >= this are "flagged risky"
	// Used by calculateFailurePredictions to classify TP/FP/FN/TN against actual CI outcomes
	WarningThreshold int `mapstructure:"warningThreshold" json:"warningThreshold"` // Default 50

	// CiFailureSource controls which CI data is used to determine actual failures.
	// "test_cases": join ci_test_cases with flaky-test quarantine (accurate, needs full collection)
	// "job_result": use ci_test_jobs.result directly (fast, works without artifact collection)
	// "both": compute predictions for both sources (stored with ci_failure_source tag)
	CiFailureSource string `mapstructure:"ciFailureSource" json:"ciFailureSource" gorm:"type:varchar(20);default:'both'"`

	// Issue linking patterns (for tracking post-merge bugs)
	BugLinkPattern string `mapstructure:"bugLinkPattern" json:"bugLinkPattern" gorm:"type:varchar(500)"`

	// ExcludeFlakyTests excludes tests/jobs that also fail on periodic/push runs
	// from CI failure determination. When enabled, only non-flaky failures count
	// toward the confusion matrix. Off by default.
	ExcludeFlakyTests bool `mapstructure:"excludeFlakyTests" json:"excludeFlakyTests" gorm:"type:boolean;default:false"`

	// ExcludeInfraFailures excludes CI job failures where no test cases actually
	// failed (indicating infrastructure issues like cluster provisioning, quota,
	// or timeout). Only applies to the job_result CI source. Off by default.
	ExcludeInfraFailures bool `mapstructure:"excludeInfraFailures" json:"excludeInfraFailures" gorm:"type:boolean;default:false"`

	// CiBackfillEnabled enables fetching CI job results from GCS for PRs that
	// have AI reviews but no CI data. Disabled by default because it requires
	// network access to the public Openshift CI GCS bucket.
	CiBackfillEnabled bool `mapstructure:"ciBackfillEnabled" json:"ciBackfillEnabled" gorm:"type:boolean;default:false"`

	// CiBackfillDays controls how many days back to backfill CI data from GCS.
	// 0 (the default) disables backfill. The task derives enabled/disabled from
	// this value: CiBackfillDays > 0 means backfill is active.
	CiBackfillDays int `mapstructure:"ciBackfillDays" json:"ciBackfillDays" gorm:"default:0"`
}

// CI failure source constants
const (
	CiSourceTestCases = "test_cases" // Use ci_test_cases with flaky-test quarantine
	CiSourceJobResult = "job_result" // Use ci_test_jobs.result directly
	CiSourceBoth      = "both"       // Compute predictions for both sources
)

func (AiReviewScopeConfig) TableName() string {
	return "_tool_aireview_scope_configs"
}

// GetDefaultScopeConfig returns a scope config with sensible defaults
func GetDefaultScopeConfig() *AiReviewScopeConfig {
	return &AiReviewScopeConfig{
		CodeRabbitEnabled:     true,
		CodeRabbitUsername:    "coderabbitai",
		CodeRabbitPattern:     `(?i)(coderabbit|walkthrough|summary by coderabbit)`,
		CursorBugbotEnabled:   false,
		CursorBugbotUsername:  "cursor-bugbot",
		CursorBugbotPattern:   `(?i)(cursor|bugbot)`,
		QodoEnabled:           true,
		QodoUsername:          "qodo-merge",
		QodoPattern:           `(?i)(qodo|pr reviewer guide|estimated effort to review)`,
		GeminiEnabled:         true,
		GeminiUsername:        "gemini-code-assist",
		GeminiPattern:         `(?i)(I'm Gemini Code Assist|codereviewagent|gstatic\.com/codereviewagent)`,
		AiCommitPatterns:      `(?i)(?:co-authored-by|assisted-by|made-with):.*(?:\bai\b|chatgpt|openai)`,
		AiPrLabelPattern:      `(?i)(ai-reviewed|coderabbit|automated-review)`,
		RiskHighPattern:       `(?i)(critical|security|breaking|major)`,
		RiskMediumPattern:     `(?i)(warning|medium|moderate)`,
		RiskLowPattern:        `(?i)(minor|low|info|suggestion)`,
		ObservationWindowDays: 14,
		WarningThreshold:      50,
		CiFailureSource:       CiSourceBoth,
		BugLinkPattern:        `(?i)(fixes|closes|resolves)\s*#(\d+)`,
	}
}
