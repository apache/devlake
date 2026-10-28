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

package code

import (
	"time"

	"github.com/apache/devlake/core/models/domainlayer"
)

// AiPredictionMetrics is the project-scoped domain representation of aggregated
// AI failure-prediction accuracy metrics (precision, recall, AUC, etc.) per period.
// Converted from _tool_aireview_prediction_metrics.
type AiPredictionMetrics struct {
	domainlayer.DomainEntity

	ProjectName     string `gorm:"index;type:varchar(255)"`
	RepoId          string `gorm:"index;type:varchar(255)"`
	AiTool          string `gorm:"type:varchar(100)"`
	CiFailureSource string `gorm:"type:varchar(20);index"`

	PeriodStart time.Time `gorm:"index"`
	PeriodEnd   time.Time
	PeriodType  string `gorm:"type:varchar(20)"`

	TruePositives  int
	FalsePositives int
	FalseNegatives int
	TrueNegatives  int

	Precision   float64
	Recall      float64
	Accuracy    float64
	F1Score     float64
	Specificity float64
	FprPct      float64
	PrAuc       float64
	RocAuc      float64

	WarningThreshold         int
	TotalPrs                 int
	FlaggedPrs               int
	FailedPrs                int
	RecommendedAutonomyLevel string `gorm:"type:varchar(50)"`
	CalculatedAt             time.Time
}

func (AiPredictionMetrics) TableName() string {
	return "ai_prediction_metrics"
}
