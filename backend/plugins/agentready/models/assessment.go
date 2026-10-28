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

type AgentReadyAssessment struct {
	common.NoPKModel

	Id                 string    `gorm:"primaryKey;type:varchar(255)"`
	RepoId             string    `gorm:"index;type:varchar(255)"`
	RepoName           string    `gorm:"type:varchar(255)"`
	ConnectionId       uint64    `gorm:"primaryKey"`
	Provider           string    `gorm:"type:varchar(50)"`
	SchemaVersion      string    `gorm:"type:varchar(20)"`
	OverallScore       float64   `gorm:"type:float"`
	CertificationLevel string    `gorm:"type:varchar(50)"`
	AttributesAssessed int       `gorm:"type:int"`
	AttributesTotal    int       `gorm:"type:int"`
	Branch             string    `gorm:"type:varchar(255)"`
	CommitHash         string    `gorm:"type:varchar(40)"`
	DurationSeconds    float64   `gorm:"type:float"`
	AssessedAt         time.Time `gorm:"index"`
	CollectedAt        time.Time
	RawJSON            string `gorm:"type:longtext"`
}

func (AgentReadyAssessment) TableName() string {
	return "_tool_agentready_assessments"
}

const (
	CertPlatinum         = "Platinum"
	CertGold             = "Gold"
	CertSilver           = "Silver"
	CertBronze           = "Bronze"
	CertNeedsImprovement = "Needs Improvement"
	CertNone             = "None"

	ProjectMappingTable = "repos"
)
