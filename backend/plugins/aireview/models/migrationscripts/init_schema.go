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

package migrationscripts

import (
	"time"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/migrationhelper"
)

var _ plugin.MigrationScript = (*initSchema)(nil)

type initSchema struct{}

type aiReview20260127 struct {
	// common.NoPKModel fields
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	RawDataParams    string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable     string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId        uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark    string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	Id               string    `gorm:"primaryKey;type:varchar(255)"`
	PullRequestId    string    `gorm:"index;type:varchar(255)"`
	RepoId           string    `gorm:"index;type:varchar(255)"`
	AiTool           string    `gorm:"type:varchar(100)"`
	AiToolUser       string    `gorm:"type:varchar(255)"`
	ReviewId         string    `gorm:"type:varchar(255)"`
	Body             string    `gorm:"type:longtext"`
	Summary          string    `gorm:"type:text"`
	CreatedDate      time.Time
	UpdatedDate      *time.Time
	RiskLevel        string `gorm:"type:varchar(50)"`
	RiskScore        int
	RiskConfidence   int
	IssuesFound      int
	SuggestionsCount int
	FilesReviewed    int
	LinesReviewed    int
	EffortComplexity string `gorm:"type:varchar(50)"`
	EffortMinutes    int
	ReviewState      string `gorm:"type:varchar(50)"`
	SourcePlatform   string `gorm:"type:varchar(50)"`
	SourceUrl        string `gorm:"type:varchar(500)"`
}

func (aiReview20260127) TableName() string {
	return "_tool_aireview_reviews"
}

type aiReviewFinding20260127 struct {
	// common.NoPKModel fields
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	RawDataParams     string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable      string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId         uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark     string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	Id                string    `gorm:"primaryKey;type:varchar(255)"`
	AiReviewId        string    `gorm:"index;type:varchar(255)"`
	PullRequestId     string    `gorm:"index;type:varchar(255)"`
	RepoId            string    `gorm:"index;type:varchar(255)"`
	AiTool            string    `gorm:"type:varchar(100)"`
	Category          string    `gorm:"type:varchar(100)"`
	Severity          string    `gorm:"type:varchar(50)"`
	Type              string    `gorm:"type:varchar(100)"`
	Title             string    `gorm:"type:varchar(500)"`
	Description       string    `gorm:"type:text"`
	FilePath          string    `gorm:"type:varchar(500)"`
	LineStart         int
	LineEnd           int
	CommitSha         string `gorm:"type:varchar(255)"`
	CodeSnippet       string `gorm:"type:text"`
	SuggestedCode     string `gorm:"type:text"`
	SuggestionApplied bool
	IsResolved        bool
	ResolvedAt        *time.Time
	ResolvedBy        string `gorm:"type:varchar(255)"`
	Resolution        string `gorm:"type:varchar(100)"`
	ResponseTime      int
	CreatedDate       time.Time
	SourceCommentId   string `gorm:"type:varchar(255)"`
}

func (aiReviewFinding20260127) TableName() string {
	return "_tool_aireview_findings"
}

type aiFailurePrediction20260127 struct {
	// common.RawDataOrigin fields (from NoPKModel).
	// CreatedAt/UpdatedAt from NoPKModel are omitted here because they are declared
	// below with UpdatedAt as *time.Time, matching the original shadowing behavior.
	RawDataParams         string `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable          string `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId             uint64 `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark         string `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	Id                    string `gorm:"primaryKey;type:varchar(255)"`
	PullRequestId         string `gorm:"index;type:varchar(255)"`
	RepoId                string `gorm:"index;type:varchar(255)"`
	AiTool                string `gorm:"type:varchar(100)"`
	WasFlaggedRisky       bool
	RiskScore             int
	FlaggedAt             time.Time
	PrMergedAt            *time.Time
	HadCiFailure          bool
	CiFailureAt           *time.Time
	HadBugReported        bool
	BugReportedAt         *time.Time
	BugIssueId            string `gorm:"type:varchar(255)"`
	HadRollback           bool
	RollbackAt            *time.Time
	PredictionOutcome     string `gorm:"type:varchar(20)"`
	ObservationWindowDays int
	ObservationEndDate    time.Time
	CreatedAt             time.Time
	UpdatedAt             *time.Time
}

func (aiFailurePrediction20260127) TableName() string {
	return "_tool_aireview_failure_predictions"
}

type aiPredictionMetrics20260127 struct {
	// common.NoPKModel fields
	CreatedAt                time.Time `json:"createdAt"`
	UpdatedAt                time.Time `json:"updatedAt"`
	RawDataParams            string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable             string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId                uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark            string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	Id                       string    `gorm:"primaryKey;type:varchar(255)"`
	RepoId                   string    `gorm:"index;type:varchar(255)"`
	AiTool                   string    `gorm:"type:varchar(100)"`
	PeriodStart              time.Time
	PeriodEnd                time.Time
	PeriodType               string `gorm:"type:varchar(20)"`
	TruePositives            int
	FalsePositives           int
	FalseNegatives           int
	TrueNegatives            int
	Precision                float64
	Recall                   float64
	Accuracy                 float64
	F1Score                  float64
	TotalPrs                 int
	FlaggedPrs               int
	FailedPrs                int
	ObservedPrs              int
	RecommendedAutonomyLevel string `gorm:"type:varchar(50)"`
	CalculatedAt             time.Time
}

func (aiPredictionMetrics20260127) TableName() string {
	return "_tool_aireview_prediction_metrics"
}

type aiReviewScopeConfig20260127 struct {
	// common.Model fields
	ID        uint64    `gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// common.ScopeConfig fields
	Entities              []string `gorm:"type:json;serializer:json" json:"entities"`
	ConnectionId          uint64   `json:"connectionId" gorm:"index"`
	Name                  string   `gorm:"type:varchar(255);uniqueIndex" json:"name"`
	CodeRabbitEnabled     bool     `gorm:"type:boolean"`
	CodeRabbitUsername    string   `gorm:"type:varchar(255)"`
	CodeRabbitPattern     string   `gorm:"type:varchar(500)"`
	CursorBugbotEnabled   bool     `gorm:"type:boolean"`
	CursorBugbotUsername  string   `gorm:"type:varchar(255)"`
	CursorBugbotPattern   string   `gorm:"type:varchar(500)"`
	QodoEnabled           bool     `gorm:"type:boolean"`
	QodoUsername          string   `gorm:"type:varchar(255)"`
	QodoPattern           string   `gorm:"type:varchar(500)"`
	AiCommitPatterns      string   `gorm:"type:text"`
	AiPrLabelPattern      string   `gorm:"type:varchar(500)"`
	RiskHighPattern       string   `gorm:"type:varchar(500)"`
	RiskMediumPattern     string   `gorm:"type:varchar(500)"`
	RiskLowPattern        string   `gorm:"type:varchar(500)"`
	ObservationWindowDays int
	BugLinkPattern        string `gorm:"type:varchar(500)"`
}

func (aiReviewScopeConfig20260127) TableName() string {
	return "_tool_aireview_scope_configs"
}

func (script *initSchema) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&aiReview20260127{},
		&aiReviewFinding20260127{},
		&aiFailurePrediction20260127{},
		&aiPredictionMetrics20260127{},
		&aiReviewScopeConfig20260127{},
	)
}

func (script *initSchema) Version() uint64 {
	return 20260127000001
}

func (script *initSchema) Name() string {
	return "aireview init schema"
}
