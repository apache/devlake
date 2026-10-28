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

package api

import (
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira_snowflake/models"
)

var connHelper *helper.ConnectionApiHelper

// Init initialises the API layer. Called from impl.Init.
func Init(br context.BasicRes, p plugin.PluginMeta) {
	connHelper = helper.NewConnectionHelper(br, nil, p.Name())
}

// PostConnections creates a new SnowflakeJiraConnection.
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.SnowflakeJiraConnection{}
	if err := connHelper.Create(connection, input); err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{Body: connection, Status: 200}, nil
}

// GetConnections returns all SnowflakeJiraConnections.
func GetConnections(_ *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var connections []models.SnowflakeJiraConnection
	if err := connHelper.List(&connections); err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{Body: connections, Status: 200}, nil
}

// GetConnection returns a single connection by connectionId path param.
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.SnowflakeJiraConnection{}
	if err := connHelper.First(connection, input.Params); err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{Body: connection, Status: 200}, nil
}

// PatchConnection updates fields on an existing connection.
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.SnowflakeJiraConnection{}
	if err := connHelper.Patch(connection, input); err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{Body: connection, Status: 200}, nil
}

// DeleteConnection removes a connection.
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.SnowflakeJiraConnection{}
	return connHelper.Delete(connection, input)
}
