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
	"github.com/apache/devlake/core/models/migrationscripts/archived"
	"github.com/apache/devlake/helpers/migrationhelper"
)

type addUsageEventServiceAccountName struct{}

type cursorUsageEvent20261110 struct {
	ConnectionId       uint64    `gorm:"primaryKey"`
	ScopeId            string    `gorm:"primaryKey;type:varchar(255)"`
	EventId            string    `gorm:"primaryKey;type:varchar(64)"`
	EventTime          time.Time `gorm:"index"`
	ServiceAccountName string    `gorm:"type:varchar(255)"`
	archived.NoPKModel
}

func (cursorUsageEvent20261110) TableName() string { return "_tool_cursor_usage_events" }

func (*addUsageEventServiceAccountName) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, &cursorUsageEvent20261110{})
}

func (*addUsageEventServiceAccountName) Version() uint64 {
	return 20261110140000
}

func (*addUsageEventServiceAccountName) Name() string {
	return "cursor add service_account_name on usage events"
}
