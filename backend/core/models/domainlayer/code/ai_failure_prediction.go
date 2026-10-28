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

// AiFailurePrediction is the project-scoped domain representation of a
// per-PR AI failure prediction outcome (TP/FP/FN/TN/NO_CI).
// Converted from _tool_aireview_failure_predictions.
type AiFailurePrediction struct {
	domainlayer.DomainEntity

	ProjectName     string `gorm:"index;type:varchar(255)"`
	PullRequestId   string `gorm:"index;type:varchar(255)"`
	PullRequestKey  string `gorm:"index;type:varchar(255)"`
	RepoId          string `gorm:"index;type:varchar(255)"`
	RepoName        string `gorm:"type:varchar(255)"`
	AiTool          string `gorm:"type:varchar(100)"`
	CiFailureSource string `gorm:"type:varchar(20);index"`

	// PR display metadata for drill-down dashboards
	PrTitle     string `gorm:"type:varchar(500)"`
	PrUrl       string `gorm:"type:varchar(1024)"`
	PrAuthor    string `gorm:"type:varchar(255)"`
	PrCreatedAt time.Time
	Additions   int
	Deletions   int

	WasFlaggedRisky   bool
	RiskScore         int
	FlaggedAt         time.Time
	HadCiFailure      bool
	PredictionOutcome string `gorm:"type:varchar(20)"`
}

func (AiFailurePrediction) TableName() string {
	return "ai_failure_predictions"
}
