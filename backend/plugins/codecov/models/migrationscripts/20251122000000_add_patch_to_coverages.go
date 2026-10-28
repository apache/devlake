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

// codecovCoverage20251122 is a frozen snapshot of models.CodecovCoverage as of 2025-11-22.
// Do not update this struct — create a new migration instead.
type codecovCoverage20251122 struct {
	// common.Model fields (as used at time of migration)
	ID        uint64    `gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// common.RawDataOrigin fields
	RawDataParams string `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64 `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// primary key fields
	ConnectionId    uint64     `gorm:"primaryKey;type:bigint" json:"connectionId"`
	RepoId          string     `gorm:"primaryKey;type:varchar(200);index" json:"repoId"`
	FlagName        string     `gorm:"primaryKey;type:varchar(100);index" json:"flagName"`
	Branch          string     `gorm:"primaryKey;type:varchar(100)" json:"branch"`
	CommitSha       string     `gorm:"primaryKey;type:varchar(64)" json:"commitSha"`
	CommitTimestamp *time.Time `gorm:"index" json:"commitTimestamp"`
	// coverage metrics
	CoveragePercentage float64 `json:"coveragePercentage"`
	ModifiedCoverage   float64 `json:"modifiedCoverage"`
	LinesCovered       int     `json:"linesCovered"`
	LinesTotal         int     `json:"linesTotal"`
	LinesMissed        int     `json:"linesMissed"`
	Hits               int     `json:"hits"`
	Partials           int     `json:"partials"`
	Misses             int     `json:"misses"`
	MethodsCovered     int     `json:"methodsCovered"`
	MethodsTotal       int     `json:"methodsTotal"`
}

func (codecovCoverage20251122) TableName() string {
	return "_tool_codecov_coverages"
}

type addPatchToCoverages struct{}

func (u *addPatchToCoverages) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&codecovCoverage20251122{},
	)
}

func (*addPatchToCoverages) Version() uint64 {
	return 20251122000000
}

func (*addPatchToCoverages) Name() string {
	return "Codecov add Patch to coverages table"
}
