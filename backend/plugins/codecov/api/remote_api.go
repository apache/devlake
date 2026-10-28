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
	gocontext "context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/plugins/codecov/models"
)

type CodecovRemotePagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type codecovRepo struct {
	Name        string `json:"name"`
	Service     string `json:"service"`
	Language    string `json:"language"`
	Active      bool   `json:"active"`
	ActivatedAt string `json:"activated_at"`
	Updatestamp string `json:"updatestamp"`
	Private     bool   `json:"private"`
	Branch      string `json:"branch"`
	Repository  struct {
		Name string `json:"name"`
	} `json:"repository"`
}

type codecovReposResponse struct {
	Results []codecovRepo `json:"results"`
	Next    *string       `json:"next,omitempty"`
}

func listCodecovRemoteScopes(
	connection *models.CodecovConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page CodecovRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo],
	nextPage *CodecovRemotePagination,
	err errors.Error,
) {
	if page.Page == 0 {
		page.Page = 1
	}
	if page.PerPage == 0 {
		page.PerPage = 100
	}

	owner := connection.Organization
	if groupId != "" {
		owner = groupId
	}

	query := url.Values{
		"page":      []string{fmt.Sprintf("%v", page.Page)},
		"page_size": []string{fmt.Sprintf("%v", page.PerPage)},
	}

	service := connection.ServiceOrDefault()
	reposUrl := fmt.Sprintf("/api/v2/%s/%s/repos/", service, owner)
	reposBody, err := apiClient.Get(reposUrl, query, nil)
	if err != nil {
		return nil, nil, err
	}

	if reposBody.StatusCode == http.StatusNotFound {
		_ = reposBody.Body.Close()
		return nil, nil, errors.HttpStatus(http.StatusNotFound).New(fmt.Sprintf("Organization or owner '%s' not found", owner))
	}

	if reposBody.StatusCode != http.StatusOK {
		_ = reposBody.Body.Close()
		return nil, nil, errors.HttpStatus(reposBody.StatusCode).New("unexpected status code while fetching repositories")
	}

	var reposResponse codecovReposResponse
	err = api.UnmarshalResponse(reposBody, &reposResponse)
	if err != nil {
		return nil, nil, err
	}

	for _, repo := range reposResponse.Results {
		// Get repo name - it might be in repo.Name or repo.Repository.Name
		repoName := repo.Name
		if repoName == "" && repo.Repository.Name != "" {
			repoName = repo.Repository.Name
		}
		if repoName == "" {
			continue
		}

		fullName := fmt.Sprintf("%s/%s", owner, repoName)
		codecovId := fmt.Sprintf("%s/%s", owner, repoName)

		branch := repo.Branch
		if branch == "" {
			branch = "main"
		}

		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			ParentId: nil,
			Id:       codecovId,
			Name:     repoName,
			FullName: fullName,
			Data: &models.CodecovRepo{
				CodecovId:   codecovId,
				Name:        repoName,
				FullName:    fullName,
				Service:     repo.Service,
				Language:    repo.Language,
				Active:      repo.Active,
				ActivatedAt: repo.ActivatedAt,
				Updatestamp: repo.Updatestamp,
				Private:     repo.Private,
				Branch:      branch,
			},
		})
	}

	// Check if there's a next page
	if reposResponse.Next != nil && *reposResponse.Next != "" {
		nextPage = &CodecovRemotePagination{
			Page:    page.Page + 1,
			PerPage: page.PerPage,
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
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.CodecovRepo]
// @Tags plugins/codecov
// @Router /plugins/codecov/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

// SearchRemoteScopes searches scopes on the remote server.
// Bypasses the standard search helper because Codecov's repos endpoint
// is scoped to an organization (needs connection.Organization), and the
// framework callback only receives apiClient + params.
// @Summary searches scopes on the remote server
// @Description searches scopes on the remote server
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param search query string false "search"
// @Param page query int false "page number"
// @Param pageSize query int false "page size per page"
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.CodecovRepo] "the parentIds are always null"
// @Tags plugins/codecov
// @Router /plugins/codecov/connections/{connectionId}/search-remote-scopes [GET]
func SearchRemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.ModelApiHelper.FindByPk(input)
	if err != nil {
		return nil, err
	}

	apiClient, err := api.NewApiClientFromConnection(gocontext.TODO(), basicRes, connection)
	if err != nil {
		return nil, err
	}

	search := input.Query.Get("search")
	if search == "" {
		return &plugin.ApiResourceOutput{
			Body: map[string]interface{}{
				"children": []dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo]{},
				"page":     1,
				"pageSize": 50,
			},
		}, nil
	}

	page := 1
	pageSize := 50
	if v, e := strconv.Atoi(input.Query.Get("page")); e == nil && v > 0 {
		page = v
	}
	if v, e := strconv.Atoi(input.Query.Get("pageSize")); e == nil && v > 0 {
		pageSize = v
	}

	service := connection.ServiceOrDefault()
	org := connection.Organization

	// For GitLab services, search supports subgroup paths:
	//   "subgroup"          → list repos under org:subgroup
	//   "subgroup/repo"     → search "repo" under org:subgroup
	//   "sub/nested/repo"   → search "repo" under org:sub:nested
	// If no "/" is present, also try the search as a plain repo name search.
	var children []dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo]
	isGitLabService := strings.HasPrefix(service, "gitlab")

	if isGitLabService && strings.Contains(search, "/") {
		// Split on last "/" — left is subgroup path, right is repo search
		lastSlash := strings.LastIndex(search, "/")
		subgroupPath := search[:lastSlash]
		repoSearch := search[lastSlash+1:]
		subgroupOwner := org + ":" + strings.ReplaceAll(subgroupPath, "/", ":")

		children, err = searchReposForOwner(apiClient, service, subgroupOwner, repoSearch, page, pageSize)
		if err != nil {
			return nil, err
		}
	} else {
		// Standard search: search repos at the org level
		children, err = searchReposForOwner(apiClient, service, org, search, page, pageSize)
		if err != nil {
			return nil, err
		}

		// For GitLab: if no results, try interpreting search as a subgroup name
		if isGitLabService && len(children) == 0 {
			subgroupOwner := org + ":" + strings.ReplaceAll(search, "/", ":")
			subgroupChildren, subErr := searchReposForOwner(apiClient, service, subgroupOwner, "", page, pageSize)
			if subErr == nil && len(subgroupChildren) > 0 {
				children = subgroupChildren
			}
		}
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"children": children,
			"page":     page,
			"pageSize": pageSize,
		},
	}, nil
}

