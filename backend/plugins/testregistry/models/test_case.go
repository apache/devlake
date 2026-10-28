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
	"github.com/apache/devlake/core/models/common"
)

// TestCase represents a single test case within a test suite
type TestCase struct {
	common.NoPKModel

	// Primary keys: connection + job + suite + unique test case identifier
	ConnectionId uint64 `gorm:"primaryKey;type:BIGINT NOT NULL"`
	JobId        string `gorm:"primaryKey;type:varchar(255);index" json:"job_id"`   // Links to TestRegistryCIJob.JobId
	SuiteId      string `gorm:"primaryKey;type:varchar(255);index" json:"suite_id"` // Links to TestSuite.SuiteId
	TestCaseId   string `gorm:"primaryKey;type:varchar(255)" json:"test_case_id"`   // Unique identifier for the test case

	// Test case identification
	Name      string  `gorm:"type:varchar(500);index" json:"name"` // Name of the test case
	Classname string  `gorm:"type:varchar(500)" json:"classname"`  // Class name (if applicable)
	Duration  float64 `json:"duration"`                            // Duration in seconds

	// Test result status: "passed", "failed", "skipped"
	Status string `gorm:"type:varchar(50);index" json:"status"` // Test case status

	// Failure information (if status is "failed")
	FailureMessage *string `gorm:"type:text" json:"failure_message"` // Failure message from the test
	FailureOutput  *string `gorm:"type:text" json:"failure_output"`  // Detailed failure output

	// Skip information (if status is "skipped")
	SkipMessage *string `gorm:"type:text" json:"skip_message"` // Reason why the test was skipped

	// Output streams
	SystemOut *string `gorm:"type:text" json:"system_out"` // stdout output
	SystemErr *string `gorm:"type:text" json:"system_err"` // stderr output
}

func (TestCase) TableName() string {
	return "ci_test_cases"
}
