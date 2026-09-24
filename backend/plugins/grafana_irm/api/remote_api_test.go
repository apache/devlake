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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
)

func TestListGrafanaIrmRemoteScopes_ReturnsExactlyOneScopeNoNextPage(t *testing.T) {
	children, nextPage, err := listGrafanaIrmRemoteScopes(nil, nil, "", GrafanaIrmRemotePagination{})
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, "default", children[0].Id)
	assert.Nil(t, nextPage)
}

func TestSearchGrafanaIrmRemoteScopes_EmptySearchMatches(t *testing.T) {
	children, err := searchGrafanaIrmRemoteScopes(nil, &dsmodels.DsRemoteApiScopeSearchParams{})
	require.NoError(t, err)
	require.Len(t, children, 1)
}

func TestSearchGrafanaIrmRemoteScopes_MatchingSearchTermMatches(t *testing.T) {
	children, err := searchGrafanaIrmRemoteScopes(nil, &dsmodels.DsRemoteApiScopeSearchParams{Search: "incidents"})
	require.NoError(t, err)
	require.Len(t, children, 1)
}

func TestSearchGrafanaIrmRemoteScopes_NonMatchingSearchTermExcludes(t *testing.T) {
	children, err := searchGrafanaIrmRemoteScopes(nil, &dsmodels.DsRemoteApiScopeSearchParams{Search: "nonexistent"})
	require.NoError(t, err)
	assert.Len(t, children, 0)
}
