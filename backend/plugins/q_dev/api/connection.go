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
	"net/http"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/q_dev/models"
)

// 连接项目的CRUD API

// PostConnections 创建新连接 (enhanced with Identity Store validation)
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// 创建连接
	connection := &models.QDevConnection{}
	err := api.Decode(input.Body, connection, vld)
	if err != nil {
		return nil, err
	}

	// 验证连接参数 (enhanced validation)
	normalizeConnection(connection)
	if err := validateConnection(connection); err != nil {
		return nil, errors.BadInput.Wrap(err, "connection validation failed")
	}

	// 保存到数据库
	err = connectionHelper.Create(connection, input)
	if err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{Body: connection.Sanitize(), Status: http.StatusOK}, nil
}

// PatchConnection 更新现有连接 (enhanced with Identity Store validation)
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.QDevConnection{}
	if err := connectionHelper.First(&connection, input.Params); err != nil {
		return nil, err
	}
	if err := (&models.QDevConnection{}).MergeFromRequest(connection, input.Body); err != nil {
		return nil, errors.Convert(err)
	}

	// 验证更新后的连接参数 (enhanced validation)
	normalizeConnection(connection)
	if err := validateConnection(connection); err != nil {
		return nil, errors.BadInput.Wrap(err, "connection validation failed")
	}

	if err := connectionHelper.SaveWithCreateOrUpdate(connection); err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{Body: connection.Sanitize(), Status: http.StatusOK}, nil
}

// DeleteConnection 删除连接
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	conn := &models.QDevConnection{}
	output, err := connectionHelper.Delete(conn, input)
	if err != nil {
		return output, err
	}
	output.Body = conn.Sanitize()
	return output, nil
}

// ListConnections 列出所有连接
func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var connections []models.QDevConnection
	err := connectionHelper.List(&connections)
	if err != nil {
		return nil, err
	}
	// 敏感信息脱敏
	for i := 0; i < len(connections); i++ {
		connections[i] = connections[i].Sanitize()
	}
	return &plugin.ApiResourceOutput{Body: connections}, nil
}

// GetConnection 获取单个连接详情
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.QDevConnection{}
	err := connectionHelper.First(connection, input.Params)
	if err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{Body: connection.Sanitize()}, err
}

// normalizeConnection applies defaults to connection fields.
// Call this before validateConnection to set sensible defaults.
func normalizeConnection(connection *models.QDevConnection) {
	if connection.AuthType == "" {
		connection.AuthType = models.AuthTypeAccessKey
	}
	if connection.RateLimitPerHour == 0 {
		connection.RateLimitPerHour = 20000
	}
}

// validateConnection validates connection parameters including Identity Store fields.
// This function is pure — it does not mutate the connection.
func validateConnection(connection *models.QDevConnection) error {
	if connection.AuthType != models.AuthTypeAccessKey && connection.AuthType != models.AuthTypeIAMRole {
		return errors.Default.New("AuthType must be 'access_key' or 'iam_role'")
	}

	// Validate AWS credentials only for access_key auth type
	if !connection.IsIAMRoleAuth() {
		if connection.AccessKeyId == "" {
			return errors.Default.New("AccessKeyId is required")
		}
		if connection.SecretAccessKey == "" {
			return errors.Default.New("SecretAccessKey is required")
		}
	}
	if connection.Region == "" {
		return errors.Default.New("Region is required")
	}
	if connection.Bucket == "" {
		return errors.Default.New("Bucket is required")
	}

	// Identity Store fields are optional, but must be provided together if used
	if connection.IdentityStoreId == "" && connection.IdentityStoreRegion != "" {
		return errors.Default.New("IdentityStoreRegion provided but IdentityStoreId is empty")
	}
	if connection.IdentityStoreId != "" && connection.IdentityStoreRegion == "" {
		return errors.Default.New("IdentityStoreId provided but IdentityStoreRegion is empty")
	}

	// Validate rate limit
	if connection.RateLimitPerHour < 0 {
		return errors.Default.New("RateLimitPerHour must be positive")
	}

	return nil
}
