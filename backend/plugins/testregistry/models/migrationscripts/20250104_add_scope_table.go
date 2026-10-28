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

// testRegistryScope20250104 is a frozen snapshot of models.TestRegistryScope as of 2025-01-04.
// Do not update this struct — create a new migration instead.
type testRegistryScope20250104 struct {
	// common.NoPKModel fields
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// common.RawDataOrigin fields
	RawDataParams string `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64 `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// common.Scope fields
	ConnectionId  uint64 `json:"connectionId" gorm:"primaryKey"`
	ScopeConfigId uint64 `json:"scopeConfigId,omitempty"`
	// TestRegistryScope-specific fields
	FullName string `gorm:"primaryKey;type:varchar(500)" json:"fullName"`
	Name     string `gorm:"type:varchar(500)" json:"name"`
}

func (testRegistryScope20250104) TableName() string {
	return "_tool_testregistry_scopes"
}

type addScopeTable struct{}

func (*addScopeTable) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&testRegistryScope20250104{},
	)
}

func (*addScopeTable) Version() uint64 {
	return 20250104000001
}

func (*addScopeTable) Name() string {
	return "add scope table for testregistry"
}
