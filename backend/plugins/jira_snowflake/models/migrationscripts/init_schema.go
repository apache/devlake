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
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/migrationhelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

type initSchema struct{}

// snowflakeJiraConnection20260708 is the schema snapshot at migration version 20260708000001.
// AuthType was not yet present; it was added by the 20260709000001 migration.
type snowflakeJiraConnection20260708 struct {
	helper.BaseConnection `mapstructure:",squash"`
	Account               string `gorm:"column:account;not null"`
	User                  string `gorm:"column:sf_user;not null"`
	PrivateKey            string `gorm:"column:private_key"`
	Database              string `gorm:"column:sf_database;not null"`
	Schema                string `gorm:"column:sf_schema;not null"`
	Warehouse             string `gorm:"column:warehouse"`
	Role                  string `gorm:"column:sf_role"`
}

func (snowflakeJiraConnection20260708) TableName() string {
	return "_tool_jira_snowflake_connections"
}

func (u *initSchema) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, &snowflakeJiraConnection20260708{})
}

func (u *initSchema) Version() uint64 {
	return 20260708000001
}

func (u *initSchema) Name() string {
	return "jira_snowflake init schema"
}
