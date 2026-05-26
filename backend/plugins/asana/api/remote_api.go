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
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

type AsanaRemotePagination struct {
	Offset string `json:"offset"`
	Limit  int    `json:"limit"`
}

// Response types for Asana API
type asanaWorkspaceResponse struct {
	Gid            string `json:"gid"`
	Name           string `json:"name"`
	ResourceType   string `json:"resource_type"`
	IsOrganization bool   `json:"is_organization"`
}

type asanaWorkspacesListResponse struct {
	Data     []asanaWorkspaceResponse `json:"data"`
	NextPage *asanaNextPage           `json:"next_page"`
}

type asanaNextPage struct {
	Offset string `json:"offset"`
	Path   string `json:"path"`
	URI    string `json:"uri"`
}

type asanaTeamResponse struct {
	Gid          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
	Description  string `json:"description"`
	PermalinkUrl string `json:"permalink_url"`
}

type asanaTeamsListResponse struct {
	Data     []asanaTeamResponse `json:"data"`
	NextPage *asanaNextPage      `json:"next_page"`
}

type asanaPortfolioResponse struct {
	Gid          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
	PermalinkUrl string `json:"permalink_url"`
}

type asanaPortfoliosListResponse struct {
	Data     []asanaPortfolioResponse `json:"data"`
	NextPage *asanaNextPage           `json:"next_page"`
}

type asanaGoalResponse struct {
	Gid          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
	Notes        string `json:"notes"`
}

type asanaGoalsListResponse struct {
	Data     []asanaGoalResponse `json:"data"`
	NextPage *asanaNextPage      `json:"next_page"`
}

type asanaProjectResponse struct {
	Gid          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
	Archived     bool   `json:"archived"`
	PermalinkUrl string `json:"permalink_url"`
	Workspace    *struct {
		Gid string `json:"gid"`
	} `json:"workspace"`
	Team *struct {
		Gid  string `json:"gid"`
		Name string `json:"name"`
	} `json:"team"`
}

type asanaProjectsListResponse struct {
	Data     []asanaProjectResponse `json:"data"`
	NextPage *asanaNextPage         `json:"next_page"`
}

// Scope type constants
const (
	ScopeTypeTeam      = "team"
	ScopeTypePortfolio = "portfolio"
	ScopeTypeGoal      = "goal"
)

func listAsanaRemoteScopes(
	connection *models.AsanaConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page AsanaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject],
	nextPage *AsanaRemotePagination,
	err errors.Error,
) {
	if page.Limit == 0 {
		page.Limit = 100
	}

	// Level 1: No groupId - list workspaces
	if groupId == "" {
		return listAsanaWorkspaces(apiClient, page)
	}

	// Parse hierarchical groupId
	// Format examples:
	// - "workspace/{gid}" -> show Teams, Portfolios, Goals options
	// - "workspace/{gid}/team/{teamGid}" -> list projects in team
	// - "workspace/{gid}/portfolio/{portfolioGid}" -> list projects in portfolio
	// - "workspace/{gid}/goal/{goalGid}" -> list projects for goal

	if strings.HasPrefix(groupId, "workspace/") {
		parts := strings.Split(groupId[10:], "/") // Remove "workspace/" prefix

		if len(parts) == 1 {
			// Level 2: Workspace selected - show Teams, Portfolios, Goals as categories
			workspaceGid := parts[0]
			return listAsanaScopeCategories(apiClient, workspaceGid, page)
		}

		if len(parts) >= 3 {
			workspaceGid := parts[0]
			scopeType := parts[1]
			scopeGid := parts[2]

			switch scopeType {
			case ScopeTypeTeam:
				// Level 4: List projects in team
				return listAsanaTeamProjects(apiClient, workspaceGid, scopeGid, page)
			case ScopeTypePortfolio:
				// Level 4: List projects in portfolio
				return listAsanaPortfolioProjects(apiClient, workspaceGid, scopeGid, page)
			case ScopeTypeGoal:
				// Level 4: List projects for goal
				return listAsanaGoalProjects(apiClient, workspaceGid, scopeGid, page)
			}
		}

		if len(parts) == 2 {
			workspaceGid := parts[0]
			scopeType := parts[1]

			switch scopeType {
			case ScopeTypeTeam:
				// Level 3: List all teams
				return listAsanaTeams(apiClient, workspaceGid, page)
			case ScopeTypePortfolio:
				// Level 3: List all portfolios
				return listAsanaPortfolios(apiClient, workspaceGid, page)
			case ScopeTypeGoal:
				// Level 3: List all goals
				return listAsanaGoals(apiClient, workspaceGid, page)
			}
		}
	}

	return nil, nil, errors.BadInput.New("invalid groupId format")
}

