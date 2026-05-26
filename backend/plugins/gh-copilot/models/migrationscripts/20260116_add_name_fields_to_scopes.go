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
	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

// addNameFieldsToScopes adds name and fullName columns to _tool_copilot_scopes.
// These fields are required by the UI for displaying data scopes in the connection page.
type addNameFieldsToScopes struct{}

type ghCopilotScope20260116 struct {
	archived.NoPKModel
	ConnectionId       uint64     `json:"connectionId" gorm:"primaryKey"`
	ScopeConfigId      uint64     `json:"scopeConfigId,omitempty"`
	Id                 string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Organization       string     `json:"organization" gorm:"type:varchar(255)"`
	Name               string     `json:"name" gorm:"type:varchar(255)"`
	FullName           string     `json:"fullName" gorm:"type:varchar(255)"`
	ImplementationDate *time.Time `json:"implementationDate"`
	BaselinePeriodDays int        `json:"baselinePeriodDays" gorm:"default:90"`
	SeatsLastSyncedAt  *time.Time `json:"seatsLastSyncedAt"`
}

func (ghCopilotScope20260116) TableName() string {
	return "_tool_copilot_scopes"
}

func (script *addNameFieldsToScopes) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&ghCopilotScope20260116{},
	)
}

func (*addNameFieldsToScopes) Version() uint64 {
	return 20260116000000
}

func (*addNameFieldsToScopes) Name() string {
	return "copilot add name fields to scopes"
}
