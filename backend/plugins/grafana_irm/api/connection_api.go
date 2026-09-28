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
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/grafana_irm/models"
)

// testConnection calls IncidentsService.QueryIncidents with limit:1 and an
// empty queryString — the cheapest call that's valid on every stack (even one
// with zero incidents) and exercises real auth, per grafana_irm_plan.md §3.4
// (OrderDirection is required; queryString "" matches everything). Mirrors
// incidentio's testConnection pattern (api/connection_api.go there), adapted
// from a REST GET to this API's JSON-RPC POST shape.
func testConnection(ctx context.Context, connection models.GrafanaIrmConn) (*plugin.ApiResourceOutput, errors.Error) {
	if vld != nil {
		if err := vld.Struct(connection); err != nil {
			return nil, errors.BadInput.New(fmt.Sprintf("Validation failed: %s", err.Error()))
		}
	}
	apiClient, err := api.NewApiClientFromConnection(ctx, basicRes, &connection)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "no such host") || strings.Contains(errMsg, "Failed to resolve DNS") {
			return nil, errors.BadInput.New(fmt.Sprintf("Failed to resolve hostname for '%s'. Please check your Grafana Cloud URL for typos (e.g. https://<stack>.grafana.net/).", connection.Endpoint))
		}
		if strings.Contains(errMsg, "Invalid URL") || strings.Contains(errMsg, "scheme") {
			return nil, errors.BadInput.New(fmt.Sprintf("Invalid endpoint URL '%s': please ensure it starts with https:// (e.g. https://<stack>.grafana.net/).", connection.Endpoint))
		}
		if strings.Contains(errMsg, "Failed to connect") || strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "i/o timeout") {
			return nil, errors.BadInput.New(fmt.Sprintf("Failed to connect to '%s' (connection timed out or refused). Please check that the URL is spelled correctly (e.g. https://<stack>.grafana.net/) and verify your network or proxy settings.", connection.Endpoint))
		}
		if idx := strings.Index(errMsg, " Wraps:"); idx != -1 {
			errMsg = errMsg[:idx]
		}
		return nil, errors.BadInput.New(fmt.Sprintf("Invalid endpoint URL '%s': %s", connection.Endpoint, errMsg))
	}
	body := map[string]interface{}{
		"query": map[string]interface{}{
			"limit":          1,
			"orderDirection": "ASC",
			"queryString":    "",
		},
	}
	response, err := apiClient.Post("api/plugins/grafana-irm-app/resources/api/v1/IncidentsService.QueryIncidents", nil, body, nil)
	if err != nil {
		errMsg := err.Error()
		if idx := strings.Index(errMsg, " Wraps:"); idx != -1 {
			errMsg = errMsg[:idx]
		}
		if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "i/o timeout") {
			return nil, errors.BadInput.New(fmt.Sprintf("Request to Grafana IRM API timed out at '%s'. Please check that your stack URL is spelled correctly and verify your network connection.", connection.Endpoint))
		}
		return nil, errors.BadInput.New(fmt.Sprintf("Failed to reach Grafana IRM API at '%s': %s", connection.Endpoint, errMsg))
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, errors.BadInput.New("Authentication failed: invalid Service Account token or insufficient permissions for Grafana IRM.")
	}
	if response.StatusCode == http.StatusNotFound {
		return nil, errors.BadInput.New(fmt.Sprintf("Grafana IRM app endpoint not found (HTTP 404) at '%s'. Please ensure the endpoint is your Grafana Cloud stack base URL (e.g. https://<stack>.grafana.net/).", connection.Endpoint))
	}
	if response.StatusCode == http.StatusOK {
		return &plugin.ApiResourceOutput{Body: nil, Status: http.StatusOK}, nil
	}
	return &plugin.ApiResourceOutput{Body: nil, Status: response.StatusCode}, errors.BadInput.New(fmt.Sprintf("Connection test failed with HTTP status %d. Please verify your stack URL and token.", response.StatusCode))
}

// TestConnection test grafana_irm connection
// @Summary test grafana_irm connection
// @Description Test Grafana IRM Connection
// @Tags plugins/grafana_irm
// @Param body body models.GrafanaIrmConn true "json body"
// @Success 200  {object} shared.ApiBody "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/grafana_irm/test [POST]
func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var connection models.GrafanaIrmConn
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

// TestExistingConnection test grafana_irm connection
// @Summary test grafana_irm connection
// @Description Test Grafana IRM Connection
// @Tags plugins/grafana_irm
// @Param connectionId path int true "connection ID"
// @Success 200  {object} shared.ApiBody "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/grafana_irm/connections/{connectionId}/test [POST]
func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "find connection from db")
	}
	if err := api.DecodeMapStruct(input.Body, connection, false); err != nil {
		return nil, err
	}
	testConnectionResult, testConnectionErr := testConnection(context.TODO(), connection.GrafanaIrmConn)
	if testConnectionErr != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, testConnectionErr)
	}
	return testConnectionResult, nil
}

// @Summary create grafana_irm connection
// @Description Create Grafana IRM connection
// @Tags plugins/grafana_irm
// @Param body body models.GrafanaIrmConnection true "json body"
// @Success 200  {object} models.GrafanaIrmConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/grafana_irm/connections [POST]
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Post(input)
}

// @Summary patch grafana_irm connection
// @Description Patch Grafana IRM connection
// @Tags plugins/grafana_irm
// @Param body body models.GrafanaIrmConnection true "json body"
// @Success 200  {object} models.GrafanaIrmConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/grafana_irm/connections/{connectionId} [PATCH]
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Patch(input)
}

// @Summary delete grafana_irm connection
// @Description Delete Grafana IRM connection
// @Tags plugins/grafana_irm
// @Success 200  {object} models.GrafanaIrmConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 409  {object} services.BlueprintProjectPairs "References exist to this connection"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/grafana_irm/connections/{connectionId} [DELETE]
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

// @Summary list grafana_irm connections
// @Description List Grafana IRM connections
// @Tags plugins/grafana_irm
// @Success 200  {object} models.GrafanaIrmConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/grafana_irm/connections [GET]
func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

// @Summary get grafana_irm connection
// @Description Get Grafana IRM connection
// @Tags plugins/grafana_irm
// @Success 200  {object} models.GrafanaIrmConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/grafana_irm/connections/{connectionId} [GET]
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}
