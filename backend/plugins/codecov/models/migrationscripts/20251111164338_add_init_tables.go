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
	"github.com/apache/devlake/helpers/migrationhelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

// codecovConnection20251111 is a frozen snapshot of models.CodecovConnection as of 2025-11-11.
// Do not update this struct — create a new migration instead.
type codecovConnection20251111 struct {
	helper.BaseConnection `mapstructure:",squash"`
	helper.RestConnection `mapstructure:",squash"`
	helper.AccessToken    `mapstructure:",squash"`
	Organization          string `gorm:"type:varchar(255)"`
}

func (codecovConnection20251111) TableName() string {
	return "_tool_codecov_connections"
}

// codecovRepo20251111 is a frozen snapshot of models.CodecovRepo as of 2025-11-11.
// Do not update this struct — create a new migration instead.
type codecovRepo20251111 struct {
	// common.Scope fields (inlined)
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	RawDataParams string    `gorm:"column:_raw_data_params;type:varchar(255);index" json:"_raw_data_params"`
	RawDataTable  string    `gorm:"column:_raw_data_table;type:varchar(255)" json:"_raw_data_table"`
	RawDataId     uint64    `gorm:"column:_raw_data_id" json:"_raw_data_id"`
	RawDataRemark string    `gorm:"column:_raw_data_remark" json:"_raw_data_remark"`
	ConnectionId  uint64    `json:"connectionId" gorm:"primaryKey"`
	ScopeConfigId uint64    `json:"scopeConfigId,omitempty"`
	// CodecovRepo-specific fields
	CodecovId   string `gorm:"primaryKey;type:varchar(255)"`
	Name        string `gorm:"type:varchar(255)"`
	FullName    string `gorm:"type:varchar(255)"`
	Service     string `gorm:"type:varchar(255)"`
	Language    string `gorm:"type:varchar(255)"`
	Active      bool
	ActivatedAt string `gorm:"type:varchar(255)"`
	Updatestamp string `gorm:"type:varchar(255)"`
	Private     bool
	Branch      string `gorm:"type:varchar(255)"`
}

func (codecovRepo20251111) TableName() string {
	return "_tool_codecov_repos"
}

// codecovScopeConfig20251111 is a frozen snapshot of models.CodecovScopeConfig as of 2025-11-11.
// Do not update this struct — create a new migration instead.
type codecovScopeConfig20251111 struct {
	// common.ScopeConfig fields (inlined)
	ID           uint64    `gorm:"primaryKey"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Entities     []string  `gorm:"type:json;serializer:json" json:"entities"`
	ConnectionId uint64    `json:"connectionId" gorm:"index"`
	Name         string    `gorm:"type:varchar(255);uniqueIndex" json:"name"`
}

func (codecovScopeConfig20251111) TableName() string {
	return "_tool_codecov_scope_configs"
}

type addInitTables struct{}

func (u *addInitTables) Up(basicRes context.BasicRes) errors.Error {
	err := migrationhelper.AutoMigrateTables(
		basicRes,
		&codecovConnection20251111{},
		&codecovRepo20251111{},
		&codecovScopeConfig20251111{},
	)
	return err
}

func (*addInitTables) Version() uint64 {
	return 20251111164338
}

func (*addInitTables) Name() string {
	return "Codecov init schemas"
}
