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
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

// SnowflakeJiraConnection holds the credentials and config for a Snowflake-backed Jira connection.
// The private key PEM is stored encrypted (encrypt:"yes") using DevLake's field-level encryption.
type SnowflakeJiraConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	// Account is the Snowflake account identifier, e.g. "myorg-myaccount"
	Account string `json:"account" gorm:"column:account;not null" mapstructure:"account" validate:"required"`
	// User is the Snowflake service user name
	User string `json:"user" gorm:"column:sf_user;not null" mapstructure:"user" validate:"required"`
	// AuthType controls how the plugin authenticates to Snowflake.
	// "keypair" (default/production): JWT authentication using PrivateKey.
	// "externalbrowser": SSO via browser — only works when DevLake runs on a desktop host (not in a container).
	AuthType string `json:"authType" gorm:"column:auth_type;default:keypair" mapstructure:"authType"`
	// PrivateKey is the RSA private key in PKCS#8 PEM format, stored encrypted.
	// Required when AuthType is "keypair"; ignored for "externalbrowser".
	PrivateKey string `json:"privateKey" encrypt:"yes" gorm:"column:private_key" mapstructure:"privateKey"`
	// Database is the Snowflake database, e.g. "JIRA_DB"
	Database string `json:"database" gorm:"column:sf_database;not null" mapstructure:"database" validate:"required"`
	// Schema is the Snowflake schema, e.g. "CLOUDRHAI_MARTS"
	Schema string `json:"schema" gorm:"column:sf_schema;not null" mapstructure:"schema" validate:"required"`
	// Warehouse is the virtual warehouse to use; defaults to the account default
	Warehouse string `json:"warehouse" gorm:"column:warehouse" mapstructure:"warehouse"`
	// Role is the Snowflake role to assume
	Role string `json:"role" gorm:"column:sf_role" mapstructure:"role"`
}

func (SnowflakeJiraConnection) TableName() string {
	return "_tool_jira_snowflake_connections"
}
