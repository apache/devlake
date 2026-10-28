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

// TektonTask represents a Tekton task run within a PipelineRun
// This table stores individual task execution details from Tekton PipelineRuns
type TektonTask struct {
	common.NoPKModel

	// Primary keys: connection + job + task name
	ConnectionId uint64 `gorm:"primaryKey;type:BIGINT NOT NULL"`
	JobId        string `gorm:"primaryKey;type:varchar(255);index" json:"job_id"`    // Links to TestRegistryCIJob.JobId (PipelineRunName)
	TaskName     string `gorm:"primaryKey;type:varchar(500);index" json:"task_name"` // Task name (e.g., "deploy-konflux")

	// Task status: "Succeeded", "Failed", etc.
	Status string `gorm:"type:varchar(100);index" json:"status"` // Task status

	// Duration in seconds (parsed from duration string like "499s")
	DurationSec float64 `json:"duration_sec"` // Duration in seconds as a number
}

func (TektonTask) TableName() string {
	return "ci_tekton_tasks"
}
