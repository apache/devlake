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
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/helpers/utils"
)

type DsSearchRemoteScopes[C plugin.ToolLayerApiConnection, S plugin.ToolLayerScope] func(
	apiClient plugin.ApiClient, params *models.DsRemoteApiScopeSearchParams) (children []models.DsRemoteApiScopeListEntry[S], err errors.Error)

// DsSearchRemoteScopesWithCount is like DsSearchRemoteScopes but additionally returns the total
// number of matching scopes reported by the remote server. A negative total means "unknown".
type DsSearchRemoteScopesWithCount[C plugin.ToolLayerApiConnection, S plugin.ToolLayerScope] func(
	apiClient plugin.ApiClient, params *models.DsRemoteApiScopeSearchParams) (children []models.DsRemoteApiScopeListEntry[S], total int, err errors.Error)

type DsRemoteApiScopeSearchHelper[C plugin.ToolLayerApiConnection, S plugin.ToolLayerScope] struct {
	*DsRemoteApiProxyHelper[C]
	searchRemoteScopes DsSearchRemoteScopesWithCount[C, S]
}

func NewDsRemoteApiScopeSearchHelper[
	C plugin.ToolLayerApiConnection,
	S plugin.ToolLayerScope,
](
	rap *DsRemoteApiProxyHelper[C],
	searchRemoteScopes DsSearchRemoteScopes[C, S],
) *DsRemoteApiScopeSearchHelper[C, S] {
	var withCount DsSearchRemoteScopesWithCount[C, S]
	if searchRemoteScopes != nil {
		withCount = func(apiClient plugin.ApiClient, params *models.DsRemoteApiScopeSearchParams) ([]models.DsRemoteApiScopeListEntry[S], int, errors.Error) {
			children, err := searchRemoteScopes(apiClient, params)
			return children, -1, err
		}
	}
	return NewDsRemoteApiScopeSearchHelperWithCount[C, S](rap, withCount)
}

// NewDsRemoteApiScopeSearchHelperWithCount creates a search helper whose response additionally
// contains the total number of matching scopes ("count"), if the search function reports it.
func NewDsRemoteApiScopeSearchHelperWithCount[
	C plugin.ToolLayerApiConnection,
	S plugin.ToolLayerScope,
](
	rap *DsRemoteApiProxyHelper[C],
	searchRemoteScopes DsSearchRemoteScopesWithCount[C, S],
) *DsRemoteApiScopeSearchHelper[C, S] {
	return &DsRemoteApiScopeSearchHelper[C, S]{
		DsRemoteApiProxyHelper: rap,
		searchRemoteScopes:     searchRemoteScopes,
	}
}

// SearchRemoteScopes searches remote scopes
func (rss *DsRemoteApiScopeSearchHelper[C, S]) Get(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	_, apiClient, err := rss.prepare(input)
	if err != nil {
		return nil, err
	}
	params := &models.DsRemoteApiScopeSearchParams{
		Page:     1,
		PageSize: 50,
	}
	err = utils.DecodeMapStruct(input.Query, params, true)
	if err != nil {
		return nil, err
	}
	if e := vld.Struct(params); e != nil {
		return nil, errors.BadInput.Wrap(e, "invalid params")
	}
	children, total, err := rss.searchRemoteScopes(apiClient, params)
	if err != nil {
		return nil, err
	}
	return &plugin.ApiResourceOutput{
		Body: buildDsRemoteApiScopeSearchBody(children, total, params),
	}, nil
}

func buildDsRemoteApiScopeSearchBody[S plugin.ToolLayerScope](
	children []models.DsRemoteApiScopeListEntry[S],
	total int,
	params *models.DsRemoteApiScopeSearchParams,
) map[string]interface{} {
	if children == nil {
		children = []models.DsRemoteApiScopeListEntry[S]{}
	}
	// the config-ui is expecting the parent id to be null
	for i := range children {
		children[i].ParentId = nil
	}
	body := map[string]interface{}{
		"children": children,
		"page":     params.Page,
		"pageSize": params.PageSize,
	}
	// only expose the total if the plugin knows it, so the config-ui can fall back to its heuristic
	if total >= 0 {
		body["count"] = total
	}
	return body
}
