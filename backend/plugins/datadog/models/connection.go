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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

// DatadogAccessToken carries the pair of keys Datadog requires: an API key
// identifying the organization and an application key identifying the caller.
// Both travel as headers, so this is not a bearer-token connection.
type DatadogAccessToken struct {
	ApiKey         string `mapstructure:"apiKey" validate:"required" encrypt:"yes" json:"apiKey"`
	ApplicationKey string `mapstructure:"applicationKey" validate:"required" encrypt:"yes" json:"applicationKey"`
}

func (at *DatadogAccessToken) SetupAuthentication(request *http.Request) errors.Error {
	request.Header.Set("DD-API-KEY", at.ApiKey)
	request.Header.Set("DD-APPLICATION-KEY", at.ApplicationKey)
	return nil
}

type DatadogConn struct {
	helper.RestConnection `mapstructure:",squash"`
	DatadogAccessToken    `mapstructure:",squash"`
	// WebUrl is the organization's Datadog UI origin, e.g.
	// https://app.datadoghq.com. The API returns no incident permalink, so
	// this is what makes a domain issue clickable. Optional.
	WebUrl string `mapstructure:"webUrl" json:"webUrl"`
}

func (connection DatadogConn) Sanitize() DatadogConn {
	connection.ApiKey = utils.SanitizeString(connection.ApiKey)
	connection.ApplicationKey = utils.SanitizeString(connection.ApplicationKey)
	return connection
}

type DatadogConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	DatadogConn           `mapstructure:",squash"`
}

// MergeFromRequest preserves the stored keys when an incoming PATCH body
// omits one or echoes back the sanitized form. The config-UI sends the
// sanitized secrets on every PATCH, and this guard is what makes that safe.
func (connection *DatadogConnection) MergeFromRequest(target *DatadogConnection, body map[string]interface{}) error {
	apiKey := target.ApiKey
	appKey := target.ApplicationKey
	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}
	if target.ApiKey == "" || target.ApiKey == utils.SanitizeString(apiKey) {
		target.ApiKey = apiKey
	}
	if target.ApplicationKey == "" || target.ApplicationKey == utils.SanitizeString(appKey) {
		target.ApplicationKey = appKey
	}
	return nil
}

func (DatadogConnection) TableName() string {
	return "_tool_datadog_connections"
}

func (connection DatadogConnection) Sanitize() DatadogConnection {
	connection.DatadogConn = connection.DatadogConn.Sanitize()
	return connection
}
