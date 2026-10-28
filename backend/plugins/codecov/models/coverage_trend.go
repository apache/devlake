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

type CodecovCoverageTrend struct {
	common.NoPKModel // Includes CreatedAt, UpdatedAt, and RawDataOrigin
	ConnectionId     uint64 `gorm:"primaryKey;type:bigint" json:"connectionId"`
	RepoId             string    `gorm:"primaryKey;type:varchar(200);index" json:"repoId"`
	FlagName           string    `gorm:"primaryKey;type:varchar(100);index" json:"flagName"`
	Branch             string    `gorm:"primaryKey;type:varchar(100)" json:"branch"`
	Date               time.Time `gorm:"primaryKey;type:date" json:"date"`
	CoveragePercentage float64   `json:"coveragePercentage"`
	LinesCovered       int       `json:"linesCovered"`
	LinesTotal         int       `json:"linesTotal"`
	MethodsCovered     int       `json:"methodsCovered"`
	MethodsTotal       int       `json:"methodsTotal"`
}

func (CodecovCoverageTrend) TableName() string {
	return "_tool_codecov_coverage_trends"
}
