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

type AgentReadyFinding struct {
	common.NoPKModel

	Id                 string   `gorm:"primaryKey;type:varchar(255)"`
	ConnectionId       uint64   `gorm:"primaryKey"`
	AssessmentId       string   `gorm:"index;type:varchar(255)"`
	RepoId             string   `gorm:"index;type:varchar(255)"`
	AttributeId        string   `gorm:"type:varchar(255)"`
	AttributeName      string   `gorm:"type:varchar(255)"`
	Category           string   `gorm:"type:varchar(255)"`
	Tier               int      `gorm:"type:int"`
	Status             string   `gorm:"type:varchar(50)"`
	Score              *float64 `gorm:"type:float"`
	MeasuredValue      string   `gorm:"type:text"`
	Threshold          string   `gorm:"type:text"`
	Evidence           string   `gorm:"type:text"`
	RemediationSummary string   `gorm:"type:text"`
	RemediationSteps   string   `gorm:"type:text"`
	DefaultWeight      float64  `gorm:"type:float"`
}

func (AgentReadyFinding) TableName() string {
	return "_tool_agentready_findings"
}

const (
	FindingStatusPass          = "pass"
	FindingStatusFail          = "fail"
	FindingStatusSkipped       = "skipped"
	FindingStatusError         = "error"
	FindingStatusNotApplicable = "not_applicable"

	TierEssential = 1
	TierCritical  = 2
	TierImportant = 3
	TierAdvanced  = 4
)
