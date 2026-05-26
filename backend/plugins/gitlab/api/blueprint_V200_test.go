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

	coreModels "github.com/apache/incubator-devlake/core/models"
	mockplugin "github.com/apache/incubator-devlake/mocks/core/plugin"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/gitlab/models"
	"github.com/apache/incubator-devlake/plugins/gitlab/tasks"
	"github.com/stretchr/testify/assert"
)

func mockGitlabPlugin(t *testing.T) {
	mockMeta := mockplugin.NewPluginMeta(t)
	mockMeta.On("RootPkgPath").Return("github.com/apache/incubator-devlake/plugins/gitlab")
	mockMeta.On("Name").Return("dummy").Maybe()
	err := plugin.RegisterPlugin(pluginName, mockMeta)
	assert.Equal(t, err, nil)
}

func TestMakeScopes(t *testing.T) {
	mockGitlabPlugin(t)

	const connectionId = 1
	const gitlabProjectId = 37
	const expectDomainScopeId = "gitlab:GitlabProject:1:37"

	actualScopes, err := makeScopeV200(
		connectionId,
		[]*srvhelper.ScopeDetail[models.GitlabProject, models.GitlabScopeConfig]{
			{
				Scope: models.GitlabProject{
					Scope: common.Scope{
						ConnectionId: connectionId,
					},
					GitlabId: gitlabProjectId,
				},
				ScopeConfig: &models.GitlabScopeConfig{
					ScopeConfig: common.ScopeConfig{
						Entities: []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CICD},
					},
				},
			},
		},
	)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(actualScopes))
	assert.Equal(t, actualScopes[0].ScopeId(), expectDomainScopeId)
	assert.Equal(t, actualScopes[1].ScopeId(), expectDomainScopeId)
	assert.Equal(t, actualScopes[2].ScopeId(), expectDomainScopeId)
}

func TestMakeScopesWithEmptyEntities(t *testing.T) {
	mockGitlabPlugin(t)

	const connectionId = 1
	const gitlabProjectId = 37
	const expectDomainScopeId = "gitlab:GitlabProject:1:37"

	actualScopes, err := makeScopeV200(
		connectionId,
		[]*srvhelper.ScopeDetail[models.GitlabProject, models.GitlabScopeConfig]{
			{
				Scope: models.GitlabProject{
					Scope: common.Scope{
						ConnectionId: connectionId,
					},
					GitlabId: gitlabProjectId,
				},
				ScopeConfig: &models.GitlabScopeConfig{
					ScopeConfig: common.ScopeConfig{
						Entities: []string{},
					},
				},
			},
		},
	)
	assert.Nil(t, err)
	// empty entities should default to all domain types, producing repo + cicd + board scopes
	assert.Equal(t, 3, len(actualScopes))
	assert.Equal(t, actualScopes[0].ScopeId(), expectDomainScopeId)
}

func TestMakeScopesWithCrossEntity(t *testing.T) {
	mockGitlabPlugin(t)

	const connectionId = 1
	const gitlabProjectId = 37
	const expectDomainScopeId = "gitlab:GitlabProject:1:37"

	actualScopes, err := makeScopeV200(
		connectionId,
		[]*srvhelper.ScopeDetail[models.GitlabProject, models.GitlabScopeConfig]{
			{
				Scope: models.GitlabProject{
					Scope: common.Scope{
						ConnectionId: connectionId,
					},
					GitlabId: gitlabProjectId,
				},
				ScopeConfig: &models.GitlabScopeConfig{
					ScopeConfig: common.ScopeConfig{
						Entities: []string{plugin.DOMAIN_TYPE_CROSS, plugin.DOMAIN_TYPE_TICKET},
					},
				},
			},
		},
	)
	assert.Nil(t, err)
	// CROSS entity should trigger repo scope creation, plus ticket = board scope
	assert.Equal(t, 2, len(actualScopes))
	assert.Equal(t, actualScopes[0].ScopeId(), expectDomainScopeId)
	assert.Equal(t, "repos", actualScopes[0].TableName())
	assert.Equal(t, "boards", actualScopes[1].TableName())
}

