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

// codecovFlag20251115 is a frozen snapshot of models.CodecovFlag as of 2025-11-15.
// Do not update this struct — create a new migration instead.
type codecovFlag20251115 struct {
	// common.NoPKModel fields
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	RawDataParams string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// primary keys
	ConnectionId uint64 `gorm:"primaryKey;type:bigint"`
	RepoId       string `gorm:"primaryKey;type:varchar(200)"`
	FlagName     string `gorm:"primaryKey;type:varchar(100)"`
	// fields
	Carryforward bool
	Deleted      bool
	Yaml         string `gorm:"type:text"`
	Coverage     *float64
}

func (codecovFlag20251115) TableName() string {
	return "_tool_codecov_flags"
}

type addRawDataOriginToFlags struct{}

func (u *addRawDataOriginToFlags) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&codecovFlag20251115{},
	)
}

func (*addRawDataOriginToFlags) Version() uint64 {
	return 20251115000000
}

func (*addRawDataOriginToFlags) Name() string {
	return "Codecov add RawDataOrigin to flags table"
}
