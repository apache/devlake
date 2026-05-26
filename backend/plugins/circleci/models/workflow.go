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
	"github.com/apache/incubator-devlake/core/models/common"
)

type CircleciWorkflow struct {
	ConnectionId   uint64              `gorm:"primaryKey;type:BIGINT"`
	Id             string              `gorm:"primaryKey;type:varchar(100)" json:"id"`
	ProjectSlug    string              `gorm:"type:varchar(255)" json:"project_slug"`
	PipelineId     string              `gorm:"type:varchar(100)" json:"pipeline_id"`
	CanceledBy     string              `gorm:"type:varchar(100)" json:"canceled_by"`
	Name           string              `gorm:"type:varchar(255)" json:"name"`
	ErroredBy      string              `gorm:"type:varchar(100)" json:"errored_by"`
	Tag            string              `gorm:"type:varchar(100)" json:"tag"`
	Status         string              `gorm:"type:varchar(100)" json:"status"`
	StartedBy      string              `gorm:"type:varchar(100)" json:"started_by"`
	PipelineNumber int64               `json:"pipeline_number"`
	CreatedDate    *common.Iso8601Time `json:"created_at"`
	StoppedAt      *common.Iso8601Time `json:"stopped_at"`
	DurationSec    float64             `json:"duration_sec"`

	common.NoPKModel `swaggerignore:"true" json:"-" mapstructure:"-"`
}

func (CircleciWorkflow) TableName() string {
	return "_tool_circleci_workflows"
}
