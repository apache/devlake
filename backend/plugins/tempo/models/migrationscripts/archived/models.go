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

package archived

import (
	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
)

type TempoConnection struct {
	archived.BaseConnection `mapstructure:",squash"`
	archived.RestConnection `mapstructure:",squash"`
	archived.AccessToken    `mapstructure:",squash"`
}

func (TempoConnection) TableName() string {
	return "_tool_tempo_connections"
}

type TempoScopeConfig struct {
	archived.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`
	ConnectionId         uint64 `mapstructure:"connectionId" json:"connectionId"`
	Name                 string `gorm:"type:varchar(255)" mapstructure:"name" json:"name"`
}

func (TempoScopeConfig) TableName() string {
	return "_tool_tempo_scope_configs"
}

type TempoTeam struct {
	ConnectionId  uint64 `gorm:"primaryKey"`
	TeamId        int64  `gorm:"primaryKey"`
	ScopeConfigId uint64
	Key           string `gorm:"type:varchar(255)"`
	Name          string `gorm:"type:varchar(255)"`
	Summary       string `gorm:"type:varchar(255)"`
	archived.NoPKModel
}

func (TempoTeam) TableName() string {
	return "_tool_tempo_teams"
}

type TempoWorklog struct {
	ConnectionId     uint64 `gorm:"primaryKey"`
	TempoWorklogId   int64  `gorm:"primaryKey"`
	TeamId           int64  `gorm:"index"`
	IssueId          int64  `gorm:"index"`
	IssueKey         string `gorm:"type:varchar(255)"`
	AuthorAccountId  string `gorm:"type:varchar(255)"`
	TimeSpentSeconds int
	BillableSeconds  int
	StartDate        string `gorm:"type:varchar(255)"`
	StartTime        string `gorm:"type:varchar(255)"`
	Description      string `gorm:"type:text"`
	CreatedAt        string `gorm:"type:varchar(255)"`
	UpdatedAt        string `gorm:"type:varchar(255)"`
	archived.NoPKModel
}

func (TempoWorklog) TableName() string {
	return "_tool_tempo_worklogs"
}
