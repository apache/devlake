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
	"fmt"
	"net/url"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/incubator-devlake/helpers/pluginhelper/api/models"
	"github.com/apache/incubator-devlake/plugins/sonarqube/models"
)

type SonarqubeRemotePagination struct {
	Page     int `json:"p"`
	PageSize int `json:"ps"`
}

// sonarqubeSearchProjectsQueryFilter builds the "filter" query value for
// api/components/search_projects. SonarQube expects text search via the filter
// language (e.g. filter=query = "term"), not a bare "q" parameter; see
// SearchProjectsAction in SonarQube.
func sonarqubeSearchProjectsQueryFilter(term string) string {
	term = strings.TrimSpace(term)
	term = strings.ReplaceAll(term, `"`, "")
	return fmt.Sprintf(`query = "%s"`, term)
}

func querySonarqubeProjects(
	apiClient plugin.ApiClient,
	keyword string,
	page SonarqubeRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.SonarqubeProject],
	nextPage *SonarqubeRemotePagination,
	err errors.Error,
) {
	if page.PageSize == 0 {
		page.PageSize = 100
	}
	if page.Page == 0 {
		page.Page = 1
	}
	// Use components/search_projects so non-admin (Browse) tokens can list projects.
	q := url.Values{
		"p":  {fmt.Sprintf("%v", page.Page)},
		"ps": {fmt.Sprintf("%v", page.PageSize)},
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		q.Set("filter", sonarqubeSearchProjectsQueryFilter(keyword))
	}
	res, err := apiClient.Get("components/search_projects", q, nil)
	if err != nil {
		return
	}

	resBody := struct {
		Paging struct {
			PageIndex int `json:"pageIndex"`
			PageSize  int `json:"pageSize"`
			Total     int `json:"total"`
		} `json:"paging"`
		Components []*models.SonarqubeApiProject
	}{}

	err = api.UnmarshalResponse(res, &resBody)
	if err != nil {
		return
	}

	for _, project := range resBody.Components {
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.SonarqubeProject]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			Id:       fmt.Sprintf("%v", project.ProjectKey),
			ParentId: nil,
			Name:     project.Name,
			FullName: project.Name,
			Data:     project.ConvertApiScope(),
		})
	}

	if resBody.Paging.Total > resBody.Paging.PageIndex*resBody.Paging.PageSize {
		nextPage = &SonarqubeRemotePagination{
			Page:     resBody.Paging.PageIndex + 1,
			PageSize: resBody.Paging.PageSize,
		}
	}

	return
}

func listSonarqubeRemoteScopes(
	connection *models.SonarqubeConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page SonarqubeRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.SonarqubeProject],
	nextPage *SonarqubeRemotePagination,
	err errors.Error,
) {
	return querySonarqubeProjects(apiClient, "", page)
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
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.SonarqubeProject]
// @Tags plugins/sonarqube
// @Router /plugins/sonarqube/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

func searchSonarqubeRemoteProjects(
	apiClient plugin.ApiClient,
	params *dsmodels.DsRemoteApiScopeSearchParams,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.SonarqubeProject],
	err errors.Error,
) {
	if params.Page == 0 {
		params.Page = 1
	}
	page := SonarqubeRemotePagination{
		Page:     params.Page,
		PageSize: params.PageSize,
	}
	children, _, err = querySonarqubeProjects(apiClient, params.Search, page)
	return
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
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.SonarqubeProject] "the parentIds are always null"
// @Tags plugins/sonarqube
// @Router /plugins/sonarqube/connections/{connectionId}/search-remote-scopes [GET]
func SearchRemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeSearch.Get(input)
}

// @Summary Remote server API proxy
// @Description Forward API requests to the specified remote server
// @Param connectionId path int true "connection ID"
// @Param path path string true "path to a API endpoint"
// @Tags plugins/sonarqube
// @Router /plugins/sonarqube/connections/{connectionId}/proxy/{path} [GET]
func Proxy(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raProxy.Proxy(input)
}