// Level 1: List workspaces
func listAsanaWorkspaces(
	apiClient plugin.ApiClient,
	page AsanaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject],
	nextPage *AsanaRemotePagination,
	err errors.Error,
) {
	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", page.Limit))
	query.Set("opt_fields", "name,resource_type,is_organization")
	if page.Offset != "" {
		query.Set("offset", page.Offset)
	}

	res, err := apiClient.Get("workspaces", query, nil)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to fetch workspaces from Asana API")
	}

	var response asanaWorkspacesListResponse
	err = api.UnmarshalResponse(res, &response)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal Asana workspaces response")
	}

	for _, workspace := range response.Data {
		groupId := fmt.Sprintf("workspace/%s", workspace.Gid)
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject]{
			Type:     api.RAS_ENTRY_TYPE_GROUP,
			ParentId: nil, // Root level, no parent
			Id:       groupId,
			Name:     workspace.Name,
			FullName: workspace.Name,
		})
	}

	if response.NextPage != nil && response.NextPage.Offset != "" {
		nextPage = &AsanaRemotePagination{
			Offset: response.NextPage.Offset,
			Limit:  page.Limit,
		}
	}

	return children, nextPage, nil
}

// Level 2: Show scope categories (Teams, Portfolios, Goals)
func listAsanaScopeCategories(
	apiClient plugin.ApiClient,
	workspaceGid string,
	page AsanaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject],
	nextPage *AsanaRemotePagination,
	err errors.Error,
) {
	// Parent is the workspace
	parentId := fmt.Sprintf("workspace/%s", workspaceGid)

	// Return the three main categories: Teams, Portfolios, Goals
	children = []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject]{
		{
			Type:     api.RAS_ENTRY_TYPE_GROUP,
			ParentId: &parentId,
			Id:       fmt.Sprintf("workspace/%s/%s", workspaceGid, ScopeTypeTeam),
			Name:     "🏢 Teams",
			FullName: "Teams",
		},
		{
			Type:     api.RAS_ENTRY_TYPE_GROUP,
			ParentId: &parentId,
			Id:       fmt.Sprintf("workspace/%s/%s", workspaceGid, ScopeTypePortfolio),
			Name:     "📁 Portfolios",
			FullName: "Portfolios",
		},
		{
			Type:     api.RAS_ENTRY_TYPE_GROUP,
			ParentId: &parentId,
			Id:       fmt.Sprintf("workspace/%s/%s", workspaceGid, ScopeTypeGoal),
			Name:     "🎯 Goals",
			FullName: "Goals",
		},
	}

	return children, nil, nil
}

// Level 3: List teams in workspace
func listAsanaTeams(
	apiClient plugin.ApiClient,
	workspaceGid string,
	page AsanaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject],
	nextPage *AsanaRemotePagination,
	err errors.Error,
) {
	// Parent is the "Teams" category
	parentId := fmt.Sprintf("workspace/%s/%s", workspaceGid, ScopeTypeTeam)

	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", page.Limit))
	query.Set("opt_fields", "name,resource_type,description,permalink_url")
	if page.Offset != "" {
		query.Set("offset", page.Offset)
	}

	apiPath := fmt.Sprintf("workspaces/%s/teams", workspaceGid)
	res, err := apiClient.Get(apiPath, query, nil)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to fetch teams from Asana API")
	}

	var response asanaTeamsListResponse
	err = api.UnmarshalResponse(res, &response)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal Asana teams response")
	}

	for _, team := range response.Data {
		groupId := fmt.Sprintf("workspace/%s/%s/%s", workspaceGid, ScopeTypeTeam, team.Gid)
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject]{
			Type:     api.RAS_ENTRY_TYPE_GROUP,
			ParentId: &parentId,
			Id:       groupId,
			Name:     team.Name,
			FullName: team.Name,
		})
	}

	if response.NextPage != nil && response.NextPage.Offset != "" {
		nextPage = &AsanaRemotePagination{
			Offset: response.NextPage.Offset,
			Limit:  page.Limit,
		}
	}

	return children, nextPage, nil
}

