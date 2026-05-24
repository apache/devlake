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

type TempoWorklog struct {
	common.NoPKModel `mapstructure:",squash" gorm:"embedded"`
	ConnectionId     uint64 `json:"connectionId" mapstructure:"connectionId" gorm:"primaryKey"`
	TempoWorklogId   int64  `json:"tempoWorklogId" mapstructure:"tempoWorklogId" gorm:"primaryKey"`
	TeamId           int64  `json:"teamId" mapstructure:"teamId" gorm:"index"`
	IssueId          int64  `json:"issueId" mapstructure:"issueId" gorm:"index"`
	IssueKey         string `json:"issueKey" mapstructure:"issueKey" gorm:"type:varchar(255)"`
	AuthorAccountId  string `json:"authorAccountId" mapstructure:"authorAccountId" gorm:"type:varchar(255)"`
	TimeSpentSeconds int    `json:"timeSpentSeconds" mapstructure:"timeSpentSeconds"`
	BillableSeconds  int    `json:"billableSeconds" mapstructure:"billableSeconds"`
	StartDate        string `json:"startDate" mapstructure:"startDate" gorm:"type:varchar(255)"`
	StartTime        string `json:"startTime" mapstructure:"startTime" gorm:"type:varchar(255)"`
	Description      string `json:"description" mapstructure:"description" gorm:"type:text"`
	CreatedAt        string `json:"createdAt" mapstructure:"createdAt" gorm:"type:varchar(255)"`
	UpdatedAt        string `json:"updatedAt" mapstructure:"updatedAt" gorm:"type:varchar(255)"`
}

func (TempoWorklog) TableName() string {
	return "_tool_tempo_worklogs"
}
