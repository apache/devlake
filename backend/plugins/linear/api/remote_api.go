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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/incubator-devlake/helpers/pluginhelper/api/models"
	"github.com/apache/incubator-devlake/plugins/linear/models"
)

// LinearRemotePagination drives cursor-based pagination through the GraphQL
// `teams` connection when listing remote scopes for the config UI.
type LinearRemotePagination struct {
	Cursor string `json:"cursor"`
}

// linearTeamsGraphqlResponse mirrors the shape of the `teams` query response.
type linearTeamsGraphqlResponse struct {
	Data struct {
		Teams struct {
			Nodes []struct {
				Id          string `json:"id"`
				Name        string `json:"name"`
				Key         string `json:"key"`
				Description string `json:"description"`
			} `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"teams"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

const remoteScopesPageSize = 100

// listLinearRemoteScopes lists Linear teams as selectable scopes. Linear teams
// are a flat list, so there are no intermediate groups.
func listLinearRemoteScopes(
	_ *models.LinearConnection,
	apiClient plugin.ApiClient,
	_ string,
	page LinearRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.LinearTeam],
	nextPage *LinearRemotePagination,
	err errors.Error,
) {
	after := ""
	if page.Cursor != "" {
		after = fmt.Sprintf(", after: %q", page.Cursor)
	}
	query := fmt.Sprintf(
		"query { teams(first: %d%s) { nodes { id name key description } pageInfo { hasNextPage endCursor } } }",
		remoteScopesPageSize, after,
	)

	res, err := apiClient.Post("", nil, map[string]interface{}{"query": query}, nil)
	if err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to query Linear teams")
	}
	var response linearTeamsGraphqlResponse
	if err := api.UnmarshalResponse(res, &response); err != nil {
		return nil, nil, errors.Default.Wrap(err, "failed to unmarshal Linear teams response")
	}
	if len(response.Errors) > 0 {
		return nil, nil, errors.Default.New("linear graphql teams query failed: " + response.Errors[0].Message)
	}

	return mapLinearTeamsToScopeEntries(response), nextPageFrom(response), nil
}

// mapLinearTeamsToScopeEntries converts a teams response into scope-list
// entries. Each team is a selectable (leaf) scope.
func mapLinearTeamsToScopeEntries(response linearTeamsGraphqlResponse) []dsmodels.DsRemoteApiScopeListEntry[models.LinearTeam] {
	children := make([]dsmodels.DsRemoteApiScopeListEntry[models.LinearTeam], 0, len(response.Data.Teams.Nodes))
	for _, team := range response.Data.Teams.Nodes {
		team := team
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.LinearTeam]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			ParentId: nil,
			Id:       team.Id,
			Name:     team.Name,
			FullName: team.Name,
			Data: &models.LinearTeam{
				TeamId:      team.Id,
				Name:        team.Name,
				Key:         team.Key,
				Description: team.Description,
			},
		})
	}
	return children
}

// nextPageFrom returns the cursor for the following page, or nil when the
// teams connection has been fully traversed.
func nextPageFrom(response linearTeamsGraphqlResponse) *LinearRemotePagination {
	pageInfo := response.Data.Teams.PageInfo
	if pageInfo.HasNextPage && pageInfo.EndCursor != "" {
		return &LinearRemotePagination{Cursor: pageInfo.EndCursor}
	}
	return nil
}

// RemoteScopes lists the Linear teams available on the connection so the
// config UI can enumerate selectable scopes.
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

// Proxy forwards arbitrary requests to the Linear API through the connection.
func Proxy(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raProxy.Proxy(input)
}
