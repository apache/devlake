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
	"fmt"
	"net/http"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/taiga/models"
	"github.com/apache/incubator-devlake/server/api/shared"
)

// TaigaTestConnResponse is the response struct for testing a connection
type TaigaTestConnResponse struct {
	shared.ApiBody
	Connection *models.TaigaConnection
}

// testConnection tests the Taiga connection
func testConnection(ctx context.Context, connection models.TaigaConnection) (*TaigaTestConnResponse, errors.Error) {
	// If username and password are provided, authenticate to get a token
	if connection.Username != "" && connection.Password != "" && connection.Token == "" {
		// Create a temporary connection without token for authentication
		tempConnection := connection
		tempConnection.Token = ""

		// Create a temporary API client to call the auth endpoint
		tempApiClient, err := api.NewApiClientFromConnection(ctx, basicRes, &tempConnection)
		if err != nil {
			return nil, errors.Default.Wrap(err, "error creating API client")
		}

		// Prepare auth request body
		authBody := map[string]interface{}{
			"type":     "normal",
			"username": connection.Username,
			"password": connection.Password,
		}

		// Authenticate to get token
		authResponse := struct {
			AuthToken string `json:"auth_token"`
		}{}

		res, err := tempApiClient.Post("auth", nil, authBody, nil)
		if err != nil {
			return nil, errors.Default.Wrap(err, "error authenticating with Taiga")
		}

		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusBadRequest {
			return nil, errors.HttpStatus(http.StatusBadRequest).New("authentication failed - please check your username and password")
		}

		if res.StatusCode != http.StatusOK {
			return nil, errors.HttpStatus(res.StatusCode).New(fmt.Sprintf("unexpected status code during auth: %d", res.StatusCode))
		}

		// Parse the auth response
		err = api.UnmarshalResponse(res, &authResponse)
		if err != nil {
			return nil, errors.Default.Wrap(err, "error parsing authentication response")
		}

		// Set the token for validation
		connection.Token = authResponse.AuthToken
	}

	// validate - but make Token optional if we have username/password
	if vld != nil {
		if connection.Token == "" && (connection.Username == "" || connection.Password == "") {
			return nil, errors.Default.New("either token or username/password must be provided")
		}
	}

	apiClient, err := api.NewApiClientFromConnection(ctx, basicRes, &connection)
	if err != nil {
		return nil, err
	}

	// test connection by making a request to the user endpoint
	res, err := apiClient.Get("users/me", nil, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error testing connection")
	}

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, errors.HttpStatus(http.StatusBadRequest).New("authentication error when testing connection - please check your credentials")
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.HttpStatus(res.StatusCode).New(fmt.Sprintf("unexpected status code: %d", res.StatusCode))
	}

	connection = connection.Sanitize()
	body := TaigaTestConnResponse{}
	body.Success = true
	body.Message = "success"
	body.Connection = &connection

	return &body, nil
}

// TestConnection tests the Taiga connection
// @Summary test taiga connection
// @Description Test Taiga Connection
// @Tags plugins/taiga
// @Param body body models.TaigaConnection true "json body"
// @Success 200  {object} TaigaTestConnResponse "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/taiga/test [POST]
func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// decode
	var connection models.TaigaConnection
	err := api.DecodeMapStruct(input.Body, &connection, false)
	if err != nil {
		return nil, err
	}
	// test connection
	result, err := testConnection(context.TODO(), connection)
	if err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}
	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}

// TestExistingConnection tests an existing Taiga connection
// @Summary test existing taiga connection
// @Description Test Existing Taiga Connection
// @Tags plugins/taiga
// @Success 200  {object} TaigaTestConnResponse "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/taiga/connections/:connectionId/test [POST]
func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "find connection from db")
	}
	// test connection
	result, err := testConnection(context.TODO(), *connection)
	if err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}
	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}

// PostConnections creates a new Taiga connection
// @Summary create taiga connection
// @Description Create Taiga Connection
// @Tags plugins/taiga
// @Success 200  {object} models.TaigaConnection
// @Failure 400
// @Failure 500
// @Router /plugins/taiga/connections [POST]
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Post(input)
}

// ListConnections lists all Taiga connections
// @Summary list taiga connections
// @Description List Taiga Connections
// @Tags plugins/taiga
// @Success 200  {object} []models.TaigaConnection
// @Failure 400
// @Failure 500
// @Router /plugins/taiga/connections [GET]
func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

// GetConnection gets a Taiga connection by ID
// @Summary get taiga connection
// @Description Get Taiga Connection
// @Tags plugins/taiga
// @Success 200  {object} models.TaigaConnection
// @Failure 400
// @Failure 500
// @Router /plugins/taiga/connections/:connectionId [GET]
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}

// PatchConnection updates a Taiga connection
// @Summary patch taiga connection
// @Description Patch Taiga Connection
// @Tags plugins/taiga
// @Success 200  {object} models.TaigaConnection
// @Failure 400
// @Failure 500
// @Router /plugins/taiga/connections/:connectionId [PATCH]
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Patch(input)
}

// DeleteConnection deletes a Taiga connection
// @Summary delete taiga connection
// @Description Delete Taiga Connection
// @Tags plugins/taiga
// @Success 200
// @Failure 400
// @Failure 500
// @Router /plugins/taiga/connections/:connectionId [DELETE]
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}
