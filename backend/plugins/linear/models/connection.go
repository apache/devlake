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
	"net/http"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/utils"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// LinearConn holds the essential information to connect to the Linear API.
// Linear authenticates with a personal API key passed verbatim in the
// `Authorization` header (NO `Bearer` prefix), so we implement our own
// SetupAuthentication instead of reusing helper.AccessToken.
type LinearConn struct {
	helper.RestConnection `mapstructure:",squash"`
	Token                 string `mapstructure:"token" validate:"required" json:"token" gorm:"serializer:encdec"`
}

// SetupAuthentication sets up the HTTP request authentication for the Linear API.
func (lc *LinearConn) SetupAuthentication(req *http.Request) errors.Error {
	req.Header.Set("Authorization", lc.Token)
	return nil
}

func (lc *LinearConn) Sanitize() LinearConn {
	lc.Token = utils.SanitizeString(lc.Token)
	return *lc
}

// LinearConnection holds LinearConn plus ID/Name for database storage.
type LinearConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	LinearConn            `mapstructure:",squash"`
}

func (connection LinearConnection) Sanitize() LinearConnection {
	connection.LinearConn = connection.LinearConn.Sanitize()
	return connection
}

func (connection *LinearConnection) MergeFromRequest(target *LinearConnection, body map[string]interface{}) error {
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

func (LinearConnection) TableName() string {
	return "_tool_linear_connections"
}
