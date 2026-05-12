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

// RootlyAccessToken implements HTTP Bearer Authentication with an access token
type RootlyAccessToken helper.AccessToken

// SetupAuthentication sets up the request headers for authentication
func (at *RootlyAccessToken) SetupAuthentication(request *http.Request) errors.Error {
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at.Token))
	return nil
}

// RootlyConn holds the essential information to connect to the Rootly API
type RootlyConn struct {
	helper.RestConnection `mapstructure:",squash"`
	RootlyAccessToken     `mapstructure:",squash"`
}

// RootlyConnection holds RootlyConn plus ID/Name for database storage
type RootlyConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	RootlyConn            `mapstructure:",squash"`
}

func (connection *RootlyConnection) MergeFromRequest(target *RootlyConnection, body map[string]interface{}) error {
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

// This object conforms to what the frontend currently expects.
type RootlyResponse struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	RootlyConnection
}

// ApiUserResponse represents the Rootly /users/current response for token validation.
type ApiUserResponse struct {
	Id   string
	Name string `json:"name"`
}

func (RootlyConnection) TableName() string {
	return "_tool_rootly_connections"
}

func (connection RootlyConnection) Sanitize() RootlyConnection {
	connection.Token = utils.SanitizeString(connection.Token)
	return connection
}
