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
)

var _ plugin.MigrationScript = (*fixAiReviewDomainColumns)(nil)

type fixAiReviewDomainColumns struct{}

// rawDataOriginFields is the frozen snapshot of common.NoPKModel fields that
// GORM expects when calling CreateOrUpdate on domain entities.
type rawDataOriginFields struct {
	RawDataParams string `gorm:"column:_raw_data_params;type:varchar(255);index"`
	RawDataTable  string `gorm:"column:_raw_data_table;type:varchar(255)"`
	RawDataId     uint64 `gorm:"column:_raw_data_id"`
	RawDataRemark string `gorm:"column:_raw_data_remark"`
}

type fullAiReview20260422 struct {
	Id          string    `gorm:"primaryKey;type:varchar(255)"`
	ProjectName string    `gorm:"index;type:varchar(255)"`
	PullRequestId string  `gorm:"index;type:varchar(255)"`
	RepoId      string    `gorm:"index;type:varchar(255)"`
	AiTool      string    `gorm:"type:varchar(100)"`
	CreatedDate time.Time `gorm:"index"`
	RiskLevel   string    `gorm:"type:varchar(50)"`
	RiskScore   int
	IssuesFound      int
	SuggestionsCount int
	PreMergeChecksPassed int `gorm:"default:0"`
	PreMergeChecksFailed int `gorm:"default:0"`
	ReactionsTotalCount  int `gorm:"default:0"`
	ReactionsThumbsUp    int `gorm:"default:0"`
	ReactionsThumbsDown  int `gorm:"default:0"`
	ReviewState string `gorm:"type:varchar(50)"`
	SourceUrl   string `gorm:"type:varchar(500)"`
	CreatedAt   time.Time
	UpdatedAt   *time.Time
	rawDataOriginFields
}

func (fullAiReview20260422) TableName() string { return "ai_reviews" }

type fullAiFailurePrediction20260422 struct {
	Id              string    `gorm:"primaryKey;type:varchar(255)"`
	ProjectName     string    `gorm:"index;type:varchar(255)"`
	PullRequestId   string    `gorm:"index;type:varchar(255)"`
	PullRequestKey  string    `gorm:"index;type:varchar(255)"`
	RepoId          string    `gorm:"index;type:varchar(255)"`
	RepoName        string    `gorm:"type:varchar(255)"`
	AiTool          string    `gorm:"type:varchar(100)"`
	CiFailureSource string    `gorm:"type:varchar(20);index"`
	PrTitle         string    `gorm:"type:varchar(500)"`
	PrUrl           string    `gorm:"type:varchar(1024)"`
	PrAuthor        string    `gorm:"type:varchar(255)"`
	PrCreatedAt     time.Time
	Additions       int
	Deletions       int
	WasFlaggedRisky   bool
	RiskScore         int
	FlaggedAt         time.Time
	HadCiFailure      bool
	PredictionOutcome string `gorm:"type:varchar(20)"`
	CreatedAt         time.Time
	UpdatedAt         *time.Time
	rawDataOriginFields
}

func (fullAiFailurePrediction20260422) TableName() string { return "ai_failure_predictions" }

type fullAiPredictionMetrics20260422 struct {
	Id              string    `gorm:"primaryKey;type:varchar(255)"`
	ProjectName     string    `gorm:"index;type:varchar(255)"`
	RepoId          string    `gorm:"index;type:varchar(255)"`
	AiTool          string    `gorm:"type:varchar(100)"`
	CiFailureSource string    `gorm:"type:varchar(20);index"`
	PeriodStart     time.Time `gorm:"index"`
	PeriodEnd       time.Time
	PeriodType      string `gorm:"type:varchar(20)"`
	TruePositives   int
	FalsePositives  int
	FalseNegatives  int
	TrueNegatives   int
	Precision       float64
	Recall          float64
	Accuracy        float64
	F1Score         float64
	Specificity     float64
	FprPct          float64
	PrAuc           float64
	RocAuc          float64
	WarningThreshold         int
	TotalPrs                 int
	FlaggedPrs               int
	FailedPrs                int
	RecommendedAutonomyLevel string `gorm:"type:varchar(50)"`
	CalculatedAt             time.Time
	CreatedAt                time.Time
	UpdatedAt                *time.Time
	rawDataOriginFields
}

func (fullAiPredictionMetrics20260422) TableName() string { return "ai_prediction_metrics" }

func (*fixAiReviewDomainColumns) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	// AutoMigrate with embedded structs does not reliably add columns whose names
	// start with '_'. Use explicit ALTER TABLE statements instead.
	tables := []string{"ai_reviews", "ai_failure_predictions", "ai_prediction_metrics"}
	for _, table := range tables {
		sqls := []string{
			"ALTER TABLE `" + table + "` ADD COLUMN `_raw_data_params` varchar(255)",
			"ALTER TABLE `" + table + "` ADD COLUMN `_raw_data_table` varchar(255)",
			"ALTER TABLE `" + table + "` ADD COLUMN `_raw_data_id` bigint unsigned",
			"ALTER TABLE `" + table + "` ADD COLUMN `_raw_data_remark` longtext",
		}
		for _, sql := range sqls {
			if err := db.Exec(sql); err != nil {
				return errors.Default.Wrap(err, "failed: "+sql)
			}
		}
	}
	return nil
}

func (*fixAiReviewDomainColumns) Version() uint64 {
	return 20260422000002
}

func (*fixAiReviewDomainColumns) Name() string {
	return "add missing _raw_data_* columns to ai review domain tables"
}
