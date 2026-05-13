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
	"net/url"
	"strconv"
	"time"

	"github.com/apache/incubator-devlake/core/models/common"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/incubator-devlake/helpers/pluginhelper/api/models"
	"github.com/apache/incubator-devlake/plugins/rootly/models"
)

type RootlyRemotePagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type ServiceResponse struct {
	Data []struct {
		Id         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Name        string     `json:"name"`
			Slug        string     `json:"slug"`
			Description *string    `json:"description"`
			HtmlUrl     *string    `json:"html_url"`
			CreatedAt   *time.Time `json:"created_at"`
		} `json:"attributes"`
	} `json:"data"`
	Meta struct {
		TotalCount  int `json:"total_count"`
		TotalPages  int `json:"total_pages"`
		CurrentPage int `json:"current_page"`
	} `json:"meta"`
}

func queryRootlyRemoteScopes(
	apiClient plugin.ApiClient,
	_ string,
	page RootlyRemotePagination,
	search string,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.Service],
	nextPage *RootlyRemotePagination,
	err errors.Error,
) {
	if page.PerPage == 0 {
		page.PerPage = 50
	}
	if page.Page == 0 {
		page.Page = 1
	}
	query := url.Values{
		"page[number]": {strconv.Itoa(page.Page)},
		"page[size]":   {strconv.Itoa(page.PerPage)},
	}
	if search != "" {
		query.Set("filter[search]", search)
	}
	var res *http.Response
	res, err = apiClient.Get("services", query, nil)
	if err != nil {
		return
	}
	response := &ServiceResponse{}
	err = api.UnmarshalResponse(res, response)
	if err != nil {
		return
	}
	for _, item := range response.Data {
		htmlUrl := ""
		if item.Attributes.HtmlUrl != nil {
			htmlUrl = *item.Attributes.HtmlUrl
		}
		entry := dsmodels.DsRemoteApiScopeListEntry[models.Service]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			Id:       item.Id,
			Name:     item.Attributes.Name,
			FullName: item.Attributes.Name,
			Data: &models.Service{
				Url:  htmlUrl,
				Id:   item.Id,
				Name: item.Attributes.Name,
				Scope: common.Scope{
					NoPKModel: common.NoPKModel{},
				},
			},
		}
		if item.Attributes.CreatedAt != nil {
			entry.Data.Scope.NoPKModel.CreatedAt = *item.Attributes.CreatedAt
		}
		children = append(children, entry)
	}

	if page.Page < response.Meta.TotalPages {
		nextPage = &RootlyRemotePagination{
			Page:    page.Page + 1,
			PerPage: page.PerPage,
		}
	}

	return
}

func listRootlyRemoteScopes(
	connection *models.RootlyConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page RootlyRemotePagination,
) (
	[]dsmodels.DsRemoteApiScopeListEntry[models.Service],
	*RootlyRemotePagination,
	errors.Error,
) {
	return queryRootlyRemoteScopes(apiClient, groupId, page, "")
}

func searchRootlyRemoteScopes(
	apiClient plugin.ApiClient,
	params *dsmodels.DsRemoteApiScopeSearchParams,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.Service],
	err errors.Error,
) {
	page := params.Page
	if page == 0 {
		page = 1
	}
	children, _, err = queryRootlyRemoteScopes(apiClient, "", RootlyRemotePagination{
		Page:    page,
		PerPage: params.PageSize,
	}, params.Search)
	return
}

// RemoteScopes list all available scopes (services) for this connection
// @Summary list all available scopes (services) for this connection
// @Description list all available scopes (services) for this connection
// @Tags plugins/rootly
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param groupId query string false "group ID"
// @Param pageToken query string false "page Token"
// @Success 200  {object} RemoteScopesOutput
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/rootly/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

// SearchRemoteScopes use the Search API and only return project
// @Summary use the Search API and only return project
// @Description use the Search API and only return project
// @Tags plugins/rootly
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param search query string false "search"
// @Param page query int false "page number"
// @Param pageSize query int false "page size per page"
// @Success 200  {object} SearchRemoteScopesOutput
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/rootly/connections/{connectionId}/search-remote-scopes [GET]
func SearchRemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeSearch.Get(input)
}
