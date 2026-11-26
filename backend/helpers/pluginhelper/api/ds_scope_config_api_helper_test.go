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
	"testing"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/srvhelper"
	"github.com/apache/devlake/helpers/unithelper"
	mockcontext "github.com/apache/devlake/mocks/core/context"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testScopeConfig is a minimal scope config used to exercise the ownership check.
type testScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`
}

func (testScopeConfig) TableName() string {
	return "_tool_test_scope_configs"
}

// testScopeConfigConnection is a minimal connection: ToolLayerConnection only
// needs a table name and the id accessor.
type testScopeConfigConnection struct {
	common.Model
}

func (testScopeConfigConnection) TableName() string {
	return "_tool_test_scope_config_connections"
}

func (c testScopeConfigConnection) ConnectionId() uint64 {
	return c.ID
}

// mockScopeConfigHelper builds a DsScopeConfigApiHelper whose database returns
// the passed scope config for every primary key lookup.
func mockScopeConfigHelper(t *testing.T, stored *testScopeConfig) (*DsScopeConfigApiHelper[testScopeConfigConnection, TestFakeGithubRepo, testScopeConfig], *mockdal.Dal) {
	t.Helper()
	mockDal := new(mockdal.Dal)
	mockDal.On("GetColumns", mock.Anything, mock.Anything).Return(
		[]dal.ColumnMeta{newTestColumnMeta(t, "id")}, nil,
	)
	mockDal.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*testScopeConfig) = *stored
	}).Return(nil)

	mockRes := new(mockcontext.BasicRes)
	mockRes.On("GetDal").Return(mockDal)
	mockRes.On("GetLogger").Return(unithelper.DummyLogger())

	srvHelper := srvhelper.NewScopeConfigSrvHelper[testScopeConfigConnection, TestFakeGithubRepo, testScopeConfig](mockRes, nil)
	return NewDsScopeConfigApiHelper[testScopeConfigConnection, TestFakeGithubRepo, testScopeConfig](mockRes, srvHelper, nil), mockDal
}

func newTestColumnMeta(t *testing.T, name string) *mockdal.ColumnMeta {
	t.Helper()
	columnMeta := new(mockdal.ColumnMeta)
	columnMeta.On("Name").Return(name)
	columnMeta.On("PrimaryKey").Return(true, true)
	return columnMeta
}

func patchScopeConfigInput(connectionId, scopeConfigId uint64) *plugin.ApiResourceInput {
	return &plugin.ApiResourceInput{
		Params: map[string]string{
			"connectionId":  fmt.Sprintf("%d", connectionId),
			"scopeConfigId": fmt.Sprintf("%d", scopeConfigId),
		},
		Body: map[string]interface{}{"name": "patched"},
	}
}

func TestPatchScopeConfigRejectsForeignConnection(t *testing.T) {
	stored := &testScopeConfig{ScopeConfig: common.ScopeConfig{
		Model:        common.Model{ID: 5},
		ConnectionId: 1,
		Name:         "original",
	}}
	apiHelper, mockDal := mockScopeConfigHelper(t, stored)
	// Nothing must be written when the connection in the URL does not own the scope config.
	mockDal.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()

	output, err := apiHelper.Patch(patchScopeConfigInput(2, 5))

	require.NotNil(t, err)
	assert.Equal(t, errors.NotFound, err.GetType())
	assert.Nil(t, output)
	mockDal.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	assert.Equal(t, uint64(1), stored.ConnectionId, "the stored scope config must keep its connection")
	assert.Equal(t, "original", stored.Name, "the stored scope config must not be modified")
}

func TestPatchScopeConfigAcceptsOwningConnection(t *testing.T) {
	stored := &testScopeConfig{ScopeConfig: common.ScopeConfig{
		Model:        common.Model{ID: 5},
		ConnectionId: 1,
		Name:         "original",
	}}
	apiHelper, mockDal := mockScopeConfigHelper(t, stored)
	var updated *testScopeConfig
	mockDal.On("Update", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		updated = args.Get(0).(*testScopeConfig)
	}).Return(nil).Once()

	output, err := apiHelper.Patch(patchScopeConfigInput(1, 5))

	require.Nil(t, err)
	require.NotNil(t, output)
	require.NotNil(t, updated)
	assert.Equal(t, uint64(1), updated.ConnectionId)
	assert.Equal(t, "patched", updated.Name)
}
