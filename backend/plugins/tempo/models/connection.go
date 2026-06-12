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
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/models/common"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// TempoConn holds the essential information to connect to the Tempo API
type TempoConn struct {
	helper.RestConnection `mapstructure:",squash"`
	helper.AccessToken    `mapstructure:",squash"`
}

// TempoConnection holds TempoConn plus ID/Name for database storage
type TempoConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	TempoConn             `mapstructure:",squash"`
}

func (TempoConnection) TableName() string {
	return "_tool_tempo_connections"
}

func (connection TempoConnection) Connection() dal.Tabler {
	return &connection
}

func (connection TempoConnection) Sanitize() TempoConnection {
	connection.TempoConn.Token = ""
	return connection
}

// This object conforms to what the frontend currently expects.
type TempoResponse struct {
	Name string `json:"name"`
	ID   uint64 `json:"id"`
	TempoConnection
}

// TempoScopeConfig holds the configuration for a Tempo scope
type TempoScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`
	Name               string `json:"name" mapstructure:"name" gorm:"type:varchar(255)"`
}

func (TempoScopeConfig) TableName() string {
	return "_tool_tempo_scope_configs"
}

func (c TempoScopeConfig) ScopeConfigId() uint64 {
	return c.ID
}

func (c TempoScopeConfig) ScopeConfigConnectionId() uint64 {
	return c.ConnectionId
}
