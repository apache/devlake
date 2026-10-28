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

var _ plugin.MigrationScript = (*addTestCasesTable)(nil)

// testCase20250111 is a frozen snapshot of models.TestCase as of 2025-01-11.
// Do not update this struct — create a new migration instead.
type testCase20250111 struct {
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
	SuiteId      string `gorm:"primaryKey;type:varchar(255);index" json:"suite_id"`
	TestCaseId   string `gorm:"primaryKey;type:varchar(255)" json:"test_case_id"`
	// fields
	Name           string  `gorm:"type:varchar(500);index" json:"name"`
	Classname      string  `gorm:"type:varchar(500)" json:"classname"`
	Duration       float64 `json:"duration"`
	Status         string  `gorm:"type:varchar(50);index" json:"status"`
	FailureMessage *string `gorm:"type:text" json:"failure_message"`
	FailureOutput  *string `gorm:"type:text" json:"failure_output"`
	SkipMessage    *string `gorm:"type:text" json:"skip_message"`
	SystemOut      *string `gorm:"type:text" json:"system_out"`
	SystemErr      *string `gorm:"type:text" json:"system_err"`
}

func (testCase20250111) TableName() string {
	return "ci_test_cases"
}

type addTestCasesTable struct{}

func (*addTestCasesTable) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&testCase20250111{},
	)
}

func (*addTestCasesTable) Version() uint64 {
	return 20250111000001
}

func (*addTestCasesTable) Name() string {
	return "add ci_test_cases table for testregistry"
}
