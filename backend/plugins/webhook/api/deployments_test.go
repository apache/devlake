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
	coremodels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/apache/devlake/plugins/webhook/models"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockProjectWebhookDb sets up the webhook api package with a mocked database and
// with project "demo", whose blueprint has id 3. It returns the ids of the
// blueprints that were reloaded.
func mockProjectWebhookDb(t *testing.T, setupDal func(mockDal *mockdal.Dal)) *[]uint64 {
	res := unithelper.DummyBasicRes(setupDal)
	reloaded := &[]uint64{}
	prevBasicRes, prevLogger, prevVld, prevConnectionHelper := basicRes, logger, vld, connectionHelper
	prevGetProject, prevGetBlueprint, prevReloadBlueprint := getProject, getBlueprint, reloadBlueprint
	basicRes = res
	logger = res.GetLogger()
	vld = validator.New()
	connectionHelper = api.NewConnectionHelper(res, vld, pluginName)
	blueprint := &coremodels.Blueprint{ProjectName: "demo"}
	blueprint.ID = 3
	getProject = func(name string) (*coremodels.ApiOutputProject, errors.Error) {
		return &coremodels.ApiOutputProject{Blueprint: blueprint}, nil
	}
	getBlueprint = func(blueprintId uint64, shouldSanitize bool) (*coremodels.Blueprint, errors.Error) {
		return blueprint, nil
	}
	reloadBlueprint = func(blueprintId uint64) errors.Error {
		*reloaded = append(*reloaded, blueprintId)
		return nil
	}
	t.Cleanup(func() {
		basicRes, logger, vld, connectionHelper = prevBasicRes, prevLogger, prevVld, prevConnectionHelper
		getProject, getBlueprint, reloadBlueprint = prevGetProject, prevGetBlueprint, prevReloadBlueprint
	})
	return reloaded
}

func projectDeploymentsInput() *plugin.ApiResourceInput {
	return &plugin.ApiResourceInput{Params: map[string]string{"projectName": "demo"}}
}

func TestGetOrCreateConnectionCreatesTheConnectionOnTheFirstCall(t *testing.T) {
	notFound := errors.NotFound.New("record not found")
	attached := &coremodels.BlueprintConnection{}
	reloaded := mockProjectWebhookDb(t, func(mockDal *mockdal.Dal) {
		mockDal.On("First", mock.AnythingOfType("*models.WebhookConnection"), mock.Anything).Return(notFound)
		mockDal.On("IsErrorNotFound", notFound).Return(true)
		mockDal.On("Create", mock.AnythingOfType("*models.WebhookConnection"), mock.Anything).Run(func(args mock.Arguments) {
			args.Get(0).(*models.WebhookConnection).ID = 7
		}).Return(nil)
		mockDal.On("Create", mock.AnythingOfType("*models.BlueprintConnection"), mock.Anything).Run(func(args mock.Arguments) {
			*attached = *args.Get(0).(*coremodels.BlueprintConnection)
		}).Return(nil)
	})

	connection, err, shouldReturn := getOrCreateConnection(projectDeploymentsInput(), "deployments")

	// the not-found error from the lookup must not be returned once the connection exists
	assert.Nil(t, err)
	assert.False(t, shouldReturn)
	if assert.NotNil(t, connection) {
		assert.Equal(t, uint64(7), connection.ID)
		assert.Equal(t, "demo_deployments", connection.Name)
	}
	assert.Equal(t, coremodels.BlueprintConnection{BlueprintId: 3, PluginName: "webhook", ConnectionId: 7}, *attached)
	// the scheduled job is reloaded, so it plans with the new connection
	assert.Equal(t, []uint64{3}, *reloaded)
}

func TestGetOrCreateConnectionReturnsTheExistingConnection(t *testing.T) {
	reloaded := mockProjectWebhookDb(t, func(mockDal *mockdal.Dal) {
		mockDal.On("First", mock.AnythingOfType("*models.WebhookConnection"), mock.Anything).Run(func(args mock.Arguments) {
			args.Get(0).(*models.WebhookConnection).ID = 7
		}).Return(nil)
	})

	connection, err, shouldReturn := getOrCreateConnection(projectDeploymentsInput(), "deployments")

	assert.Nil(t, err)
	assert.False(t, shouldReturn)
	if assert.NotNil(t, connection) {
		assert.Equal(t, uint64(7), connection.ID)
	}
	assert.Empty(t, *reloaded)
}

func TestGetOrCreateConnectionFailsWhenTheBlueprintCannotBeReloaded(t *testing.T) {
	notFound := errors.NotFound.New("record not found")
	mockProjectWebhookDb(t, func(mockDal *mockdal.Dal) {
		mockDal.On("First", mock.AnythingOfType("*models.WebhookConnection"), mock.Anything).Return(notFound)
		mockDal.On("IsErrorNotFound", notFound).Return(true)
		mockDal.On("Create", mock.Anything, mock.Anything).Return(nil)
	})
	reloadErr := errors.Default.New("created cron job failed")
	reloadBlueprint = func(blueprintId uint64) errors.Error { return reloadErr }

	connection, err, shouldReturn := getOrCreateConnection(projectDeploymentsInput(), "deployments")

	assert.Nil(t, connection)
	assert.Equal(t, reloadErr, err)
	assert.True(t, shouldReturn)
}
