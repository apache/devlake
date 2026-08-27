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
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

const (
	// DefaultEndpoint is the Cursor Admin API base URL.
	DefaultEndpoint = "https://api.cursor.com"
	// DefaultRateLimitPerHour matches the documented 20 requests/minute Admin API limit.
	DefaultRateLimitPerHour = 1200
)

const (
	// KeyTierPersonal indicates a User API key (cannot access team endpoints).
	KeyTierPersonal = "personal"
	// KeyTierTeam indicates a Team Admin API key (admin:* scope, no analytics).
	KeyTierTeam = "team"
	// KeyTierEnterprise indicates an Enterprise Admin API key (admin:*, analytics, AI code tracking).
	KeyTierEnterprise = "enterprise"
)

// CursorConn stores Cursor Admin API connection settings.
type CursorConn struct {
	helper.RestConnection `mapstructure:",squash"`

	Token            string `mapstructure:"token" json:"token"`
	KeyTier          string `mapstructure:"keyTier" json:"keyTier" gorm:"type:varchar(20)"`
	HasBugbotReviews bool   `mapstructure:"hasBugbotReviews" json:"hasBugbotReviews"`
}

// SetupAuthentication uses HTTP Basic auth with the API key as username and an empty password.
func (conn *CursorConn) SetupAuthentication(request *http.Request) errors.Error {
	if conn == nil {
		return errors.BadInput.New("connection is required")
	}
	token := strings.TrimSpace(conn.Token)
	if token == "" {
		return errors.BadInput.New("token is required")
	}
	request.SetBasicAuth(token, "")
	return nil
}

func (conn *CursorConn) Sanitize() CursorConn {
	if conn == nil {
		return CursorConn{}
	}
	clone := *conn
	clone.Token = utils.SanitizeString(clone.Token)
	return clone
}

// CursorConnection persists connection details with metadata required by DevLake.
type CursorConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	CursorConn            `mapstructure:",squash"`
}

func (CursorConnection) TableName() string {
	return "_tool_cursor_connections"
}

func (connection CursorConnection) Sanitize() CursorConnection {
	connection.CursorConn = connection.CursorConn.Sanitize()
	return connection
}

func (connection *CursorConnection) Normalize() {
	if connection == nil {
		return
	}
	if connection.Endpoint == "" {
		connection.Endpoint = DefaultEndpoint
	}
	if connection.RateLimitPerHour <= 0 {
		connection.RateLimitPerHour = DefaultRateLimitPerHour
	}
	if connection.KeyTier == "" {
		connection.KeyTier = KeyTierTeam
	}
}

func (connection *CursorConnection) MergeFromRequest(target *CursorConnection, body map[string]interface{}) error {
	if target == nil {
		return nil
	}
	originalToken := target.Token
	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}
	sanitizedOriginal := utils.SanitizeString(originalToken)
	if target.Token == "" || target.Token == sanitizedOriginal {
		target.Token = originalToken
	}
	return nil
}
