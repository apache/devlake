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
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/rootly/models"
)

func testConnection(ctx context.Context, connection models.RootlyConn) (*plugin.ApiResourceOutput, errors.Error) {
	if vld != nil {
		if err := vld.Struct(connection); err != nil {
			return nil, errors.Default.Wrap(err, "error validating target")
		}
	}
	apiClient, err := api.NewApiClientFromConnection(ctx, basicRes, &connection)
	if err != nil {
		return nil, err
	}
	response, err := apiClient.Get("users/current", nil, nil)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusUnauthorized {
		return nil, errors.HttpStatus(http.StatusBadRequest).New("StatusUnauthorized error while testing connection")
	}
	if response.StatusCode == http.StatusOK {
		return &plugin.ApiResourceOutput{Body: nil, Status: http.StatusOK}, nil
	}
	return &plugin.ApiResourceOutput{Body: nil, Status: response.StatusCode}, errors.HttpStatus(response.StatusCode).Wrap(err, "could not validate connection")
}

// TestConnection test rootly connection
// @Summary test rootly connection
// @Description Test Rootly Connection
// @Tags plugins/rootly
// @Param body body models.RootlyConn true "json body"
// @Success 200  {object} shared.ApiBody "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/rootly/test [POST]
func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var connection models.RootlyConn
	err := api.Decode(input.Body, &connection, vld)
	if err != nil {
		return nil, err
	}
	testConnectionResult, testConnectionErr := testConnection(context.TODO(), connection)
	if testConnectionErr != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, testConnectionErr)
	}
	return testConnectionResult, nil
}

// TestExistingConnection test rootly connection
// @Summary test rootly connection
// @Description Test Rootly Connection
// @Tags plugins/rootly
// @Param connectionId path int true "connection ID"
// @Success 200  {object} shared.ApiBody "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/rootly/connections/{connectionId}/test [POST]
func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "find connection from db")
	}
	if err := api.DecodeMapStruct(input.Body, connection, false); err != nil {
		return nil, err
	}
	testConnectionResult, testConnectionErr := testConnection(context.TODO(), connection.RootlyConn)
	if testConnectionErr != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, testConnectionErr)
	}
	return testConnectionResult, nil
}

// @Summary create rootly connection
// @Description Create Rootly connection
// @Tags plugins/rootly
// @Param body body models.RootlyConnection true "json body"
// @Success 200  {object} models.RootlyConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/rootly/connections [POST]
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Post(input)
}

// @Summary patch rootly connection
// @Description Patch Rootly connection
// @Tags plugins/rootly
// @Param body body models.RootlyConnection true "json body"
// @Success 200  {object} models.RootlyConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/rootly/connections/{connectionId} [PATCH]
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Patch(input)
}

// @Summary delete rootly connection
// @Description Delete Rootly connection
// @Tags plugins/rootly
// @Success 200  {object} models.RootlyConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 409  {object} services.BlueprintProjectPairs "References exist to this connection"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/rootly/connections/{connectionId} [DELETE]
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

// @Summary list rootly connections
// @Description List Rootly connections
// @Tags plugins/rootly
// @Success 200  {object} models.RootlyConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/rootly/connections [GET]
func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

// @Summary get rootly connection
// @Description Get Rootly connection
// @Tags plugins/rootly
// @Success 200  {object} models.RootlyConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/rootly/connections/{connectionId} [GET]
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}
