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

	mockplugin "github.com/apache/incubator-devlake/mocks/core/plugin"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/bitbucket/models"
	"github.com/stretchr/testify/assert"
)

func mockBitbucketPlugin(t *testing.T) {
	mockMeta := mockplugin.NewPluginMeta(t)
	mockMeta.On("RootPkgPath").Return("github.com/apache/incubator-devlake/plugins/bitbucket")
	mockMeta.On("Name").Return("dummy").Maybe()
	err := plugin.RegisterPlugin("bitbucket", mockMeta)
	assert.Equal(t, err, nil)
}

func TestMakeScopes(t *testing.T) {
	mockBitbucketPlugin(t)

	const connectionId uint64 = 1
	const bitbucketId = "owner/repo"
	const expectDomainScopeId = "bitbucket:BitbucketRepo:1:owner/repo"

	actualScopes, err := makeScopesV200(
		[]*srvhelper.ScopeDetail[models.BitbucketRepo, models.BitbucketScopeConfig]{
			{
				Scope: models.BitbucketRepo{
					Scope: common.Scope{
						ConnectionId: connectionId,
					},
					BitbucketId: bitbucketId,
				},
				ScopeConfig: &models.BitbucketScopeConfig{
					ScopeConfig: common.ScopeConfig{
						Entities: []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CICD},
					},
				},
			},
		},
		&models.BitbucketConnection{
			BaseConnection: api.BaseConnection{
				Model: common.Model{
					ID: connectionId,
				},
			},
		},
	)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(actualScopes))
	assert.Equal(t, expectDomainScopeId, actualScopes[0].ScopeId())
	assert.Equal(t, expectDomainScopeId, actualScopes[1].ScopeId())
	assert.Equal(t, expectDomainScopeId, actualScopes[2].ScopeId())
}

func TestMakeScopesWithEmptyEntities(t *testing.T) {
	mockBitbucketPlugin(t)

	const connectionId uint64 = 1
	const bitbucketId = "owner/repo"
	const expectDomainScopeId = "bitbucket:BitbucketRepo:1:owner/repo"

	actualScopes, err := makeScopesV200(
		[]*srvhelper.ScopeDetail[models.BitbucketRepo, models.BitbucketScopeConfig]{
			{
				Scope: models.BitbucketRepo{
					Scope: common.Scope{
						ConnectionId: connectionId,
					},
					BitbucketId: bitbucketId,
				},
				ScopeConfig: &models.BitbucketScopeConfig{
					ScopeConfig: common.ScopeConfig{
						Entities: []string{},
					},
				},
			},
		},
		&models.BitbucketConnection{
			BaseConnection: api.BaseConnection{
				Model: common.Model{
					ID: connectionId,
				},
			},
		},
	)
	assert.Nil(t, err)
	// empty entities should default to all domain types, producing repo + cicd + board scopes
	assert.Equal(t, 3, len(actualScopes))
	assert.Equal(t, expectDomainScopeId, actualScopes[0].ScopeId())
}

func TestMakeScopesWithCrossEntity(t *testing.T) {
	mockBitbucketPlugin(t)

	const connectionId uint64 = 1
	const bitbucketId = "owner/repo"
	const expectDomainScopeId = "bitbucket:BitbucketRepo:1:owner/repo"

	actualScopes, err := makeScopesV200(
		[]*srvhelper.ScopeDetail[models.BitbucketRepo, models.BitbucketScopeConfig]{
			{
				Scope: models.BitbucketRepo{
					Scope: common.Scope{
						ConnectionId: connectionId,
					},
					BitbucketId: bitbucketId,
				},
				ScopeConfig: &models.BitbucketScopeConfig{
					ScopeConfig: common.ScopeConfig{
						Entities: []string{plugin.DOMAIN_TYPE_CROSS, plugin.DOMAIN_TYPE_TICKET},
					},
				},
			},
		},
		&models.BitbucketConnection{
			BaseConnection: api.BaseConnection{
				Model: common.Model{
					ID: connectionId,
				},
			},
		},
	)
	assert.Nil(t, err)
	// CROSS entity should trigger repo scope creation, plus ticket = board scope
	assert.Equal(t, 2, len(actualScopes))
	assert.Equal(t, expectDomainScopeId, actualScopes[0].ScopeId())
	assert.Equal(t, "repos", actualScopes[0].TableName())
	assert.Equal(t, "boards", actualScopes[1].TableName())
}
