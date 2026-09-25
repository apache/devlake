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
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

// YoutrackConn holds the essential information to connect to the YouTrack API.
// Auth is a permanent token sent as `Authorization: Bearer perm:...` —
// helper.AccessToken produces exactly that header (`perm:` is part of the
// token value), so no custom SetupAuthentication is needed. YouTrack offers
// no client_credentials OAuth grant for service integrations, so a permanent
// token is the only supported auth method.
type YoutrackConn struct {
	helper.RestConnection `mapstructure:",squash"`
	helper.AccessToken    `mapstructure:",squash"`
}

func (yc *YoutrackConn) Sanitize() YoutrackConn {
	yc.Token = utils.SanitizeString(yc.Token)
	return *yc
}

// YoutrackConnection holds YoutrackConn plus ID/Name for database storage.
// The Token field (inherited from helper.AccessToken) is the only field
// carrying `gorm:"serializer:encdec"` — it is encrypted at rest.
type YoutrackConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	YoutrackConn          `mapstructure:",squash"`
}

func (connection YoutrackConnection) Sanitize() YoutrackConnection {
	connection.YoutrackConn = connection.YoutrackConn.Sanitize()
	return connection
}

// MergeFromRequest merges a PATCH body into the stored connection, keeping
// the stored token when the body carries an empty or the sanitized value, so
// editing the connection name does not wipe the secret.
func (connection *YoutrackConnection) MergeFromRequest(target *YoutrackConnection, body map[string]interface{}) error {
	token := target.Token
	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}
	modifiedToken := target.Token
	if modifiedToken == "" || modifiedToken == utils.SanitizeString(token) {
		target.Token = token
	}
	return nil
}

func (YoutrackConnection) TableName() string {
	return "_tool_youtrack_connections"
}
