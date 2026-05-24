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
	"net/url"
	"strconv"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/incubator-devlake/helpers/pluginhelper/api/models"
	"github.com/apache/incubator-devlake/plugins/tempo/models"
)

type TempoRemotePagination struct {
	Limit  int `json:"limit" mapstructure:"limit"`
	Offset int `json:"offset" mapstructure:"offset"`
}

func listTempoRemoteTeams(
	connection *models.TempoConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page TempoRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.TempoTeam],
	nextPage *TempoRemotePagination,
	err errors.Error,
) {
	if page.Limit == 0 {
		page.Limit = 50
	}

	queryParams := url.Values{
		"offset": {strconv.Itoa(page.Offset)},
		"limit":  {strconv.Itoa(page.Limit)},
	}

	res, err := apiClient.Get("teams", queryParams, nil)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to get teams from Tempo API")
	}

	var response struct {
		Metadata struct {
			Count  int `json:"count"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
			Total  int `json:"total"`
		} `json:"metadata"`
		Results []models.TempoTeamResponse `json:"results"`
	}
	err = api.UnmarshalResponse(res, &response)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal teams response")
	}

	for _, team := range response.Results {
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.TempoTeam]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			Id:       strconv.FormatInt(team.Id, 10),
			ParentId: nil,
			Name:     team.Name,
			FullName: team.Name,
			Data:     team.ConvertToToolLayer(connection.ID),
		})
	}

	if page.Offset+page.Limit < response.Metadata.Total {
		nextPage = &TempoRemotePagination{
			Limit:  page.Limit,
			Offset: page.Offset + page.Limit,
		}
	}

	return children, nextPage, nil
}

// RemoteScopes list all available scopes on the remote server
// @Summary list all available scopes on the remote server
// @Description list all available scopes on the remote server
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param groupId query string false "group ID"
// @Param pageToken query string false "page Token"
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.TempoTeam]
// @Tags plugins/tempo
// @Router /plugins/tempo/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

func searchTempoRemoteTeams(
	apiClient plugin.ApiClient,
	params *dsmodels.DsRemoteApiScopeSearchParams,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.TempoTeam],
	err errors.Error,
) {
	var queryParams url.Values
	if params.Search != "" {
		queryParams = url.Values{
			"name": {params.Search},
		}
	}

	res, err := apiClient.Get("teams", queryParams, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to get teams from Tempo API")
	}

	var response struct {
		Metadata struct {
			Count  int `json:"count"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
			Total  int `json:"total"`
		} `json:"metadata"`
		Results []models.TempoTeamResponse `json:"results"`
	}
	err = api.UnmarshalResponse(res, &response)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to unmarshal teams response")
	}

	for _, team := range response.Results {
		if params.Search == "" || strings.Contains(strings.ToLower(team.Name), strings.ToLower(params.Search)) {
			children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.TempoTeam]{
				Type:     api.RAS_ENTRY_TYPE_SCOPE,
				Id:       strconv.FormatInt(team.Id, 10),
				ParentId: nil,
				Name:     team.Name,
				FullName: team.Name,
				Data:     team.ConvertToToolLayer(0), // connectionId overridden by PutMultiple handler
			})
		}
	}

	start := (params.Page - 1) * params.PageSize
	end := start + params.PageSize
	if start >= len(children) {
		return nil, nil
	}
	if end > len(children) {
		end = len(children)
	}

	return children[start:end], nil
}

// SearchRemoteScopes searches scopes on the remote server
// @Summary searches scopes on the remote server
// @Description searches scopes on the remote server
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param search query string false "search"
// @Param page query int false "page number"
// @Param pageSize query int false "page size per page"
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.TempoTeam] "the parentIds are always null"
// @Tags plugins/tempo
// @Router /plugins/tempo/connections/{connectionId}/search-remote-scopes [GET]
func SearchRemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeSearch.Get(input)
}

// Proxy forward API requests to Tempo API
// @Summary Remote server API proxy
// @Description Forward API requests to the specified remote server
// @Param connectionId path int true "connection ID"
// @Param path path string true "path to a API endpoint"
// @Router /plugins/tempo/connections/{connectionId}/proxy/{path} [GET]
// @Tags plugins/tempo
func Proxy(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raProxy.Proxy(input)
}
