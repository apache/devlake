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

var _ plugin.MigrationScript = (*addCIJobsTable)(nil)

// testRegistryCIJob20250107 is a frozen snapshot of models.TestRegistryCIJob as of 2025-01-07.
// Do not update this struct — create a new migration instead.
type testRegistryCIJob20250107 struct {
	// common.NoPKModel fields
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	RawDataParams string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	// primary keys
	ConnectionId uint64 `gorm:"primaryKey;type:BIGINT NOT NULL"`
	JobId        string `gorm:"primaryKey;type:varchar(255)" json:"job_id"`
	// fields
	JobName           string     `gorm:"type:varchar(500);index" json:"job_name"`
	JobType           string     `gorm:"type:varchar(50);index" json:"job_type"`
	Organization      string     `gorm:"type:varchar(255);index" json:"organization"`
	Repository        string     `gorm:"type:varchar(255);index" json:"repository"`
	CommitSHA         string     `gorm:"type:varchar(40);index" json:"commit_sha"`
	PullRequestNumber *int       `gorm:"type:int" json:"pull_request_number"`
	PullRequestAuthor string     `gorm:"type:varchar(255)" json:"pull_request_author"`
	TriggerType       string     `gorm:"type:varchar(50);index" json:"trigger_type"`
	Result            string     `gorm:"type:varchar(100)" json:"result"`
	Namespace         string     `gorm:"type:varchar(255)" json:"namespace"`
	QueuedAt          *time.Time `gorm:"index" json:"queued_at"`
	StartedAt         *time.Time `gorm:"index" json:"started_at"`
	FinishedAt        *time.Time `gorm:"index" json:"finished_at"`
	DurationSec       *float64   `json:"duration_sec"`
	QueuedDurationSec *float64   `json:"queued_duration_sec"`
	ViewURL           string     `gorm:"type:text" json:"view_url"`
	ScopeId           string     `gorm:"type:varchar(500);index" json:"scope_id"`
}

func (testRegistryCIJob20250107) TableName() string {
	return "ci_test_jobs"
}

type addCIJobsTable struct{}

func (*addCIJobsTable) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&testRegistryCIJob20250107{},
	)
}

func (*addCIJobsTable) Version() uint64 {
	return 20250107000001
}

func (*addCIJobsTable) Name() string {
	return "add ci_test_jobs table for testregistry"
}
