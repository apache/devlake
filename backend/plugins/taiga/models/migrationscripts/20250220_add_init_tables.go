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

type TaigaConnection20250220 struct {
	ID               uint64    `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	Name             string    `gorm:"type:varchar(100);uniqueIndex" json:"name"`
	Endpoint         string    `json:"endpoint"`
	Username         string    `json:"username"`
	Password         string    `json:"password"`
	Token            string    `json:"token"`
	Proxy            string    `json:"proxy"`
	RateLimitPerHour int       `json:"rateLimitPerHour"`
}

func (TaigaConnection20250220) TableName() string {
	return "_tool_taiga_connections"
}

type TaigaProject20250220 struct {
	ConnectionId     uint64 `gorm:"primaryKey"`
	ProjectId        uint64 `gorm:"primaryKey;autoIncrement:false"`
	Name             string `gorm:"type:varchar(255)"`
	Slug             string `gorm:"type:varchar(255)"`
	Description      string `gorm:"type:text"`
	Url              string `gorm:"type:varchar(255)"`
	IsPrivate        bool
	TotalMilestones  int
	TotalStoryPoints float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RawDataParams    string `gorm:"column:_raw_data_params;type:varchar(255);index"`
	RawDataTable     string `gorm:"column:_raw_data_table;type:varchar(255)"`
	RawDataId        uint64 `gorm:"column:_raw_data_id"`
	RawDataRemark    string `gorm:"column:_raw_data_remark"`
	ScopeConfigId    uint64
}

func (TaigaProject20250220) TableName() string {
	return "_tool_taiga_projects"
}

type TaigaScopeConfig20250220 struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	ConnectionId uint64    `json:"connectionId" gorm:"index"`
	Name         string    `gorm:"type:varchar(255);uniqueIndex" json:"name"`
	Entities     string    `gorm:"type:json" json:"entities"`
}

func (TaigaScopeConfig20250220) TableName() string {
	return "_tool_taiga_scope_configs"
}

type TaigaUserStory20250220 struct {
	ConnectionId   uint64 `gorm:"primaryKey"`
	ProjectId      uint64 `gorm:"index"`
	UserStoryId    uint64 `gorm:"primaryKey;autoIncrement:false"`
	Ref            int
	Subject        string `gorm:"type:varchar(255)"`
	Description    string `gorm:"type:text"`
	Status         string `gorm:"type:varchar(100)"`
	StatusColor    string `gorm:"type:varchar(20)"`
	IsClosed       bool
	CreatedDate    *time.Time
	ModifiedDate   *time.Time
	FinishedDate   *time.Time
	AssignedTo     uint64
	AssignedToName string `gorm:"type:varchar(255)"`
	TotalPoints    float64
	MilestoneId    uint64
	MilestoneName  string `gorm:"type:varchar(255)"`
	Priority       int
	IsBlocked      bool
	BlockedNote    string `gorm:"type:text"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RawDataParams  string `gorm:"column:_raw_data_params;type:varchar(255);index"`
	RawDataTable   string `gorm:"column:_raw_data_table;type:varchar(255)"`
	RawDataId      uint64 `gorm:"column:_raw_data_id"`
	RawDataRemark  string `gorm:"column:_raw_data_remark"`
}

func (TaigaUserStory20250220) TableName() string {
	return "_tool_taiga_user_stories"
}

type addInitTables20250220 struct{}

func (*addInitTables20250220) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&TaigaConnection20250220{},
		&TaigaProject20250220{},
		&TaigaScopeConfig20250220{},
		&TaigaUserStory20250220{},
	)
}

func (*addInitTables20250220) Version() uint64 {
	return 20250220000001
}

func (*addInitTables20250220) Name() string {
	return "Taiga init schemas"
}