func TestMakeDataSourcePipelinePlanV200(t *testing.T) {
	mockGitlabPlugin(t)

	const connectionID uint64 = 1
	const gitlabProjectId = 37
	const scopeConfigId uint64 = 2
	const scopeConfigName string = "gitlab scope config"
	const gitlabEndPoint string = "https://gitlab.com/api/v4/"
	const httpUrlToRepo string = "https://this_is_cloneUrl"
	const gitlabToken string = "nddtf"
	const gitlabProjectName string = "gitlab-test"
	const pathWithNamespace string = "nddtf/gitlab-test"
	const expectDomainScopeId = "gitlab:GitlabProject:1:37"

	scopeConfig := &models.GitlabScopeConfig{
		ScopeConfig: common.ScopeConfig{
			Entities: []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CICD},
		},
		PrType: "hey,man,wasup",
		Refdiff: map[string]interface{}{
			"tagsPattern": "pattern",
			"tagsLimit":   10,
			"tagsOrder":   "reverse semver",
		},
	}

	actualPlans, err := makePipelinePlanV200(
		[]plugin.SubTaskMeta{
			tasks.ConvertProjectMeta,
			tasks.CollectApiIssuesMeta,
			tasks.ExtractApiIssuesMeta,
			tasks.ConvertIssuesMeta,
			tasks.ConvertIssueLabelsMeta,
			tasks.CollectApiJobsMeta,
			tasks.ExtractApiJobsMeta,
			tasks.CollectApiPipelinesMeta,
			tasks.ExtractApiPipelinesMeta,
		},
		&models.GitlabConnection{
			BaseConnection: api.BaseConnection{
				Model: common.Model{
					ID: connectionID,
				},
			},
			GitlabConn: models.GitlabConn{
				RestConnection: api.RestConnection{},
				AccessToken: api.AccessToken{
					Token: gitlabToken,
				},
			},
		},
		[]*srvhelper.ScopeDetail[models.GitlabProject, models.GitlabScopeConfig]{
			{
				Scope: models.GitlabProject{
					GitlabId:          gitlabProjectId,
					Name:              gitlabProjectName,
					PathWithNamespace: pathWithNamespace,
					HttpUrlToRepo:     httpUrlToRepo,
					Scope: common.Scope{
						ConnectionId: connectionID,
					},
				},
				ScopeConfig: scopeConfig,
			},
		},
	)
	assert.Nil(t, err)

	var expectPlans = coreModels.PipelinePlan{
		{
			{
				Plugin: pluginName,
				Subtasks: []string{
					tasks.ConvertProjectMeta.Name,
					tasks.CollectApiIssuesMeta.Name,
					tasks.ExtractApiIssuesMeta.Name,
					tasks.ConvertIssuesMeta.Name,
					tasks.ConvertIssueLabelsMeta.Name,
					tasks.CollectApiJobsMeta.Name,
					tasks.ExtractApiJobsMeta.Name,
					tasks.CollectApiPipelinesMeta.Name,
					tasks.ExtractApiPipelinesMeta.Name,
				},
				Options: map[string]interface{}{
					"connectionId": connectionID,
					"projectId":    gitlabProjectId,
					"fullName":     pathWithNamespace,
				},
			},
			{
				Plugin: "gitextractor",
				Options: map[string]interface{}{
					"proxy":        "",
					"repoId":       expectDomainScopeId,
					"name":         gitlabProjectName,
					"fullName":     pathWithNamespace,
					"url":          "https://git:nddtf@this_is_cloneUrl",
					"connectionId": connectionID,
					"pluginName":   pluginName,
				},
			},
		},
		{
			{
				Plugin: "refdiff",
				Options: map[string]interface{}{
					"tagsLimit":   10,
					"tagsOrder":   "reverse semver",
					"tagsPattern": "pattern",
				},
			},
		},
	}

	assert.Equal(t, expectPlans, actualPlans)
}
