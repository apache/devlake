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
	"github.com/apache/incubator-devlake/plugins/tempo/models"
	"github.com/apache/incubator-devlake/server/api/shared"
)

type TempoTestConnResponse struct {
	shared.ApiBody
	Connection *models.TempoConnection
}

func testConnection(ctx context.Context, connection models.TempoConnection) (*TempoTestConnResponse, errors.Error) {
	// Create API client
	apiClient, err := api.NewApiClientFromConnection(ctx, basicRes, &connection)
	if err != nil {
		return nil, err
	}

	// Test connection by fetching teams
	res, err := apiClient.Get("teams", nil, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to test connection to Tempo API")
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.HttpStatus(res.StatusCode).New("failed to connect to Tempo API")
	}

	// Sanitize and return response
	connection = connection.Sanitize()
	body := TempoTestConnResponse{}
	body.Success = true
	body.Message = "success"
	body.Connection = &connection

	return &body, nil
}

// TestConnection test tempo connection
// @Summary test tempo connection
// @Description Test Tempo Connection
// @Tags plugins/tempo
// @Param body body models.TempoConnection true "json body"
// @Success 200  {object} TempoTestConnResponse "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/tempo/test [POST]
func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// Decode
	var connection models.TempoConnection
	if err := api.Decode(input.Body, &connection, nil); err != nil {
		return nil, err
	}
	// Test connection
	result, err := testConnection(context.TODO(), connection)
	if err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}
	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}

// TestExistingConnection test tempo connection
// @Summary test tempo connection
// @Description Test Tempo Connection
// @Tags plugins/tempo
// @Param connectionId path int true "connection ID"
// @Success 200  {object} TempoTestConnResponse "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/tempo/connections/{connectionId}/test [POST]
func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.Convert(err)
	}
	// Test connection
	if result, err := testConnection(context.TODO(), *connection); err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	} else {
		return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
	}
}

// PostConnections create tempo connection
// @Summary create tempo connection
// @Description Create Tempo connection
// @Tags plugins/tempo
// @Param body body models.TempoConnection true "json body"
// @Success 200  {object} models.TempoConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/tempo/connections [POST]
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Post(input)
}

// PatchConnection patch tempo connection
// @Summary patch tempo connection
// @Description Patch Tempo connection
// @Tags plugins/tempo
// @Param body body models.TempoConnection true "json body"
// @Success 200  {object} models.TempoConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/tempo/connections/{connectionId} [PATCH]
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Patch(input)
}

// DeleteConnection delete a tempo connection
// @Summary delete a tempo connection
// @Description Delete a Tempo connection
// @Tags plugins/tempo
// @Success 200  {object} models.TempoConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 409  {object} srvhelper.DsRefs "References exist to this connection"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/tempo/connections/{connectionId} [DELETE]
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

// ListConnections get all tempo connections
// @Summary get all tempo connections
// @Description Get all Tempo connections
// @Tags plugins/tempo
// @Success 200  {object} []models.TempoConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/tempo/connections [GET]
func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

// GetConnection get tempo connection detail
// @Summary get tempo connection detail
// @Description Get Tempo connection detail
// @Tags plugins/tempo
// @Success 200  {object} models.TempoConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/tempo/connections/{connectionId} [GET]
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}

// GetTeams get teams for a connection
// @Summary get teams
// @Description Get teams for a Tempo connection
// @Tags plugins/tempo
// @Param connectionId path int true "connection ID"
// @Success 200  {object} []models.TempoTeam
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/tempo/connections/{connectionId}/teams [GET]
func GetTeams(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.FindByPk(input)
	if err != nil {
		return nil, err
	}

	// Create API client
	apiClient, err := api.NewApiClientFromConnection(context.TODO(), basicRes, connection)
	if err != nil {
		return nil, err
	}

	// Get teams from Tempo API
	res, err := apiClient.Get("teams", nil, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to get teams from Tempo API")
	}

	var teams []models.TempoTeamResponse
	err = api.UnmarshalResponse(res, &teams)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to unmarshal teams response")
	}

	// Convert to tool layer models
	result := make([]models.TempoTeam, 0, len(teams))
	for _, t := range teams {
		result = append(result, *t.ConvertToToolLayer(connection.ID))
	}

	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}
