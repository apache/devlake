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
	"reflect"
	"strings"
	"testing"

	coreModels "github.com/apache/incubator-devlake/core/models"
	mockplugin "github.com/apache/incubator-devlake/mocks/core/plugin"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/azuredevops_go/models"
	"github.com/apache/incubator-devlake/plugins/azuredevops_go/tasks"
	"github.com/stretchr/testify/assert"
)

const (
	connectionID        uint64 = 1
	azuredevopsRepoId          = "ad05901f-c9b0-4938-bc8a-a22eb2467ceb"
	expectDomainScopeId        = "azuredevops_go:AzuredevopsRepo:1:ad05901f-c9b0-4938-bc8a-a22eb2467ceb"
)

func mockAzuredevopsPlugin(t *testing.T) {
	mockMeta := mockplugin.NewPluginMeta(t)
	mockMeta.On("RootPkgPath").Return("github.com/apache/incubator-devlake/plugins/azuredevops_go")
	mockMeta.On("Name").Return("dummy").Maybe()
	err := plugin.RegisterPlugin("azuredevops_go", mockMeta)
	assert.Equal(t, err, nil)
}

func TestMakeScopes(t *testing.T) {
	mockAzuredevopsPlugin(t)

	actualScopes, err := makeScopeV200(
		connectionID,
		[]*srvhelper.ScopeDetail[models.AzuredevopsRepo, models.AzuredevopsScopeConfig]{
			{
				Scope: models.AzuredevopsRepo{
					Scope: common.Scope{
						ConnectionId: connectionID,
					},
					Id:   azuredevopsRepoId,
					Type: models.RepositoryTypeADO,
				},
				ScopeConfig: &models.AzuredevopsScopeConfig{
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
	mockAzuredevopsPlugin(t)

	actualScopes, err := makeScopeV200(
		connectionID,
		[]*srvhelper.ScopeDetail[models.AzuredevopsRepo, models.AzuredevopsScopeConfig]{
			{
				Scope: models.AzuredevopsRepo{
					Scope: common.Scope{
						ConnectionId: connectionID,
					},
					Id:   azuredevopsRepoId,
					Type: models.RepositoryTypeADO,
				},
				ScopeConfig: &models.AzuredevopsScopeConfig{
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
	mockAzuredevopsPlugin(t)

	actualScopes, err := makeScopeV200(
		connectionID,
		[]*srvhelper.ScopeDetail[models.AzuredevopsRepo, models.AzuredevopsScopeConfig]{
			{
				Scope: models.AzuredevopsRepo{
					Scope: common.Scope{
						ConnectionId: connectionID,
					},
					Id:   azuredevopsRepoId,
					Type: models.RepositoryTypeADO,
				},
				ScopeConfig: &models.AzuredevopsScopeConfig{
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
	mockAzuredevopsPlugin(t)

	const (
		httpUrlToRepo          = "https://this_is_cloneUrl"
		azureDevOpsToken       = "personal-access-token"
		azureDevOpsProjectName = "azuredevops-test-project"
		azureDevOpsOrgName     = "azuredevops-test-org"
	)

	actualPlans, err := makePipelinePlanV200(
		[]plugin.SubTaskMeta{
			tasks.CollectApiPullRequestsMeta,
			tasks.ExtractApiPullRequestsMeta,
			tasks.CollectBuildsMeta,
			tasks.ExtractApiBuildsMeta,
		},
		&models.AzuredevopsConnection{
			BaseConnection: api.BaseConnection{
				Model: common.Model{
					ID: connectionID,
				},
			},
			AzuredevopsConn: models.AzuredevopsConn{
				AzuredevopsAccessToken: models.AzuredevopsAccessToken{
					Token: azureDevOpsToken,
				},
			},
		},
		[]*srvhelper.ScopeDetail[models.AzuredevopsRepo, models.AzuredevopsScopeConfig]{
			{
				Scope: models.AzuredevopsRepo{
					Id: fmt.Sprint(azuredevopsRepoId),
					AzureDevOpsPK: models.AzureDevOpsPK{
						ProjectId:      azureDevOpsProjectName,
						OrganizationId: azureDevOpsOrgName,
					},
					Name:      azureDevOpsProjectName,
					Url:       httpUrlToRepo,
					RemoteUrl: httpUrlToRepo,
					Type:      models.RepositoryTypeADO,
				},
				ScopeConfig: &models.AzuredevopsScopeConfig{
					ScopeConfig: common.ScopeConfig{
						Entities: []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_CODE_REVIEW, plugin.DOMAIN_TYPE_CICD},
					},
					DeploymentPattern: "(?i)deploy",
					ProductionPattern: "(?i)prod",
					Refdiff: map[string]interface{}{
						"tagsPattern": "pattern",
						"tagsLimit":   10,
						"tagsOrder":   "reverse semver",
					},
				},
			},
		},
	)
	assert.Nil(t, err)

	var expectPlans = coreModels.PipelinePlan{
		{
			{
				Plugin: "azuredevops_go",
				Subtasks: []string{
					tasks.CollectApiPullRequestsMeta.Name,
					tasks.ExtractApiPullRequestsMeta.Name,
					tasks.CollectBuildsMeta.Name,
					tasks.ExtractApiBuildsMeta.Name,
				},
				Options: map[string]interface{}{
					"name":           azureDevOpsProjectName,
					"connectionId":   connectionID,
					"projectId":      azureDevOpsProjectName,
					"repositoryId":   fmt.Sprint(azuredevopsRepoId),
					"organizationId": azureDevOpsOrgName,
					"repositoryType": models.RepositoryTypeADO,
					"externalId":     "",
				},
			},
			{
				Plugin: "gitextractor",
				Options: map[string]interface{}{
					"proxy":          "",
					"repoId":         expectDomainScopeId,
					"name":           azureDevOpsProjectName,
					"url":            "https://git:personal-access-token@this_is_cloneUrl",
					"noShallowClone": true,
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

func TestMakeRemoteRepoScopes(t *testing.T) {
	mockAzuredevopsPlugin(t)

	data := []struct {
		Name           string
		Type           string
		Private        bool
		ExpectedScopes []string
	}{
		{Name: "Azure DevOps Repository", Type: models.RepositoryTypeADO, Private: false, ExpectedScopes: []string{"*code.Repo", "*ticket.Board", "*devops.CicdScope"}},

		{Name: "Public GitHub Repository", Type: models.RepositoryTypeGithub, Private: false, ExpectedScopes: []string{"*code.Repo", "*devops.CicdScope"}},

		{Name: "Private GitHub Repository", Type: models.RepositoryTypeGithub, Private: true, ExpectedScopes: []string{"*devops.CicdScope"}},
	}

	for _, d := range data {

		t.Run(d.Name, func(t *testing.T) {
			id := strings.ToLower(d.Name)
			id = strings.ReplaceAll(id, " ", "-")
			actualScopes, err := makeScopeV200(
				connectionID,
				[]*srvhelper.ScopeDetail[models.AzuredevopsRepo, models.AzuredevopsScopeConfig]{
					{
						Scope: models.AzuredevopsRepo{
							Scope: common.Scope{
								ConnectionId: connectionID,
							},
							Id:        id,
							Type:      d.Type,
							Name:      d.Name,
							IsPrivate: d.Private,
						},
						ScopeConfig: &models.AzuredevopsScopeConfig{
							ScopeConfig: common.ScopeConfig{
								Entities: []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_TICKET,
									plugin.DOMAIN_TYPE_CICD, plugin.DOMAIN_TYPE_CODE_REVIEW},
							},
						},
					},
				},
			)
			assert.Nil(t, err)
			var count int

			for _, s := range actualScopes {
				xType := reflect.TypeOf(s)
				assert.Contains(t, d.ExpectedScopes, xType.String())
				count++
			}
			assert.Equal(t, count, len(d.ExpectedScopes))
		})

	}
}
