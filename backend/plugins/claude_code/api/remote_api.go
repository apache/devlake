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
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helperapi "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/incubator-devlake/helpers/pluginhelper/api/models"
	"github.com/apache/incubator-devlake/helpers/utils"
	"github.com/apache/incubator-devlake/plugins/claude_code/models"
)

// ClaudeCodeRemotePagination is a placeholder for remote scope pagination.
// Claude Code currently returns a single organization scope per connection.
type ClaudeCodeRemotePagination struct {
	Page int `json:"page"`
}

func listClaudeCodeRemoteScopes(
	connection *models.ClaudeCodeConnection,
	_ plugin.ApiClient,
	_ string,
	_ ClaudeCodeRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.ClaudeCodeScope],
	nextPage *ClaudeCodeRemotePagination,
	err errors.Error,
) {
	if connection == nil {
		return nil, nil, errors.BadInput.New("connection is required")
	}

	organizationId := strings.TrimSpace(connection.Organization)
	if organizationId == "" {
		return []dsmodels.DsRemoteApiScopeListEntry[models.ClaudeCodeScope]{}, nil, nil
	}

	children = append(children, makeClaudeCodeRemoteScopeEntry(organizationId))
	return children, nil, nil
}

func makeClaudeCodeRemoteScopeEntry(organizationId string) dsmodels.DsRemoteApiScopeListEntry[models.ClaudeCodeScope] {
	organizationId = strings.TrimSpace(organizationId)
	return dsmodels.DsRemoteApiScopeListEntry[models.ClaudeCodeScope]{
		Type:     helperapi.RAS_ENTRY_TYPE_SCOPE,
		Id:       organizationId,
		Name:     organizationId,
		FullName: organizationId,
		Data: &models.ClaudeCodeScope{
			Id:           organizationId,
			Organization: organizationId,
			Name:         organizationId,
			FullName:     organizationId,
		},
	}
}

// RemoteScopes lists all available scopes for this connection.
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

// SearchRemoteScopes searches scopes for this connection.
func SearchRemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.ClaudeCodeConnection{}
	if err := connectionHelper.First(connection, input.Params); err != nil {
		return nil, err
	}

	params := &dsmodels.DsRemoteApiScopeSearchParams{
		Page:     1,
		PageSize: 50,
	}
	if err := utils.DecodeMapStruct(input.Query, params, true); err != nil {
		return nil, err
	}
	if err := errors.Convert(vld.Struct(params)); err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid params")
	}

	children := []dsmodels.DsRemoteApiScopeListEntry[models.ClaudeCodeScope]{}
	organizationId := strings.TrimSpace(connection.Organization)
	searchLower := strings.ToLower(strings.TrimSpace(params.Search))
	if organizationId != "" && strings.Contains(strings.ToLower(organizationId), searchLower) {
		children = append(children, makeClaudeCodeRemoteScopeEntry(organizationId))
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"children": children,
			"page":     params.Page,
			"pageSize": params.PageSize,
		},
	}, nil
}
