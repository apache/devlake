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

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	mockplugin "github.com/apache/incubator-devlake/mocks/core/plugin"
	"github.com/apache/incubator-devlake/plugins/linear/models"
	"github.com/stretchr/testify/assert"
)

func mockLinearPlugin(t *testing.T) {
	mockMeta := mockplugin.NewPluginMeta(t)
	mockMeta.On("RootPkgPath").Return("github.com/apache/incubator-devlake/plugins/linear")
	mockMeta.On("Name").Return("linear").Maybe()
	_ = plugin.RegisterPlugin("linear", mockMeta)
}

func TestMakeScopesV200(t *testing.T) {
	mockLinearPlugin(t)

	const connectionId uint64 = 1
	const teamId = "team-1"
	const expectDomainScopeId = "linear:LinearTeam:1:team-1"

	scopes, err := makeScopesV200(
		[]*srvhelper.ScopeDetail[models.LinearTeam, models.LinearScopeConfig]{
			{
				Scope: models.LinearTeam{
					Scope:  common.Scope{ConnectionId: connectionId},
					TeamId: teamId,
					Name:   "Engineering",
				},
				ScopeConfig: &models.LinearScopeConfig{
					ScopeConfig: common.ScopeConfig{Entities: []string{plugin.DOMAIN_TYPE_TICKET}},
				},
			},
		},
		&models.LinearConnection{
			BaseConnection: helper.BaseConnection{Model: common.Model{ID: connectionId}},
		},
	)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(scopes))
	assert.Equal(t, expectDomainScopeId, scopes[0].ScopeId())
}

func TestMakePipelinePlanV200PassesScopeConfigId(t *testing.T) {
	const scopeConfigId uint64 = 42
	subtaskMetas := []plugin.SubTaskMeta{
		{Name: "convertIssues", EnabledByDefault: true, DomainTypes: []string{plugin.DOMAIN_TYPE_TICKET}},
	}

	plan, err := makePipelinePlanV200(
		subtaskMetas,
		[]*srvhelper.ScopeDetail[models.LinearTeam, models.LinearScopeConfig]{
			{
				Scope: models.LinearTeam{
					Scope:  common.Scope{ConnectionId: 1, ScopeConfigId: scopeConfigId},
					TeamId: "team-1",
					Name:   "Engineering",
				},
				ScopeConfig: &models.LinearScopeConfig{
					ScopeConfig: common.ScopeConfig{Entities: []string{plugin.DOMAIN_TYPE_TICKET}},
				},
			},
		},
		&models.LinearConnection{
			BaseConnection: helper.BaseConnection{Model: common.Model{ID: 1}},
		},
	)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(plan))
	assert.Equal(t, 1, len(plan[0]))
	// the scope's scopeConfigId must be threaded into the task options so the
	// convertor can resolve label-based issue-type mapping at runtime.
	assert.EqualValues(t, scopeConfigId, plan[0][0].Options["scopeConfigId"])
}

func TestMakeScopesV200WithoutTicketEntity(t *testing.T) {
	mockLinearPlugin(t)

	scopes, err := makeScopesV200(
		[]*srvhelper.ScopeDetail[models.LinearTeam, models.LinearScopeConfig]{
			{
				Scope: models.LinearTeam{
					Scope:  common.Scope{ConnectionId: 1},
					TeamId: "team-1",
				},
				ScopeConfig: &models.LinearScopeConfig{
					ScopeConfig: common.ScopeConfig{Entities: []string{}},
				},
			},
		},
		&models.LinearConnection{
			BaseConnection: helper.BaseConnection{Model: common.Model{ID: 1}},
		},
	)
	assert.Nil(t, err)
	// no ticket entity selected => no domain board scope produced
	assert.Equal(t, 0, len(scopes))
}
