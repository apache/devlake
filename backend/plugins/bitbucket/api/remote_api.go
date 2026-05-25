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
	"io"
	"net/http"
	"net/url"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/incubator-devlake/helpers/pluginhelper/api/models"
	"github.com/apache/incubator-devlake/plugins/bitbucket/models"
)

type BitbucketRemotePagination struct {
	Page    int `json:"page" validate:"required"`
	PageLen int `json:"pagelen" validate:"required"`
}

func listBitbucketRemoteScopes(
	connection *models.BitbucketConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page BitbucketRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.BitbucketRepo],
	nextPage *BitbucketRemotePagination,
	err errors.Error,
) {
	if page.Page == 0 {
		page.Page = 1
	}
	if page.PageLen == 0 {
		page.PageLen = 100
	}

	if groupId == "" {
		return listBitbucketWorkspaces(apiClient, page)
	}
	return listBitbucketRepos(apiClient, groupId, page)
}

func listBitbucketWorkspaces(
	apiClient plugin.ApiClient,
	page BitbucketRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.BitbucketRepo],
	nextPage *BitbucketRemotePagination,
	err errors.Error,
) {
	var res *http.Response
	res, err = apiClient.Get(
		"/user/permissions/workspaces",
		url.Values{
			"sort":    {"workspace.slug"},
			"fields":  {"values.workspace.slug,values.workspace.name,pagelen,page,size"},
			"page":    {fmt.Sprintf("%v", page.Page)},
			"pagelen": {fmt.Sprintf("%v", page.PageLen)},
		},
		nil,
	)
	if err != nil {
		return
	}
	if res.StatusCode > 299 {
		body, e := io.ReadAll(res.Body)
		if e != nil {
			err = errors.BadInput.Wrap(e, "failed to read response body")
			return
		}
		err = errors.HttpStatus(res.StatusCode).New(string(body))
		return
	}

	resBody := &models.WorkspaceResponse{}
	err = api.UnmarshalResponse(res, resBody)
	if err != nil {
		return
	}
	for _, r := range resBody.Values {
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.BitbucketRepo]{
			Type:     api.RAS_ENTRY_TYPE_GROUP,
			Id:       r.Workspace.Slug,
			Name:     r.Workspace.Name,
			FullName: r.Workspace.Name,
		})
	}
	return
}

func listBitbucketRepos(
	apiClient plugin.ApiClient,
	workspace string,
	page BitbucketRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.BitbucketRepo],
	nextPage *BitbucketRemotePagination,
	err errors.Error,
) {

	var res *http.Response
	// list projects part
	res, err = apiClient.Get(fmt.Sprintf("/repositories/%s", workspace), url.Values{
		"fields":  {"values.name,values.full_name,values.language,values.description,values.owner.display_name,values.created_on,values.updated_on,values.links.clone,values.links.html,pagelen,page,size"},
		"page":    {fmt.Sprintf("%v", page.Page)},
		"pagelen": {fmt.Sprintf("%v", page.PageLen)},
	}, nil)
	if err != nil {
		return
	}
	if res.StatusCode > 299 {
		body, e := io.ReadAll(res.Body)
		if e != nil {
			return nil, nil, errors.BadInput.Wrap(e, "failed to read response body")
		}
		return nil, nil, errors.BadInput.New(string(body))
	}
	var resBody models.ReposResponse
	err = api.UnmarshalResponse(res, &resBody)
	if err != nil {
		return
	}

	for _, r := range resBody.Values {
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.BitbucketRepo]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			Id:       r.FullName,
			ParentId: &workspace,
			Name:     r.Name,
			FullName: r.FullName,
			Data:     r.ConvertApiScope(),
		})
	}
	return
}

func searchBitbucketRepos(
	apiClient plugin.ApiClient,
	params *dsmodels.DsRemoteApiScopeSearchParams,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.BitbucketRepo],
	err errors.Error,
) {
	var res *http.Response
	res, err = apiClient.Get(
		"/repositories",
		url.Values{
			"sort":    {"name"},
			"fields":  {"values.name,values.full_name,values.language,values.description,values.owner.display_name,values.created_on,values.updated_on,values.links.clone,values.links.html,pagelen,page,size"},
			"role":    {"member"},
			"q":       {fmt.Sprintf(`full_name~"%s"`, params.Search)},
			"page":    {fmt.Sprintf("%v", params.Page)},
			"pagelen": {fmt.Sprintf("%v", params.PageSize)},
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	var resBody models.ReposResponse
	err = api.UnmarshalResponse(res, &resBody)
	if err != nil {
		return
	}
	for _, r := range resBody.Values {
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.BitbucketRepo]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			Id:       r.FullName,
			Name:     r.Name,
			FullName: r.FullName,
			Data:     r.ConvertApiScope(),
		})
	}
	return
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
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.BitbucketRepo]
// @Tags plugins/bitbucket
// @Router /plugins/bitbucket/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
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
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.BitbucketRepo] "the parentIds are always null"
// @Tags plugins/bitbucket
// @Router /plugins/bitbucket/connections/{connectionId}/search-remote-scopes [GET]
func SearchRemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeSearch.Get(input)
}

// @Summary Remote server API proxy
// @Description Forward API requests to the specified remote server
// @Param connectionId path int true "connection ID"
// @Param path path string true "path to a API endpoint"
// @Tags plugins/bitbucket
// @Router /plugins/bitbucket/connections/{connectionId}/proxy/{path} [GET]
func Proxy(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raProxy.Proxy(input)
}
