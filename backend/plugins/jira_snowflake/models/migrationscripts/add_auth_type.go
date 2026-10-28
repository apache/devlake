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

// addAuthType adds the auth_type column and removes the NOT NULL constraint on
// private_key so that externalbrowser connections don't require a key.
type addAuthType struct{}

// snowflakeJiraConnection20260709 is the schema snapshot at migration version 20260709000001.
// Adds AuthType compared to the 20260708 snapshot.
type snowflakeJiraConnection20260709 struct {
	helper.BaseConnection `mapstructure:",squash"`
	Account               string `gorm:"column:account;not null"`
	User                  string `gorm:"column:sf_user;not null"`
	AuthType              string `gorm:"column:auth_type;default:keypair"`
	PrivateKey            string `gorm:"column:private_key"`
	Database              string `gorm:"column:sf_database;not null"`
	Schema                string `gorm:"column:sf_schema;not null"`
	Warehouse             string `gorm:"column:warehouse"`
	Role                  string `gorm:"column:sf_role"`
}

func (snowflakeJiraConnection20260709) TableName() string {
	return "_tool_jira_snowflake_connections"
}

func (u *addAuthType) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, &snowflakeJiraConnection20260709{})
}

func (u *addAuthType) Version() uint64 {
	return 20260709000001
}

func (u *addAuthType) Name() string {
	return "jira_snowflake add auth_type"
}
