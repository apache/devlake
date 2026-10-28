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

var _ plugin.MigrationScript = (*addScopeConfigTable)(nil)

// testRegistryScopeConfig20250103 is a frozen snapshot of models.TestRegistryScopeConfig as of 2025-01-03.
// Do not update this struct — create a new migration instead.
type testRegistryScopeConfig20250103 struct {
	// common.Model fields
	ID        uint64    `gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// common.ScopeConfig fields
	Entities     []string `gorm:"type:json;serializer:json" json:"entities"`
	ConnectionId uint64   `json:"connectionId" gorm:"index"`
	Name         string   `gorm:"type:varchar(255);uniqueIndex" json:"name"`
}

func (testRegistryScopeConfig20250103) TableName() string {
	return "_tool_testregistry_scope_configs"
}

type addScopeConfigTable struct{}

func (*addScopeConfigTable) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&testRegistryScopeConfig20250103{},
	)
}

func (*addScopeConfigTable) Version() uint64 {
	return 20250103000001
}

func (*addScopeConfigTable) Name() string {
	return "add testregistry scope config table"
}
