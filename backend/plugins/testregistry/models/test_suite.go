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

// TestSuite represents a JUnit test suite from a CI job
// Each CI job can have multiple test suites from JUnit XML files
type TestSuite struct {
	common.NoPKModel

	// Primary keys: connection + job + unique suite identifier
	ConnectionId uint64 `gorm:"primaryKey;type:BIGINT NOT NULL"`
	JobId        string `gorm:"primaryKey;type:varchar(255);index" json:"job_id"` // Links to TestRegistryCIJob.JobId
	SuiteId      string `gorm:"primaryKey;type:varchar(255)" json:"suite_id"`     // Unique identifier for the suite (UID)

	// Suite identification
	Name string `gorm:"type:varchar(500);index" json:"name"` // Name of the test suite

	// Test statistics
	NumTests   uint    `json:"num_tests"`   // Total number of tests in the suite
	NumSkipped uint    `json:"num_skipped"` // Number of skipped tests
	NumFailed  uint    `json:"num_failed"`  // Number of failed tests
	Duration   float64 `json:"duration"`    // Duration in seconds

	// Properties stored as JSON (optional test suite properties)
	Properties string `gorm:"type:text" json:"properties"` // JSON string of suite properties

	// Parent suite reference (for nested suites)
	ParentSuiteId *string `gorm:"type:varchar(255);index" json:"parent_suite_id"` // NULL for top-level suites
}

func (TestSuite) TableName() string {
	return "ci_test_suites"
}
