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
	"time"

	"github.com/apache/devlake/core/models/common"
)

type AzuredevopsTestPlan struct {
	common.NoPKModel

	ConnectionId uint64 `gorm:"primaryKey"`
	AzuredevopsId int   `gorm:"primaryKey"`
	OrgId        string `gorm:"type:varchar(255)"`
	ProjectId    string `gorm:"type:varchar(255)"`
	Name         string `gorm:"type:varchar(255)"`
	State        string `gorm:"type:varchar(255)"`
	RootSuiteId  int
	StartedDate  *time.Time
	EndedDate    *time.Time
	Url          string `gorm:"type:varchar(500)"`
}

func (AzuredevopsTestPlan) TableName() string {
	return "_tool_azuredevops_go_test_plans"
}

type AzuredevopsTestCase struct {
	common.NoPKModel

	ConnectionId  uint64 `gorm:"primaryKey"`
	AzuredevopsId int    `gorm:"primaryKey"`
	PlanId        int    `gorm:"primaryKey"`
	OrgId         string `gorm:"type:varchar(255)"`
	ProjectId     string `gorm:"type:varchar(255)"`
	Name          string `gorm:"type:varchar(255)"`
	WorkItemId    int
	Url           string `gorm:"type:varchar(500)"`
	CreatedDate   *time.Time
	CreatorId     string `gorm:"type:varchar(255)"`
}

func (AzuredevopsTestCase) TableName() string {
	return "_tool_azuredevops_go_test_cases"
}

type AzuredevopsTestRun struct {
	common.NoPKModel

	ConnectionId  uint64 `gorm:"primaryKey"`
	AzuredevopsId int    `gorm:"primaryKey"`
	OrgId         string `gorm:"type:varchar(255)"`
	ProjectId     string `gorm:"type:varchar(255)"`
	PlanId        int
	Name          string `gorm:"type:varchar(255)"`
	State         string `gorm:"type:varchar(255)"`
	StartedDate   *time.Time
	CompletedDate *time.Time
	TotalTests    int
	PassedTests   int
	Url           string `gorm:"type:varchar(500)"`
}

func (AzuredevopsTestRun) TableName() string {
	return "_tool_azuredevops_go_test_runs"
}

type AzuredevopsTestResult struct {
	common.NoPKModel

	ConnectionId  uint64 `gorm:"primaryKey"`
	AzuredevopsId int    `gorm:"primaryKey"`
	RunId         int    `gorm:"primaryKey"`
	OrgId         string `gorm:"type:varchar(255)"`
	ProjectId     string `gorm:"type:varchar(255)"`
	PlanId        int
	TestCaseId    int
	TestCaseTitle string `gorm:"type:varchar(255)"`
	Outcome       string `gorm:"type:varchar(255)"`
	State         string `gorm:"type:varchar(255)"`
	StartedDate   *time.Time
	CompletedDate *time.Time
	DurationMs    int
	Url           string `gorm:"type:varchar(500)"`
	CreatorId     string `gorm:"type:varchar(255)"`
}

func (AzuredevopsTestResult) TableName() string {
	return "_tool_azuredevops_go_test_results"
}
