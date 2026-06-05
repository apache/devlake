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
	"fmt"
	"net/http"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/utils"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// AsanaConn holds the essential information to connect to the Asana API
type AsanaConn struct {
	helper.RestConnection `mapstructure:",squash"`
	Token                 string `mapstructure:"token" json:"token" gorm:"serializer:encdec"`
}

func (ac *AsanaConn) Sanitize() AsanaConn {
	ac.Token = utils.SanitizeString(ac.Token)
	return *ac
}

// AsanaConnection holds AsanaConn plus ID/Name for database storage
type AsanaConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	AsanaConn             `mapstructure:",squash"`
}

func (connection *AsanaConnection) MergeFromRequest(target *AsanaConnection, body map[string]interface{}) error {
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

func (connection AsanaConnection) Sanitize() AsanaConnection {
	connection.AsanaConn = connection.AsanaConn.Sanitize()
	return connection
}

// SetupAuthentication sets up the HTTP Request Authentication (Bearer token)
func (ac *AsanaConn) SetupAuthentication(req *http.Request) errors.Error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ac.Token))
	return nil
}

func (AsanaConnection) TableName() string {
	return "_tool_asana_connections"
}
