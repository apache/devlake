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

// TestRegistryCIJob represents a unified CI/CD job from either Prow (Openshift CI) or Tekton
// This table stores common information that both job types share, making it easy for agents
// to query and analyze CI/CD data regardless of the underlying platform.
type TestRegistryCIJob struct {
	common.NoPKModel

	// Primary keys: connection + unique job identifier
	ConnectionId uint64 `gorm:"primaryKey;type:BIGINT NOT NULL"`
	JobId        string `gorm:"primaryKey;type:varchar(255)" json:"job_id"` // Unique job ID from source system

	// Job identification
	JobName string `gorm:"type:varchar(500);index" json:"job_name"` // Name of the job/pipeline
	JobType string `gorm:"type:varchar(50);index" json:"job_type"`  // "prow" or "tekton"

	// Repository and Organization (key identifiers as requested)
	Organization string `gorm:"type:varchar(255);index" json:"organization"` // GitHub org or Quay org
	Repository   string `gorm:"type:varchar(255);index" json:"repository"`   // Repository name

	// Git references
	CommitSHA string `gorm:"type:varchar(40);index" json:"commit_sha"` // Git commit SHA

	// Pull Request information (for Prow presubmit, Tekton PR-triggered runs)
	PullRequestNumber *int   `gorm:"type:int" json:"pull_request_number"` // PR number if triggered by PR
	PullRequestAuthor string `gorm:"type:varchar(255)" json:"pull_request_author"`

	// Trigger type: "pull_request" (PR-triggered/Presubmit), "push" (push to branch/Postsubmit), or "periodic" (scheduled)
	TriggerType string `gorm:"type:varchar(50);index" json:"trigger_type"`

	// Status and result
	Result string `gorm:"type:varchar(100)" json:"result"` // "SUCCESS", "FAILURE", "ABORTED", etc.

	// Execution environment (optional - only if applicable)
	Namespace string `gorm:"type:varchar(255)" json:"namespace"` // Kubernetes namespace (if applicable)

	// Timestamps
	QueuedAt          *time.Time `gorm:"index" json:"queued_at"`   // When job was queued
	StartedAt         *time.Time `gorm:"index" json:"started_at"`  // When job started executing
	FinishedAt        *time.Time `gorm:"index" json:"finished_at"` // When job completed
	DurationSec       *float64   `json:"duration_sec"`             // Execution duration in seconds
	QueuedDurationSec *float64   `json:"queued_duration_sec"`      // Time spent in queue

	// URLs
	ViewURL string `gorm:"type:text" json:"view_url"` // URL to view job in UI

	// Foreign key to scope (which repository/scope this job belongs to)
	ScopeId string `gorm:"type:varchar(500);index" json:"scope_id"` // Links to TestRegistryScope.FullName
}

func (TestRegistryCIJob) TableName() string {
	return "ci_test_jobs"
}
