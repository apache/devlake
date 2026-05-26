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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/incubator-devlake/helpers/pluginhelper/api/models"
	"github.com/apache/incubator-devlake/plugins/taiga/models"
)

type TaigaRemotePagination struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type TaigaApiProject struct {
	Id          uint64 `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func queryTaigaProjects(
	apiClient plugin.ApiClient,
	keyword string,
	page TaigaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.TaigaProject],
	nextPage *TaigaRemotePagination,
	err errors.Error,
) {
	if page.PageSize == 0 {
		page.PageSize = 100
	}
	if page.Page == 0 {
		page.Page = 1
	}

	query := url.Values{
		"page":      {fmt.Sprintf("%d", page.Page)},
		"page_size": {fmt.Sprintf("%d", page.PageSize)},
	}
	if keyword != "" {
		query.Set("search", keyword)
	}

	res, err := apiClient.Get("projects", query, nil)
	if err != nil {
		return
	}

	var projects []TaigaApiProject
	err = api.UnmarshalResponse(res, &projects)
	if err != nil {
		return
	}

	for _, project := range projects {
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.TaigaProject]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			Id:       fmt.Sprintf("%d", project.Id),
			ParentId: nil,
			Name:     project.Name,
			FullName: project.Name,
			Data: &models.TaigaProject{
				ProjectId:   project.Id,
				Name:        project.Name,
				Slug:        project.Slug,
				Description: project.Description,
			},
		})
	}

	// Check if there are more pages
	if len(projects) == page.PageSize {
		nextPage = &TaigaRemotePagination{
			Page:     page.Page + 1,
			PageSize: page.PageSize,
		}
	}

	return
}

func listTaigaRemoteScopes(
	_ *models.TaigaConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page TaigaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.TaigaProject],
	nextPage *TaigaRemotePagination,
	err errors.Error,
) {
	return queryTaigaProjects(apiClient, "", page)
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
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.TaigaProject]
// @Tags plugins/taiga
// @Router /plugins/taiga/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}
