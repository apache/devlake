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

	"github.com/mitchellh/mapstructure"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
	"github.com/apache/devlake/server/api/shared"
)

type CodecovTestConnResponse struct {
	shared.ApiBody
	Organization string `json:"organization"`
}

// TestConnection test codecov connection
// @Summary test codecov connection
// @Description Test codecov Connection
// @Tags plugins/codecov
// @Param body body models.CodecovConn true "json body"
// @Success 200  {object} CodecovTestConnResponse
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/codecov/test [POST]
func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// process input
	var conn models.CodecovConn
	e := mapstructure.Decode(input.Body, &conn)
	if e != nil {
		return nil, errors.Convert(e)
	}
	testConnectionResult, err := testConnection(context.TODO(), conn)
	if err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}
	return &plugin.ApiResourceOutput{Body: testConnectionResult, Status: http.StatusOK}, nil
}

// @Summary create codecov connection
// @Description Create codecov connection
// @Tags plugins/codecov
// @Param body body models.CodecovConnection true "json body"
// @Success 200  {object} models.CodecovConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/codecov/connections [POST]
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Post(input)
}

// @Summary patch codecov connection
// @Description Patch codecov connection
// @Tags plugins/codecov
// @Param body body models.CodecovConnection true "json body"
// @Success 200  {object} models.CodecovConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/codecov/connections/{connectionId} [PATCH]
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Patch(input)
}

// @Summary delete a codecov connection
// @Description Delete a codecov connection
// @Tags plugins/codecov
// @Success 200  {object} models.CodecovConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 409  {object} srvhelper.DsRefs "References exist to this connection"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/codecov/connections/{connectionId} [DELETE]
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

// @Summary get all codecov connections
// @Description Get all codecov connections
// @Tags plugins/codecov
// @Success 200  {object} []models.CodecovConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/codecov/connections [GET]
func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

// @Summary get codecov connection detail
// @Description Get codecov connection detail
// @Tags plugins/codecov
// @Success 200  {object} models.CodecovConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/codecov/connections/{connectionId} [GET]
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}

// @Summary test existing codecov connection
// @Description Test existing codecov connection
// @Tags plugins/codecov
// @Success 200  {object} CodecovTestConnResponse
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/test [POST]
func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.Convert(err)
	}
	testConnectionResult, testConnectionErr := testConnection(context.TODO(), connection.CodecovConn)
	if testConnectionErr != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, testConnectionErr)
	}
	return &plugin.ApiResourceOutput{Body: testConnectionResult, Status: http.StatusOK}, nil
}

func testConnection(ctx context.Context, conn models.CodecovConn) (*CodecovTestConnResponse, errors.Error) {
	if vld != nil {
		if err := vld.Struct(conn); err != nil {
			return nil, errors.Convert(err)
		}
	}

	if err := conn.ValidateService(); err != nil {
		return nil, err
	}

	apiClient, err := api.NewApiClientFromConnection(ctx, basicRes, &conn)
	if err != nil {
		return nil, err
	}

	service := conn.ServiceOrDefault()
	testUrl := fmt.Sprintf("/api/v2/%s/%s/users", service, conn.Organization)
	res, err := apiClient.Get(testUrl, nil, nil)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "verify token failed")
	}
	if res.StatusCode == http.StatusUnauthorized {
		return nil, errors.HttpStatus(http.StatusBadRequest).New("StatusUnauthorized error when testing connection")
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, errors.HttpStatus(http.StatusBadRequest).New(fmt.Sprintf("Organization '%s' not found or token does not have access", conn.Organization))
	}
	if res.StatusCode != http.StatusOK {
		return nil, errors.HttpStatus(res.StatusCode).New("unexpected status code while testing connection")
	}

	return &CodecovTestConnResponse{
		ApiBody: shared.ApiBody{
			Success: true,
			Message: "success",
		},
		Organization: conn.Organization,
	}, nil
}
