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

type AgentReadyMetric struct {
	common.NoPKModel

	Id             string    `gorm:"primaryKey;type:varchar(255)"`
	ConnectionId   uint64    `gorm:"primaryKey"`
	RepoId         string    `gorm:"index;type:varchar(255)"`
	AssessedAt     time.Time `gorm:"index"`
	PassCount      int       `gorm:"type:int"`
	FailCount      int       `gorm:"type:int"`
	SkipCount      int       `gorm:"type:int"`
	Tier1PassRate  float64   `gorm:"type:float"`
	Tier2PassRate  float64   `gorm:"type:float"`
	Tier3PassRate  float64   `gorm:"type:float"`
	Tier4PassRate  float64   `gorm:"type:float"`
	CategoryScores string    `gorm:"type:text"`
}

func (AgentReadyMetric) TableName() string {
	return "_tool_agentready_metrics"
}
