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
	"encoding/base64"
	"encoding/json"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api/models"
)

const (
	RAS_ENTRY_TYPE_GROUP = "group"
	RAS_ENTRY_TYPE_SCOPE = "scope"
)

// DsListRemoteScopes is the function type for listing remote scopes that must be implemented by the plugin
type DsListRemoteScopes[C plugin.ToolLayerApiConnection, S plugin.ToolLayerScope, P any] func(
	connection *C, apiClient plugin.ApiClient, groupId string, page P) (children []models.DsRemoteApiScopeListEntry[S], nextPage *P, errr errors.Error)

// DsRemoteApiScopeListHelper is a helper to list scopes page by page on remote servers
// P is the page info type
type DsRemoteApiScopeListHelper[C plugin.ToolLayerApiConnection, S plugin.ToolLayerScope, P any] struct {
	*DsRemoteApiProxyHelper[C]
	listRemoteScopes DsListRemoteScopes[C, S, P]
}

// NewDsRemoteApiScopeListHelper creates a new DsRemoteApiScopeListHelper
func NewDsRemoteApiScopeListHelper[
	C plugin.ToolLayerApiConnection,
	S plugin.ToolLayerScope,
	P any,
](
	rap *DsRemoteApiProxyHelper[C],
	listRemoteScopes DsListRemoteScopes[C, S, P],
) *DsRemoteApiScopeListHelper[C, S, P] {
	return &DsRemoteApiScopeListHelper[C, S, P]{
		DsRemoteApiProxyHelper: rap,
		listRemoteScopes:       listRemoteScopes,
	}
}

// Get returns scopes on the data source
func (rsl *DsRemoteApiScopeListHelper[C, S, P]) Get(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, apiClient, err := rsl.prepare(input)
	if err != nil {
		return nil, err
	}
	groupId := input.Query.Get("groupId")
	pageInfo := new(P)
	pageToken := input.Query.Get("pageToken")
	// decode page token, we use pageToken because the pagination strategy varies from plugin to plugin
	// some may use `page` and `size` while some may adopt `offset` and `limit`, even some may use `cursor`
	if pageToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(pageToken)
		if err != nil {
			return nil, errors.BadInput.Wrap(err, "failed to decode page token")
		}
		errors.Must(json.Unmarshal(decoded, pageInfo))
	}
	children, nextPage, err := rsl.listRemoteScopes(connection, apiClient, groupId, *pageInfo)
	if err != nil {
		return nil, err
	}
	// construct next page token
	nextPageToken := ""
	if nextPage != nil {
		nextPageJson := errors.Must1(json.Marshal(nextPage))
		nextPageToken = base64.StdEncoding.EncodeToString(nextPageJson)
	}
	if children == nil {
		children = []models.DsRemoteApiScopeListEntry[S]{}
	}
	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"children":      children,
			"nextPageToken": nextPageToken,
		},
	}, nil
}