// searchReposForOwner queries the Codecov API for repos belonging to owner,
// optionally filtered by a search term.
func searchReposForOwner(
	apiClient plugin.ApiClient,
	service, owner, search string,
	page, pageSize int,
) ([]dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo], errors.Error) {
	query := url.Values{
		"page":      []string{fmt.Sprintf("%v", page)},
		"page_size": []string{fmt.Sprintf("%v", pageSize)},
	}
	if search != "" {
		query.Set("search", search)
	}

	reposUrl := fmt.Sprintf("/api/v2/%s/%s/repos/", service, owner)
	res, err := apiClient.Get(reposUrl, query, nil)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusNotFound {
		_ = res.Body.Close()
		return []dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo]{}, nil
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, errors.HttpStatus(res.StatusCode).New("unexpected status code while searching repositories")
	}

	var reposResponse codecovReposResponse
	err = api.UnmarshalResponse(res, &reposResponse)
	if err != nil {
		return nil, err
	}

	children := make([]dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo], 0, len(reposResponse.Results))
	for _, repo := range reposResponse.Results {
		repoName := repo.Name
		if repoName == "" && repo.Repository.Name != "" {
			repoName = repo.Repository.Name
		}
		if repoName == "" {
			continue
		}

		fullName := fmt.Sprintf("%s/%s", owner, repoName)
		codecovId := fullName

		branch := repo.Branch
		if branch == "" {
			branch = "main"
		}

		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			ParentId: nil,
			Id:       codecovId,
			Name:     repoName,
			FullName: fullName,
			Data: &models.CodecovRepo{
				CodecovId:   codecovId,
				Name:        repoName,
				FullName:    fullName,
				Service:     repo.Service,
				Language:    repo.Language,
				Active:      repo.Active,
				ActivatedAt: repo.ActivatedAt,
				Updatestamp: repo.Updatestamp,
				Private:     repo.Private,
				Branch:      branch,
			},
		})
	}

	return children, nil
}
