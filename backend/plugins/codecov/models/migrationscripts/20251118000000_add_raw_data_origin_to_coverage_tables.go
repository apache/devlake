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
	"github.com/apache/devlake/helpers/migrationhelper"
)

// codecovCoverage20251118 is a frozen snapshot of models.CodecovCoverage as of 2025-11-18.
// At this time the model used common.Model (with ID) + common.RawDataOrigin.
// Do not update this struct — create a new migration instead.
type codecovCoverage20251118 struct {
	// common.Model fields (as used at time of migration)
	ID        uint64    `gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// common.RawDataOrigin fields
	RawDataParams string `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64 `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// primary keys
	ConnectionId    uint64     `gorm:"primaryKey;type:bigint"`
	RepoId          string     `gorm:"primaryKey;type:varchar(200);index"`
	FlagName        string     `gorm:"primaryKey;type:varchar(100);index"`
	Branch          string     `gorm:"primaryKey;type:varchar(100)"`
	CommitSha       string     `gorm:"primaryKey;type:varchar(64)"`
	CommitTimestamp *time.Time `gorm:"index"`
	// coverage metrics
	CoveragePercentage float64
	ModifiedCoverage   float64
	LinesCovered       int
	LinesTotal         int
	LinesMissed        int
	Hits               int
	Partials           int
	Misses             int
	MethodsCovered     int
	MethodsTotal       int
}

func (codecovCoverage20251118) TableName() string {
	return "_tool_codecov_coverages"
}

// codecovCoverageTrend20251118 is a frozen snapshot of models.CodecovCoverageTrend as of 2025-11-18.
// Do not update this struct — create a new migration instead.
type codecovCoverageTrend20251118 struct {
	// common.NoPKModel fields
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	RawDataParams string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// primary keys
	ConnectionId uint64    `gorm:"primaryKey;type:bigint"`
	RepoId       string    `gorm:"primaryKey;type:varchar(200);index"`
	FlagName     string    `gorm:"primaryKey;type:varchar(100);index"`
	Branch       string    `gorm:"primaryKey;type:varchar(100)"`
	Date         time.Time `gorm:"primaryKey;type:date"`
	// fields
	CoveragePercentage float64
	LinesCovered       int
	LinesTotal         int
	MethodsCovered     int
	MethodsTotal       int
}

func (codecovCoverageTrend20251118) TableName() string {
	return "_tool_codecov_coverage_trends"
}

// codecovCommitCoverage20251118 is a frozen snapshot of models.CodecovCommitCoverage as of 2025-11-18.
// Do not update this struct — create a new migration instead.
type codecovCommitCoverage20251118 struct {
	// common.NoPKModel fields
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	RawDataParams string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// primary keys
	ConnectionId    uint64     `gorm:"primaryKey;type:bigint"`
	RepoId          string     `gorm:"primaryKey;type:varchar(200);index"`
	CommitSha       string     `gorm:"primaryKey;type:varchar(64)"`
	Branch          string     `gorm:"type:varchar(100)"`
	CommitTimestamp *time.Time `gorm:"index"`
	// fields
	OverallCoverage  float64
	ModifiedCoverage float64
	FilesChanged     int
	LinesCovered     int
	LinesTotal       int
	LinesMissed      int
	Hits             int
	Partials         int
	Misses           int
	MethodsCovered   int
	MethodsTotal     int
}

func (codecovCommitCoverage20251118) TableName() string {
	return "_tool_codecov_commit_coverages"
}

type addRawDataOriginToCoverageTables struct{}

func (u *addRawDataOriginToCoverageTables) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&codecovCoverage20251118{},
		&codecovCoverageTrend20251118{},
		&codecovCommitCoverage20251118{},
	)
}

func (*addRawDataOriginToCoverageTables) Version() uint64 {
	return 20251118000000
}

func (*addRawDataOriginToCoverageTables) Name() string {
	return "Codecov add RawDataOrigin to coverage tables"
}
