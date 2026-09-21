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

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/srvhelper"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/stretchr/testify/assert"
)

func mockYoutrackPlugin(t *testing.T) {
	mockMeta := mockplugin.NewPluginMeta(t)
	mockMeta.On("RootPkgPath").Return("github.com/apache/devlake/plugins/youtrack")
	mockMeta.On("Name").Return("youtrack").Maybe()
	_ = plugin.RegisterPlugin("youtrack", mockMeta)
}

func TestMakeScopesV200(t *testing.T) {
	mockYoutrackPlugin(t)

	const connectionId uint64 = 1
	const projectId = "0-12"
	const expectDomainScopeId = "youtrack:YoutrackProject:1:0-12"

	scopes, err := makeScopesV200(
		[]*srvhelper.ScopeDetail[models.YoutrackProject, models.YoutrackScopeConfig]{
			{
				Scope: models.YoutrackProject{
					Scope:     common.Scope{ConnectionId: connectionId},
					Id:        projectId,
					ShortName: "PROJ2",
					Name:      "Project 2",
				},
				ScopeConfig: &models.YoutrackScopeConfig{
					ScopeConfig: common.ScopeConfig{Entities: []string{plugin.DOMAIN_TYPE_TICKET}},
				},
			},
		},
		&models.YoutrackConnection{
			BaseConnection: helper.BaseConnection{Model: common.Model{ID: connectionId}},
		},
	)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(scopes))
	// the domain board id is built from the internal project id, so it
	// survives a shortName rename
	assert.Equal(t, expectDomainScopeId, scopes[0].ScopeId())
	// the board is named after the project, not the cryptic internal id
	assert.Equal(t, "Project 2", scopes[0].ScopeName())
}

func TestMakeScopesV200WithoutTicketEntity(t *testing.T) {
	mockYoutrackPlugin(t)

	scopes, err := makeScopesV200(
		[]*srvhelper.ScopeDetail[models.YoutrackProject, models.YoutrackScopeConfig]{
			{
				Scope: models.YoutrackProject{
					Scope: common.Scope{ConnectionId: 1},
					Id:    "0-12",
					Name:  "Project 2",
				},
				ScopeConfig: &models.YoutrackScopeConfig{
					ScopeConfig: common.ScopeConfig{Entities: []string{plugin.DOMAIN_TYPE_CROSS}},
				},
			},
		},
		&models.YoutrackConnection{
			BaseConnection: helper.BaseConnection{Model: common.Model{ID: 1}},
		},
	)
	assert.Nil(t, err)
	// no ticket entity selected => no domain board scope produced
	assert.Equal(t, 0, len(scopes))
}

func TestMakePipelinePlanV200PassesScopeParams(t *testing.T) {
	const scopeConfigId uint64 = 42
	subtaskMetas := []plugin.SubTaskMeta{
		{Name: "convertIssues", EnabledByDefault: true, DomainTypes: []string{plugin.DOMAIN_TYPE_TICKET}},
	}

	plan, err := makePipelinePlanV200(
		subtaskMetas,
		[]*srvhelper.ScopeDetail[models.YoutrackProject, models.YoutrackScopeConfig]{
			{
				Scope: models.YoutrackProject{
					Scope:     common.Scope{ConnectionId: 1, ScopeConfigId: scopeConfigId},
					Id:        "0-12",
					ShortName: "PROJ2",
					Name:      "Project 2",
				},
				ScopeConfig: &models.YoutrackScopeConfig{
					ScopeConfig: common.ScopeConfig{Entities: []string{plugin.DOMAIN_TYPE_TICKET}},
				},
			},
		},
		&models.YoutrackConnection{
			BaseConnection: helper.BaseConnection{Model: common.Model{ID: 1}},
		},
	)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(plan))
	assert.Equal(t, 1, len(plan[0]))
	// the scope's project id and scope config must be threaded into the task
	// options so PrepareTaskData can scope collection and resolve the
	// field-name slots and mappings at runtime
	assert.Equal(t, "0-12", plan[0][0].Options["projectId"])
	assert.EqualValues(t, scopeConfigId, plan[0][0].Options["scopeConfigId"])
	assert.EqualValues(t, 1, plan[0][0].Options["connectionId"])
	// the generated subtask list reflects the entity selection
	assert.Contains(t, plan[0][0].Subtasks, "convertIssues")
}
