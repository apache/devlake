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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

// GrafanaIrmAccessToken authenticates against the Grafana Incident API using
// a Grafana Cloud Service Account token (see grafana_irm_plan.md §3).
type GrafanaIrmAccessToken helper.AccessToken

func (at *GrafanaIrmAccessToken) SetupAuthentication(request *http.Request) errors.Error {
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at.Token))
	return nil
}

type GrafanaIrmConn struct {
	helper.RestConnection `mapstructure:",squash"`
	GrafanaIrmAccessToken `mapstructure:",squash"`
}

func (connection GrafanaIrmConn) Sanitize() GrafanaIrmConn {
	connection.Token = utils.SanitizeString(connection.Token)
	return connection
}

type GrafanaIrmConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	GrafanaIrmConn        `mapstructure:",squash"`
}

// MergeFromRequest preserves the existing token when an incoming PATCH body
// omits it or echoes the sanitized form back (see incidentio's Connection
// for the same pattern).
func (connection *GrafanaIrmConnection) MergeFromRequest(target *GrafanaIrmConnection, body map[string]interface{}) error {
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

func (GrafanaIrmConnection) TableName() string {
	return "_tool_grafana_irm_connections"
}

func (connection GrafanaIrmConnection) Sanitize() GrafanaIrmConnection {
	connection.Token = utils.SanitizeString(connection.Token)
	return connection
}
