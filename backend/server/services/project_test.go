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

package services

import (
	"fmt"
	"testing"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	blueprintservices "github.com/apache/devlake/helpers/pluginhelper/services"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type testOrdinaryPlugin struct {
	name string
}

func (p *testOrdinaryPlugin) Description() string { return "test ordinary plugin" }
func (p *testOrdinaryPlugin) RootPkgPath() string { return "plugins/test_ordinary" }
func (p *testOrdinaryPlugin) Name() string        { return p.name }

type testHookPlugin struct {
	name        string
	deleteCalls []struct {
		tx          dal.Transaction
		projectName string
	}
	deleteErr errors.Error
}

func (p *testHookPlugin) Description() string { return "test hook plugin" }
func (p *testHookPlugin) RootPkgPath() string { return "plugins/test_hook" }
func (p *testHookPlugin) Name() string        { return p.name }
func (p *testHookPlugin) BeforeDeleteProject(tx dal.Transaction, projectName string) errors.Error {
	p.deleteCalls = append(p.deleteCalls, struct {
		tx          dal.Transaction
		projectName string
	}{tx: tx, projectName: projectName})
	return p.deleteErr
}

func registerProjectDeleteTestPlugin(t *testing.T, testPlugin plugin.PluginMeta) {
	t.Helper()
	assert.NoError(t, plugin.RegisterPlugin(testPlugin.Name(), testPlugin))
	t.Cleanup(func() {
		delete(plugin.AllPlugins(), testPlugin.Name())
	})
}

func withProjectTestDatabase(t *testing.T, testDB dal.Dal) {
	t.Helper()
	previousDB, previousManager := db, bpManager
	db = testDB
	bpManager = blueprintservices.NewBlueprintManager(testDB)
	t.Cleanup(func() {
		db = previousDB
		bpManager = previousManager
	})
}

func TestRunProjectDeleteHooks(t *testing.T) {
	t.Run("skips ordinary plugins without ProjectDeleteHook", func(t *testing.T) {
		ordinary := &testOrdinaryPlugin{name: "test-ordinary-skip"}
		registerProjectDeleteTestPlugin(t, ordinary)

		tx := mockdal.NewTransaction(t)
		assert.NoError(t, runProjectDeleteHooks(tx, "test-project"))
	})

	t.Run("invokes implementing plugins with exact transaction and project name", func(t *testing.T) {
		hook := &testHookPlugin{name: "test-hook-invoke"}
		registerProjectDeleteTestPlugin(t, hook)

		tx := mockdal.NewTransaction(t)
		assert.NoError(t, runProjectDeleteHooks(tx, "test-project"))

		assert.Equal(t, 1, len(hook.deleteCalls))
		assert.Equal(t, tx, hook.deleteCalls[0].tx)
		assert.Equal(t, "test-project", hook.deleteCalls[0].projectName)
	})

	t.Run("returns wrapped error on hook veto", func(t *testing.T) {
		expectedErr := errors.Default.New("project delete vetoed by plugin")
		hook := &testHookPlugin{
			name:      "test-hook-veto",
			deleteErr: expectedErr,
		}
		registerProjectDeleteTestPlugin(t, hook)

		tx := mockdal.NewTransaction(t)
		err := runProjectDeleteHooks(tx, "test-project")

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), fmt.Sprintf("error executing delete hook for plugin %s", hook.Name()))
	})
}

func TestDeleteProject_RollsBackOnDeleteHookVeto(t *testing.T) {
	hookErr := errors.Default.New("hook rejection")
	hook := &testHookPlugin{
		name:      "test-hook-rollback-veto",
		deleteErr: hookErr,
	}
	registerProjectDeleteTestPlugin(t, hook)

	tx := mockdal.NewTransaction(t)
	notFound := errors.NotFound.New("blueprint not found")
	mockDB := mockdal.NewDal(t)
	mockDB.On("First", mock.Anything, mock.Anything).Return(nil).Once()
	mockDB.On("First", mock.Anything, mock.Anything).Return(notFound).Once()
	mockDB.On("IsErrorNotFound", mock.Anything).Return(true).Twice()
	mockDB.On("Begin").Return(tx).Once()
	tx.On("Rollback").Return(nil).Once()
	withProjectTestDatabase(t, mockDB)

	err := DeleteProject("project-veto")

	assert.Error(t, err)
	assert.ErrorIs(t, err, hookErr)
	assert.Contains(t, err.Error(), fmt.Sprintf("error executing delete hook for plugin %s", hook.Name()))
	assert.Equal(t, 1, len(hook.deleteCalls))
	assert.Equal(t, "project-veto", hook.deleteCalls[0].projectName)
	tx.AssertNotCalled(t, "Commit")
}

func TestDeleteProject_RollsBackOnBlueprintDeletionFailure(t *testing.T) {
	expectedErr := errors.Default.New("unable to delete blueprint labels")
	tx := mockdal.NewTransaction(t)
	tx.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		args.Get(0).(*models.Blueprint).ID = 42
	}).Return(nil).Once()
	tx.On("Delete", mock.Anything, mock.Anything).Return(expectedErr).Once()
	tx.On("Rollback").Return(nil).Once()

	mockDB := mockdal.NewDal(t)
	mockDB.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		if blueprint, ok := args.Get(0).(*models.Blueprint); ok {
			blueprint.ID = 42
		}
	}).Return(nil).Twice()
	mockDB.On("Pluck", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	mockDB.On("All", mock.Anything, mock.Anything).Return(nil)
	mockDB.On("Count", mock.Anything).Return(int64(0), nil).Once()
	mockDB.On("Begin").Return(tx).Once()
	withProjectTestDatabase(t, mockDB)

	err := DeleteProject("project-blueprint-failure")

	assert.ErrorIs(t, err, expectedErr)
	tx.AssertNotCalled(t, "Commit")
}

func TestDeleteProject_SuccessfulDeletionInSingleTransaction(t *testing.T) {
	hook := &testHookPlugin{
		name: "test-hook-success",
	}
	registerProjectDeleteTestPlugin(t, hook)

	tx := mockdal.NewTransaction(t)
	tx.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		args.Get(0).(*models.Blueprint).ID = 42
	}).Return(nil).Once()
	tx.On("Delete", mock.Anything, mock.Anything).Return(nil)
	tx.On("Delete", mock.Anything).Return(nil)
	tx.On("Commit").Return(nil).Once()

	mockDB := mockdal.NewDal(t)
	mockDB.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		if blueprint, ok := args.Get(0).(*models.Blueprint); ok {
			blueprint.ID = 42
		}
	}).Return(nil).Twice()
	mockDB.On("Pluck", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	mockDB.On("All", mock.Anything, mock.Anything).Return(nil)
	mockDB.On("Count", mock.Anything).Return(int64(0), nil).Once()
	mockDB.On("Begin").Return(tx).Once()
	withProjectTestDatabase(t, mockDB)

	err := DeleteProject("project-success")

	assert.NoError(t, err)
	assert.Equal(t, 1, len(hook.deleteCalls))
	assert.Equal(t, tx, hook.deleteCalls[0].tx)
	assert.Equal(t, "project-success", hook.deleteCalls[0].projectName)
	assert.Len(t, tx.Calls, 11)
	tx.AssertNotCalled(t, "Rollback")
}