// Level 3: List portfolios in workspace
func listAsanaPortfolios(
	apiClient plugin.ApiClient,
	workspaceGid string,
	page AsanaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject],
	nextPage *AsanaRemotePagination,
	err errors.Error,
) {
	// Parent is the "Portfolios" category
	parentId := fmt.Sprintf("workspace/%s/%s", workspaceGid, ScopeTypePortfolio)

	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", page.Limit))
	query.Set("workspace", workspaceGid)
	query.Set("owner", "me")
	query.Set("opt_fields", "name,resource_type,permalink_url")
	if page.Offset != "" {
		query.Set("offset", page.Offset)
	}

	res, err := apiClient.Get("portfolios", query, nil)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to fetch portfolios from Asana API")
	}

	var response asanaPortfoliosListResponse
	err = api.UnmarshalResponse(res, &response)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal Asana portfolios response")
	}

	for _, portfolio := range response.Data {
		groupId := fmt.Sprintf("workspace/%s/%s/%s", workspaceGid, ScopeTypePortfolio, portfolio.Gid)
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject]{
			Type:     api.RAS_ENTRY_TYPE_GROUP,
			ParentId: &parentId,
			Id:       groupId,
			Name:     portfolio.Name,
			FullName: portfolio.Name,
		})
	}

	if response.NextPage != nil && response.NextPage.Offset != "" {
		nextPage = &AsanaRemotePagination{
			Offset: response.NextPage.Offset,
			Limit:  page.Limit,
		}
	}

	return children, nextPage, nil
}

// Level 3: List goals in workspace
func listAsanaGoals(
	apiClient plugin.ApiClient,
	workspaceGid string,
	page AsanaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject],
	nextPage *AsanaRemotePagination,
	err errors.Error,
) {
	// Parent is the "Goals" category
	parentId := fmt.Sprintf("workspace/%s/%s", workspaceGid, ScopeTypeGoal)

	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", page.Limit))
	query.Set("workspace", workspaceGid)
	query.Set("opt_fields", "name,resource_type,notes")
	if page.Offset != "" {
		query.Set("offset", page.Offset)
	}

	res, err := apiClient.Get("goals", query, nil)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to fetch goals from Asana API")
	}

	var response asanaGoalsListResponse
	err = api.UnmarshalResponse(res, &response)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal Asana goals response")
	}

	for _, goal := range response.Data {
		groupId := fmt.Sprintf("workspace/%s/%s/%s", workspaceGid, ScopeTypeGoal, goal.Gid)
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject]{
			Type:     api.RAS_ENTRY_TYPE_GROUP,
			ParentId: &parentId,
			Id:       groupId,
			Name:     goal.Name,
			FullName: goal.Name,
		})
	}

	if response.NextPage != nil && response.NextPage.Offset != "" {
		nextPage = &AsanaRemotePagination{
			Offset: response.NextPage.Offset,
			Limit:  page.Limit,
		}
	}

	return children, nextPage, nil
}

// Level 4: List projects in a team
func listAsanaTeamProjects(
	apiClient plugin.ApiClient,
	workspaceGid string,
	teamGid string,
	page AsanaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject],
	nextPage *AsanaRemotePagination,
	err errors.Error,
) {
	// Parent is the specific team
	parentId := fmt.Sprintf("workspace/%s/%s/%s", workspaceGid, ScopeTypeTeam, teamGid)

	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", page.Limit))
	query.Set("opt_fields", "name,resource_type,archived,permalink_url,workspace,team")
	if page.Offset != "" {
		query.Set("offset", page.Offset)
	}

	apiPath := fmt.Sprintf("teams/%s/projects", teamGid)
	res, err := apiClient.Get(apiPath, query, nil)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to fetch team projects from Asana API")
	}

	var response asanaProjectsListResponse
	err = api.UnmarshalResponse(res, &response)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal Asana team projects response")
	}

	children = convertProjectsToScopes(response.Data, workspaceGid, &parentId)

	if response.NextPage != nil && response.NextPage.Offset != "" {
		nextPage = &AsanaRemotePagination{
			Offset: response.NextPage.Offset,
			Limit:  page.Limit,
		}
	}

	return children, nextPage, nil
}

