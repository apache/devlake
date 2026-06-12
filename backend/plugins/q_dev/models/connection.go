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
	"github.com/apache/incubator-devlake/core/utils"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// Auth type constants for AWS authentication
const (
	AuthTypeAccessKey = "access_key"
	AuthTypeIAMRole   = "iam_role"
)

// QDevConn holds the essential information to connect to AWS S3
type QDevConn struct {
	// AuthType determines how to authenticate with AWS: "access_key" or "iam_role"
	AuthType string `mapstructure:"authType" json:"authType"`
	// AccessKeyId for AWS (required when AuthType is "access_key")
	AccessKeyId string `mapstructure:"accessKeyId" json:"accessKeyId"`
	// SecretAccessKey for AWS (required when AuthType is "access_key")
	SecretAccessKey string `mapstructure:"secretAccessKey" json:"secretAccessKey"`
	// Region for AWS S3
	Region string `mapstructure:"region" json:"region"`
	// Bucket for AWS S3
	Bucket string `mapstructure:"bucket" json:"bucket"`
	// RateLimitPerHour limits the API requests sent to AWS
	RateLimitPerHour int `mapstructure:"rateLimitPerHour" json:"rateLimitPerHour"`

	// Required fields for IAM Identity Center
	// IdentityStoreId for AWS IAM Identity Center (required for user display names)
	IdentityStoreId string `mapstructure:"identityStoreId" json:"identityStoreId"`
	// IdentityStoreRegion for AWS IAM Identity Center (required, may differ from S3 region)
	IdentityStoreRegion string `mapstructure:"identityStoreRegion" json:"identityStoreRegion"`
}

// IsIAMRoleAuth returns true if the connection uses IAM role authentication
func (conn *QDevConn) IsIAMRoleAuth() bool {
	return conn.AuthType == AuthTypeIAMRole
}

func (conn *QDevConn) Sanitize() QDevConn {
	conn.SecretAccessKey = utils.SanitizeString(conn.SecretAccessKey)
	return *conn
}

// QDevConnection holds QDevConn plus ID/Name for database storage
type QDevConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	QDevConn              `mapstructure:",squash"`
}

func (QDevConnection) TableName() string {
	return "_tool_q_dev_connections"
}

func (connection QDevConnection) Sanitize() QDevConnection {
	connection.QDevConn = connection.QDevConn.Sanitize()
	return connection
}

func (connection *QDevConnection) MergeFromRequest(target *QDevConnection, body map[string]interface{}) error {
	secretKey := target.SecretAccessKey
	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}
	modifiedSecretKey := target.SecretAccessKey
	if modifiedSecretKey == "" || modifiedSecretKey == utils.SanitizeString(secretKey) {
		target.SecretAccessKey = secretKey
	}
	return nil
}
