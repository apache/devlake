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

package models

import (
	"testing"

	"github.com/apache/devlake/core/plugin"
	"github.com/stretchr/testify/assert"
)

func TestScopeNames(t *testing.T) {
	project := AzuredevopsRepo{
		Type: RepositoryTypeProject, Name: "proj1 (project)", ProjectId: "proj1",
		OrganizationId: "org", Id: ProjectScopeID("org", "proj1"),
	}
	assert.Equal(t, "proj1 (project)", project.ScopeName())
	assert.Equal(t, "proj1", project.ProjectName())
	assert.Equal(t, project.Id, project.ProjectDomainScopeID())

	repo := AzuredevopsRepo{Type: RepositoryTypeADO, Name: "repo1", ProjectId: "proj1", OrganizationId: "org"}
	assert.Equal(t, "repo1", repo.ScopeName())
	assert.Equal(t, "proj1/repo1", repo.DomainName())
	assert.Equal(t, "org/proj1/__project__", repo.ProjectDomainScopeID())
	remote := AzuredevopsRepo{Type: RepositoryTypeGithub, Name: "github/org/repo1", ProjectId: "proj1"}
	assert.Equal(t, "github/org/repo1", remote.DomainName())
	assert.Equal(t, "org/proj2/__project__", ProjectScopeID("org", "proj2"))
	assert.Equal(t, "https://dev.azure.com/org/proj1", ProjectBoardURL("https://dev.azure.com/", "org", "proj1"))
	assert.Equal(t, "https://ado.example/proj2", ProjectBoardURL("https://ado.example", "", "proj2"))
}

func TestScopeEntities(t *testing.T) {
	assert.Equal(t, []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CODE_QUALITY}, ScopeEntities(RepositoryTypeProject, nil))
	assert.Equal(t, []string{
		plugin.DOMAIN_TYPE_CODE,
		plugin.DOMAIN_TYPE_TICKET,
		plugin.DOMAIN_TYPE_CICD,
		plugin.DOMAIN_TYPE_CODE_QUALITY,
	}, ScopeEntities(RepositoryTypeADO, []string{
		plugin.DOMAIN_TYPE_CODE,
		plugin.DOMAIN_TYPE_TICKET,
		plugin.DOMAIN_TYPE_CICD,
		plugin.DOMAIN_TYPE_CODE_QUALITY,
	}))
	assert.Equal(t, []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_CICD}, ScopeEntities(RepositoryTypeGithub, []string{
		plugin.DOMAIN_TYPE_CODE,
		plugin.DOMAIN_TYPE_CICD,
	}))
}

func TestClaimProjectEntitiesOnce(t *testing.T) {
	claimed := map[string]struct{}{}
	entities := []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CODE_QUALITY}
	first := ClaimProjectEntities(entities, "org/proj", claimed)
	second := ClaimProjectEntities(entities, "org/proj", claimed)
	other := ClaimProjectEntities(entities, "org/other", claimed)
	assert.Equal(t, entities, first)
	assert.Equal(t, []string{plugin.DOMAIN_TYPE_CODE}, second)
	assert.Equal(t, entities, other)
	assert.Equal(t, []string{plugin.DOMAIN_TYPE_CODE}, ClaimProjectEntities(entities, "", claimed))
}
