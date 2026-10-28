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

var _ plugin.MigrationScript = (*addCiPredictionSupport)(nil)

type addCiPredictionSupport struct{}

// Up adds all CI failure prediction support columns in one step.
//
// Consolidates what were previously five incremental migrations
// (20260330, 20260331, 20260401, 20260402x1, 20260402x2) into a single
// idempotent migration. Safe to run on databases that already have some
// or all of these columns — AutoMigrate only adds missing ones.
//
// _tool_aireview_scope_configs gains:
//   - warning_threshold     int  (default 50) — risk score cutoff for TP/FP/FN/TN
//   - ci_failure_source     varchar(20)       — "test_cases", "job_result", or "both"
//   - ci_backfill_enabled   bool              — fetch historical CI data from GCS
//   - ci_backfill_days      int  (default 180)
//
// _tool_aireview_failure_predictions gains:
//   - pull_request_key  varchar(255) index   — PR number for CI join
//   - repo_short_name   varchar(255)         — after '/' for CI repo matching
//   - ci_failure_source varchar(20)  index   — which source produced this record
//   - repo_name         varchar(255)         — full org/repo for display
//   - pr_title          varchar(500)         — denormalised for drill-down
//   - pr_url            varchar(1024)        — link to the PR
//   - pr_author         varchar(255)
//   - pr_created_at     datetime
//   - additions         bigint
//   - deletions         bigint
//
// _tool_aireview_prediction_metrics gains:
//   - specificity      float64
//   - fpr_pct          float64
//   - pr_auc           float64
//   - roc_auc          float64
//   - warning_threshold int
//   - ci_failure_source varchar(20) index
func (script *addCiPredictionSupport) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	if err := db.AutoMigrate(&scopeConfigCiPrediction20260402{}); err != nil {
		return errors.Default.Wrap(err, "failed to migrate _tool_aireview_scope_configs for CI prediction support")
	}
	if err := db.AutoMigrate(&failurePredictionCiSupport20260402{}); err != nil {
		return errors.Default.Wrap(err, "failed to migrate _tool_aireview_failure_predictions for CI prediction support")
	}
	if err := db.AutoMigrate(&predictionMetricsCiSupport20260402{}); err != nil {
		return errors.Default.Wrap(err, "failed to migrate _tool_aireview_prediction_metrics for CI prediction support")
	}

	return nil
}

func (script *addCiPredictionSupport) Version() uint64 {
	return 20260402000003
}

func (script *addCiPredictionSupport) Name() string {
	return "aireview add CI failure prediction support (scope config, predictions, metrics)"
}

type scopeConfigCiPrediction20260402 struct {
	WarningThreshold  int    `gorm:"default:50"`
	CiFailureSource   string `gorm:"type:varchar(20);default:'both'"`
	CiBackfillEnabled bool   `gorm:"type:boolean;default:false"`
	CiBackfillDays    int    `gorm:"default:180"`
}

func (scopeConfigCiPrediction20260402) TableName() string {
	return "_tool_aireview_scope_configs"
}

type failurePredictionCiSupport20260402 struct {
	PullRequestKey  string    `gorm:"type:varchar(255);index"`
	RepoShortName   string    `gorm:"type:varchar(255)"`
	CiFailureSource string    `gorm:"type:varchar(20);index"`
	RepoName        string    `gorm:"type:varchar(255)"`
	PrTitle         string    `gorm:"type:varchar(500)"`
	PrUrl           string    `gorm:"type:varchar(1024)"`
	PrAuthor        string    `gorm:"type:varchar(255)"`
	PrCreatedAt     time.Time `gorm:"column:pr_created_at"`
	Additions       int
	Deletions       int
}

func (failurePredictionCiSupport20260402) TableName() string {
	return "_tool_aireview_failure_predictions"
}

type predictionMetricsCiSupport20260402 struct {
	Specificity      float64
	FprPct           float64
	PrAuc            float64
	RocAuc           float64
	WarningThreshold int    `gorm:"default:50"`
	CiFailureSource  string `gorm:"type:varchar(20);index"`
}

func (predictionMetricsCiSupport20260402) TableName() string {
	return "_tool_aireview_prediction_metrics"
}
