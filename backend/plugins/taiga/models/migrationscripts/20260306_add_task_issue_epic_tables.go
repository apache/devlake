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

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

type TaigaTask20260306 struct {
	ConnectionId   uint64 `gorm:"primaryKey"`
	ProjectId      uint64 `gorm:"index"`
	TaskId         uint64 `gorm:"primaryKey;autoIncrement:false"`
	Ref            int
	Subject        string `gorm:"type:varchar(255)"`
	Status         string `gorm:"type:varchar(100)"`
	IsClosed       bool
	CreatedDate    *time.Time
	ModifiedDate   *time.Time
	FinishedDate   *time.Time
	AssignedTo     uint64
	AssignedToName string `gorm:"type:varchar(255)"`
	UserStoryId    uint64
	MilestoneId    uint64
	IsBlocked      bool
	BlockedNote    string `gorm:"type:text"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RawDataParams  string `gorm:"column:_raw_data_params;type:varchar(255);index"`
	RawDataTable   string `gorm:"column:_raw_data_table;type:varchar(255)"`
	RawDataId      uint64 `gorm:"column:_raw_data_id"`
	RawDataRemark  string `gorm:"column:_raw_data_remark"`
}

func (TaigaTask20260306) TableName() string {
	return "_tool_taiga_tasks"
}

type TaigaIssue20260306 struct {
	ConnectionId   uint64 `gorm:"primaryKey"`
	ProjectId      uint64 `gorm:"index"`
	IssueId        uint64 `gorm:"primaryKey;autoIncrement:false"`
	Ref            int
	Subject        string `gorm:"type:varchar(255)"`
	Status         string `gorm:"type:varchar(100)"`
	IssueTypeName  string `gorm:"type:varchar(100)"`
	Priority       string `gorm:"type:varchar(100)"`
	Severity       string `gorm:"type:varchar(100)"`
	IsClosed       bool
	CreatedDate    *time.Time
	ModifiedDate   *time.Time
	FinishedDate   *time.Time
	AssignedTo     uint64
	AssignedToName string `gorm:"type:varchar(255)"`
	MilestoneId    uint64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RawDataParams  string `gorm:"column:_raw_data_params;type:varchar(255);index"`
	RawDataTable   string `gorm:"column:_raw_data_table;type:varchar(255)"`
	RawDataId      uint64 `gorm:"column:_raw_data_id"`
	RawDataRemark  string `gorm:"column:_raw_data_remark"`
}

func (TaigaIssue20260306) TableName() string {
	return "_tool_taiga_issues"
}

type TaigaEpic20260306 struct {
	ConnectionId   uint64 `gorm:"primaryKey"`
	ProjectId      uint64 `gorm:"index"`
	EpicId         uint64 `gorm:"primaryKey;autoIncrement:false"`
	Ref            int
	Subject        string `gorm:"type:varchar(255)"`
	Status         string `gorm:"type:varchar(100)"`
	IsClosed       bool
	CreatedDate    *time.Time
	ModifiedDate   *time.Time
	AssignedTo     uint64
	AssignedToName string `gorm:"type:varchar(255)"`
	Color          string `gorm:"type:varchar(20)"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RawDataParams  string `gorm:"column:_raw_data_params;type:varchar(255);index"`
	RawDataTable   string `gorm:"column:_raw_data_table;type:varchar(255)"`
	RawDataId      uint64 `gorm:"column:_raw_data_id"`
	RawDataRemark  string `gorm:"column:_raw_data_remark"`
}

func (TaigaEpic20260306) TableName() string {
	return "_tool_taiga_epics"
}

type addTaskIssueEpicTables20260306 struct{}

func (*addTaskIssueEpicTables20260306) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&TaigaTask20260306{},
		&TaigaIssue20260306{},
		&TaigaEpic20260306{},
	)
}

func (*addTaskIssueEpicTables20260306) Version() uint64 {
	return 20260306000001
}

func (*addTaskIssueEpicTables20260306) Name() string {
	return "Taiga add task, issue, epic tables"
}
