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
	"context"
	"net/http"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
	"github.com/apache/incubator-devlake/server/api/shared"
)

const defaultEndpoint = "https://api.linear.app/graphql"

type LinearTestConnResponse struct {
	shared.ApiBody
	Connection *models.LinearConn
}

func testConnection(ctx context.Context, connection models.LinearConn) (*LinearTestConnResponse, errors.Error) {
	if vld != nil {
		if err := vld.Struct(connection); err != nil {
			return nil, errors.Default.Wrap(err, "error validating target")
		}
	}
	if connection.Endpoint == "" {
		connection.Endpoint = defaultEndpoint
	}
	apiClient, err := helper.NewApiClientFromConnection(ctx, basicRes, &connection)
	if err != nil {
		return nil, err
	}
	// Linear is GraphQL-over-HTTP-POST; a minimal viewer query verifies the key.
	reqBody := map[string]interface{}{"query": "{ viewer { id name } }"}
	res, err := apiClient.Post("", nil, reqBody, nil)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "verify token failed")
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, errors.HttpStatus(http.StatusBadRequest).New("authentication failed, please check your API key")
	}
	if res.StatusCode != http.StatusOK {
		return nil, errors.HttpStatus(res.StatusCode).New("unexpected status code while testing connection")
	}
	connection = connection.Sanitize()
	body := LinearTestConnResponse{}
	body.Success = true
	body.Message = "success"
	body.Connection = &connection
	return &body, nil
}

// TestConnection test linear connection
// @Summary test linear connection
// @Description Test linear Connection
// @Tags plugins/linear
// @Param body body models.LinearConn true "json body"
// @Success 200  {object} LinearTestConnResponse "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/linear/test [POST]
func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var connection models.LinearConn
	if err := helper.Decode(input.Body, &connection, vld); err != nil {
		return nil, err
	}
	result, err := testConnection(context.TODO(), connection)
	if err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}
	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}

// TestExistingConnection test linear connection by ID
// @Summary test linear connection
// @Description Test linear Connection
// @Tags plugins/linear
// @Param connectionId path int true "connection ID"
// @Success 200  {object} LinearTestConnResponse "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/linear/connections/{connectionId}/test [POST]
func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "find connection from db")
	}
	if err := helper.DecodeMapStruct(input.Body, connection, false); err != nil {
		return nil, err
	}
	result, testErr := testConnection(context.TODO(), connection.LinearConn)
	if testErr != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, testErr)
	}
	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}

// PostConnections create linear connection
// @Summary create linear connection
// @Description Create linear connection
// @Tags plugins/linear
// @Param body body models.LinearConnection true "json body"
// @Success 200  {object} models.LinearConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/linear/connections [POST]
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Post(input)
}

// PatchConnection patch linear connection
// @Summary patch linear connection
// @Description Patch linear connection
// @Tags plugins/linear
// @Param body body models.LinearConnection true "json body"
// @Success 200  {object} models.LinearConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/linear/connections/{connectionId} [PATCH]
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Patch(input)
}

// DeleteConnection delete a linear connection
// @Summary delete a linear connection
// @Description Delete a linear connection
// @Tags plugins/linear
// @Success 200  {object} models.LinearConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 409  {object} services.BlueprintProjectPairs "References exist to this connection"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/linear/connections/{connectionId} [DELETE]
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

// ListConnections get all linear connections
// @Summary get all linear connections
// @Description Get all linear connections
// @Tags plugins/linear
// @Success 200  {object} []models.LinearConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/linear/connections [GET]
func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

// GetConnection get linear connection detail
// @Summary get linear connection detail
// @Description Get linear connection detail
// @Tags plugins/linear
// @Success 200  {object} models.LinearConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/linear/connections/{connectionId} [GET]
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}
