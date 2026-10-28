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
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/migrationhelper"
)

var _ plugin.MigrationScript = (*addTektonTasksTable)(nil)

// tektonTask20250112 is a frozen snapshot of models.TektonTask as of 2025-01-12.
// Do not update this struct — create a new migration instead.
type tektonTask20250112 struct {
	// common.NoPKModel fields
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	RawDataParams string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// primary keys
	ConnectionId uint64 `gorm:"primaryKey;type:BIGINT NOT NULL"`
	JobId        string `gorm:"primaryKey;type:varchar(255);index" json:"job_id"`
	TaskName     string `gorm:"primaryKey;type:varchar(500);index" json:"task_name"`
	// fields
	Status      string  `gorm:"type:varchar(100);index" json:"status"`
	DurationSec float64 `json:"duration_sec"`
}

func (tektonTask20250112) TableName() string {
	return "ci_tekton_tasks"
}

type addTektonTasksTable struct{}

func (*addTektonTasksTable) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&tektonTask20250112{},
	)
}

func (*addTektonTasksTable) Version() uint64 {
	return 20250112000001
}

func (*addTektonTasksTable) Name() string {
	return "add ci_tekton_tasks table for testregistry"
}