// Level 4: List projects in a portfolio
func listAsanaPortfolioProjects(
	apiClient plugin.ApiClient,
	workspaceGid string,
	portfolioGid string,
	page AsanaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject],
	nextPage *AsanaRemotePagination,
	err errors.Error,
) {
	// Parent is the specific portfolio
	parentId := fmt.Sprintf("workspace/%s/%s/%s", workspaceGid, ScopeTypePortfolio, portfolioGid)

	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", page.Limit))
	query.Set("opt_fields", "name,resource_type,archived,permalink_url,workspace,team")
	if page.Offset != "" {
		query.Set("offset", page.Offset)
	}

	apiPath := fmt.Sprintf("portfolios/%s/items", portfolioGid)
	res, err := apiClient.Get(apiPath, query, nil)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to fetch portfolio items from Asana API")
	}

	var response asanaProjectsListResponse
	err = api.UnmarshalResponse(res, &response)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal Asana portfolio items response")
	}

	children = convertProjectsToScopes(response.Data, workspaceGid, &parentId)

	if response.NextPage != nil && response.NextPage.Offset != "" {
		nextPage = &AsanaRemotePagination{
			Offset: response.NextPage.Offset,
			Limit:  page.Limit,
		}
	}

	return children, nextPage, nil
}

// Level 4: List projects associated with a goal
func listAsanaGoalProjects(
	apiClient plugin.ApiClient,
	workspaceGid string,
	goalGid string,
	page AsanaRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject],
	nextPage *AsanaRemotePagination,
	err errors.Error,
) {
	// Parent is the specific goal
	parentId := fmt.Sprintf("workspace/%s/%s/%s", workspaceGid, ScopeTypeGoal, goalGid)

	query := url.Values{}
	query.Set("limit", fmt.Sprintf("%d", page.Limit))
	query.Set("opt_fields", "name,resource_type,archived,permalink_url,workspace,team")
	if page.Offset != "" {
		query.Set("offset", page.Offset)
	}

	// Goals API: GET /goals/{goal_gid}/parentGoals for related projects
	// Note: Asana's goal-to-project relationship is through supporting work
	apiPath := fmt.Sprintf("goals/%s/supportingWork", goalGid)
	res, err := apiClient.Get(apiPath, query, nil)
	if err != nil {
		// If supporting work API fails, return empty list
		return children, nil, nil
	}

	var response asanaProjectsListResponse
	err = api.UnmarshalResponse(res, &response)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal Asana goal supporting work response")
	}

	children = convertProjectsToScopes(response.Data, workspaceGid, &parentId)

	if response.NextPage != nil && response.NextPage.Offset != "" {
		nextPage = &AsanaRemotePagination{
			Offset: response.NextPage.Offset,
			Limit:  page.Limit,
		}
	}

	return children, nextPage, nil
}

// Helper function to convert Asana projects to scope entries
func convertProjectsToScopes(
	projects []asanaProjectResponse,
	workspaceGid string,
	parentId *string,
) []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject] {
	var children []dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject]

	for _, project := range projects {
		workspaceGidVal := workspaceGid
		if project.Workspace != nil {
			workspaceGidVal = project.Workspace.Gid
		}
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.AsanaProject]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			ParentId: parentId,
			Id:       project.Gid,
			Name:     project.Name,
			FullName: project.Name,
			Data: &models.AsanaProject{
				Gid:          project.Gid,
				Name:         project.Name,
				ResourceType: project.ResourceType,
				Archived:     project.Archived,
				PermalinkUrl: project.PermalinkUrl,
				WorkspaceGid: workspaceGidVal,
			},
		})
	}

	return children
}

// RemoteScopes list all available scopes (projects) for this connection
// @Summary list all available scopes (projects) for this connection
// @Description list all available scopes (projects) for this connection
// @Tags plugins/asana
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param groupId query string false "group ID"
// @Param pageToken query string false "page Token"
// @Success 200  {object} RemoteScopesOutput
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/asana/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

func Proxy(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raProxy.Proxy(input)
}
