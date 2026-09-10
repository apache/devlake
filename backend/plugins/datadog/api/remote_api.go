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
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/plugins/datadog/models"
	"github.com/apache/devlake/plugins/datadog/models/raw"
)

type DatadogRemotePagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type incidentTypesResponse struct {
	Data []raw.IncidentType `json:"data"`
}

// queryDatadogRemoteScopes lists incident types as scopes. The
// incident-types config endpoint returns the whole list in one response,
// so search is applied client-side and there is never a next page.
func queryDatadogRemoteScopes(
	apiClient plugin.ApiClient,
	_ string,
	_ DatadogRemotePagination,
	search string,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.IncidentType],
	nextPage *DatadogRemotePagination,
	err errors.Error,
) {
	var res *http.Response
	res, err = apiClient.Get("incidents/config/types", nil, nil)
	if err != nil {
		return
	}
	response := &incidentTypesResponse{}
	err = api.UnmarshalResponse(res, response)
	if err != nil {
		return
	}
	for _, item := range response.Data {
		if item.Attributes == nil {
			continue
		}
		name := item.Attributes.Name
		if search != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(search)) {
			continue
		}
		entry := dsmodels.DsRemoteApiScopeListEntry[models.IncidentType]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			Id:       item.Id,
			Name:     name,
			FullName: name,
			Data: &models.IncidentType{
				Id:   item.Id,
				Name: name,
			},
		}
		if item.Attributes.Prefix != nil {
			entry.Data.Prefix = *item.Attributes.Prefix
		}
		if item.Attributes.IsDefault != nil {
			entry.Data.IsDefault = *item.Attributes.IsDefault
		}
		if item.Attributes.CreatedAt != nil {
			entry.Data.Scope.NoPKModel.CreatedAt = *item.Attributes.CreatedAt
			// Also on the tool model, so a scope added from here carries the
			// date before the first collection fills it in.
			entry.Data.CreatedDate = item.Attributes.CreatedAt
		}
		children = append(children, entry)
	}

	return
}

func listDatadogRemoteScopes(
	connection *models.DatadogConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page DatadogRemotePagination,
) (
	[]dsmodels.DsRemoteApiScopeListEntry[models.IncidentType],
	*DatadogRemotePagination,
	errors.Error,
) {
	return queryDatadogRemoteScopes(apiClient, groupId, page, "")
}

func searchDatadogRemoteScopes(
	apiClient plugin.ApiClient,
	params *dsmodels.DsRemoteApiScopeSearchParams,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.IncidentType],
	err errors.Error,
) {
	children, _, err = queryDatadogRemoteScopes(apiClient, "", DatadogRemotePagination{
		Page:    params.Page,
		PerPage: params.PageSize,
	}, params.Search)
	return
}

// RemoteScopes list all available scopes (incident types) for this connection
// @Summary list all available scopes (incident types) for this connection
// @Description list all available scopes (incident types) for this connection
// @Tags plugins/datadog
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param groupId query string false "group ID"
// @Param pageToken query string false "page Token"
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.IncidentType]
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/datadog/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

// SearchRemoteScopes searches incident types by name
// @Summary searches incident types by name
// @Description searches incident types by name
// @Tags plugins/datadog
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param search query string false "search"
// @Param page query int false "page number"
// @Param pageSize query int false "page size per page"
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.IncidentType]
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/datadog/connections/{connectionId}/search-remote-scopes [GET]
func SearchRemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeSearch.Get(input)
}
