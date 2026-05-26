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

package qa

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/domainlayer"
)

// QaTestCaseExecution represents a QA test case execution in the domain layer
type QaTestCaseExecution struct {
	domainlayer.DomainEntityExtended
	QaProjectId  string    `gorm:"type:varchar(255);index;comment:Project ID"`
	QaTestCaseId string    `gorm:"type:varchar(255);index;comment:Test case ID"`
	CreateTime   time.Time `gorm:"comment:Test (plan) creation time"`
	StartTime    time.Time `gorm:"comment:Test start time"`
	FinishTime   time.Time `gorm:"comment:Test finish time"`
	CreatorId    string    `gorm:"type:varchar(255);comment:Executor ID"`
	Status       string    `gorm:"type:varchar(255);comment:Test execution status | PENDING | IN_PROGRESS | SUCCESS | FAILED"` // enum, using string
	IsInvalid    bool
}

func (QaTestCaseExecution) TableName() string {
	return "qa_test_case_executions"
}
