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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/stretchr/testify/assert"
)

type searchHelperTestScope = TestFakeGithubRepo

type searchHelperTestConnection struct{}

func (searchHelperTestConnection) TableName() string        { return "_tool_test_connections" }
func (searchHelperTestConnection) ConnectionId() uint64     { return 1 }
func (searchHelperTestConnection) GetEndpoint() string      { return "" }
func (searchHelperTestConnection) GetProxy() string         { return "" }
func (searchHelperTestConnection) GetRateLimitPerHour() int { return 0 }

var _ plugin.ToolLayerApiConnection = searchHelperTestConnection{}
var _ plugin.ToolLayerScope = searchHelperTestScope{}

func TestBuildDsRemoteApiScopeSearchBody(t *testing.T) {
	params := &models.DsRemoteApiScopeSearchParams{Search: "x", Page: 2, PageSize: 10}
	parent := "parent"
	children := []models.DsRemoteApiScopeListEntry[searchHelperTestScope]{{Id: "1", ParentId: &parent}}

	body := buildDsRemoteApiScopeSearchBody(children, 42, params)
	assert.Equal(t, 42, body["count"])
	assert.Equal(t, 2, body["page"])
	assert.Equal(t, 10, body["pageSize"])
	assert.Nil(t, body["children"].([]models.DsRemoteApiScopeListEntry[searchHelperTestScope])[0].ParentId)

	body = buildDsRemoteApiScopeSearchBody[searchHelperTestScope](nil, -1, params)
	_, hasCount := body["count"]
	assert.False(t, hasCount)
	assert.NotNil(t, body["children"])
	assert.Len(t, body["children"], 0)
}

func TestNewDsRemoteApiScopeSearchHelperWrapsLegacyFunction(t *testing.T) {
	legacy := func(_ plugin.ApiClient, _ *models.DsRemoteApiScopeSearchParams) ([]models.DsRemoteApiScopeListEntry[searchHelperTestScope], errors.Error) {
		return []models.DsRemoteApiScopeListEntry[searchHelperTestScope]{{Id: "1"}}, nil
	}
	helper := NewDsRemoteApiScopeSearchHelper[searchHelperTestConnection, searchHelperTestScope](nil, legacy)

	children, total, err := helper.searchRemoteScopes(nil, &models.DsRemoteApiScopeSearchParams{})
	assert.Nil(t, err)
	assert.Len(t, children, 1)
	assert.Equal(t, -1, total)

	assert.Nil(t, NewDsRemoteApiScopeSearchHelper[searchHelperTestConnection, searchHelperTestScope](nil, nil).searchRemoteScopes)
}
