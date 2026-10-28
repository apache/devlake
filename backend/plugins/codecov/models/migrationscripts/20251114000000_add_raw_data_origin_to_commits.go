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

// codecovCommit20251114 is a frozen snapshot of models.CodecovCommit as of 2025-11-14.
// Do not update this struct — create a new migration instead.
type codecovCommit20251114 struct {
	// common.NoPKModel fields
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	RawDataParams string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// primary keys
	ConnectionId uint64 `gorm:"primaryKey;type:bigint"`
	RepoId       string `gorm:"primaryKey;type:varchar(200);index"`
	CommitSha    string `gorm:"primaryKey;type:varchar(64);index"`
	// fields
	Branch          string     `gorm:"type:varchar(100)"`
	CommitTimestamp *time.Time `gorm:"index"`
	Message         string     `gorm:"type:text"`
	Author          string     `gorm:"type:varchar(255)"`
	ParentSha       string     `gorm:"type:varchar(64)"`
}

func (codecovCommit20251114) TableName() string {
	return "_tool_codecov_commits"
}

type addRawDataOriginToCommits struct{}

func (u *addRawDataOriginToCommits) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&codecovCommit20251114{},
	)
}

func (*addRawDataOriginToCommits) Version() uint64 {
	return 20251114000000
}

func (*addRawDataOriginToCommits) Name() string {
	return "Codecov add RawDataOrigin columns to commits table"
}
