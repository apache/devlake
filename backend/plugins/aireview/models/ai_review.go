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
	"time"

	"github.com/apache/devlake/core/models/common"
)

// AiReview represents an AI-generated code review on a pull request
type AiReview struct {
	common.NoPKModel

	// Primary key fields
	Id string `gorm:"primaryKey;type:varchar(255)"`

	// Foreign key to pull_requests domain table
	PullRequestId string `gorm:"index;type:varchar(255)"`

	// Repository reference
	RepoId string `gorm:"index;type:varchar(255)"`

	// AI tool information
	AiTool     string `gorm:"type:varchar(100)"` // coderabbit, cursor_bugbot, etc.
	AiToolUser string `gorm:"type:varchar(255)"` // Bot username

	// Review metadata
	ReviewId    string    `gorm:"type:varchar(255)"` // Original review/comment ID from source
	Body        string    `gorm:"type:longtext"`     // Full review body
	Summary     string    `gorm:"type:text"`         // AI-generated summary if available
	CreatedDate time.Time `gorm:"index"`
	UpdatedDate *time.Time

	// Risk assessment
	RiskLevel      string `gorm:"type:varchar(50)"` // low, medium, high, critical
	RiskScore      int    // 0-100 risk score
	RiskConfidence int    // 0-100 confidence level

	// Metrics
	IssuesFound      int // Number of issues identified
	SuggestionsCount int // Number of suggestions made
	FilesReviewed    int // Number of files reviewed
	LinesReviewed    int // Lines of code reviewed

	// Effort estimation (from CodeRabbit/Qodo)
	EffortComplexity string `gorm:"type:varchar(50)"` // simple, moderate, complex
	EffortRating     int    // 1-5 scale numeric rating (e.g., from "🎯 3 (Moderate)")
	EffortMinutes    int    // Estimated review time in minutes

	// Pre-merge check results (from CodeRabbit)
	PreMergeChecksPassed       int `gorm:"default:0"` // Number of checks passed
	PreMergeChecksFailed       int `gorm:"default:0"` // Number of checks failed
	PreMergeChecksInconclusive int `gorm:"default:0"` // Number of inconclusive checks

	// Suggestion acceptance tracking
	SuggestionsAccepted      int     `gorm:"default:0"` // Number of suggestions accepted/applied by developer (marker-based)
	SuggestionsDiffAccepted  int     `gorm:"default:0"` // Count of suggestions with diff-based match
	SuggestionsDiffAcceptPct float64 `gorm:"default:0"` // Average acceptance % across matched suggestions

	// Developer reactions to the AI review comment
	ReactionsTotalCount int `gorm:"default:0"`
	ReactionsThumbsUp   int `gorm:"default:0"`
	ReactionsThumbsDown int `gorm:"default:0"`

	// Review outcome
	ReviewState string `gorm:"type:varchar(50)"` // approved, changes_requested, commented

	// Source information
	SourcePlatform string `gorm:"type:varchar(50)"` // github, gitlab
	SourceUrl      string `gorm:"type:varchar(500)"`
}

func (AiReview) TableName() string {
	return "_tool_aireview_reviews"
}

// AI tool type constants
const (
	AiToolCodeRabbit        = "coderabbit"
	AiToolCursorBugbot      = "cursor_bugbot"
	AiToolCursor            = "cursor"
	AiToolClaude            = "claude"
	AiToolQodo              = "qodo"
	AiToolGemini            = "gemini"
	AiToolSonarQube         = "sonarqube"
	AiToolCopilot           = "copilot"
	AiToolAssistedByUnknown = "assisted_by_unknown"
	AiToolMadeWithUnknown   = "made_with_unknown"
	AiToolOther             = "other"
)

// Risk level constants
const (
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

// Review state constants
const (
	ReviewStateApproved         = "approved"
	ReviewStateChangesRequested = "changes_requested"
	ReviewStateCommented        = "commented"
)
