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

// CheckmarxoneConn holds the essential fields to connect to CheckmarxOne API
type CheckmarxoneConn struct {
	ServerUrl    string `mapstructure:"serverUrl" validate:"required" json:"serverUrl"`
	Username     string `mapstructure:"username" json:"username"`
	Password     string `mapstructure:"password" json:"-" encrypt:"yes"`
	ClientId     string `mapstructure:"clientId" json:"clientId"`
	ClientSecret string `mapstructure:"clientSecret" json:"-" encrypt:"yes"`
}

// Sanitize returns a sanitized copy (masks secrets for safe display)
func (c CheckmarxoneConn) Sanitize() CheckmarxoneConn {
	c.Password = utils.SanitizeString(c.Password)
	c.ClientSecret = utils.SanitizeString(c.ClientSecret)
	return c
}

// CheckmarxoneConnection is the DB model for a connection record
type CheckmarxoneConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	CheckmarxoneConn      `mapstructure:",squash"`
}

func (CheckmarxoneConnection) TableName() string {
	return "_tool_checkmarxone_connections"
}

// Sanitize returns a sanitized copy
func (c CheckmarxoneConnection) Sanitize() CheckmarxoneConnection {
	c.CheckmarxoneConn = c.CheckmarxoneConn.Sanitize()
	return c
}

// MergeFromRequest merges request body into connection, preserving existing secrets
func (c *CheckmarxoneConnection) MergeFromRequest(target *CheckmarxoneConnection, body map[string]interface{}) error {
	password := target.Password
	secret := target.ClientSecret
	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}
	if target.Password == "" || target.Password == utils.SanitizeString(password) {
		target.Password = password
	}
	if target.ClientSecret == "" || target.ClientSecret == utils.SanitizeString(secret) {
		target.ClientSecret = secret
	}
	return nil
}