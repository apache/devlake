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
	"errors"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

type checkpointScopePlugin struct{}

func (checkpointScopePlugin) Name() string                { return "checkpoint_scope" }
func (checkpointScopePlugin) Description() string         { return "checkpoint scope test" }
func (checkpointScopePlugin) RootPkgPath() string         { return "checkpoint_scope" }
func (checkpointScopePlugin) GetTablesInfo() []dal.Tabler { return nil }

func TestGraphqlCheckpointScopeDeletion(t *testing.T) {
	require.NoError(t, plugin.RegisterPlugin("checkpoint_scope", checkpointScopePlugin{}))
	for _, exists := range []bool{false, true} {
		db := mockdal.NewDal(t)
		db.On("AllTables").Return([]string{}, nil)
		db.On("HasTable", mock.AnythingOfType("*models.GraphqlCollectorState")).Return(exists)
		helper := &GenericScopeApiHelper[any, plugin.ToolLayerScope, any]{db: db, log: unithelper.DummyLogger(), plugin: "checkpoint_scope"}
		tables, err := helper.getAffectedTables("checkpoint_scope")
		require.NoError(t, err)
		if !exists {
			require.NotContains(t, tables, models.GraphqlCollectorState{}.TableName())
			continue
		}
		require.Contains(t, tables, models.GraphqlCollectorState{}.TableName())
		tx := mockdal.NewTransaction(t)
		db.On("Begin").Return(tx)
		tx.On("Exec", "DELETE FROM _devlake_graphql_collector_states WHERE raw_data_table LIKE ? AND raw_data_params = ?", []interface{}{"_raw_checkpoint_scope%", "repo"}).Return(nil).Once()
		tx.On("Commit").Return(nil).Once()
		db.On("Count", mock.Anything).Return(int64(0), nil).Once()
		require.NoError(t, helper.transactionalDelete([]string{models.GraphqlCollectorState{}.TableName()}, "repo"))
	}
}

func TestGraphqlSQLCursorReadFailure(t *testing.T) {
	rows := mockdal.NewRows(t)
	failure := errors.New("connection lost while reading cursor")
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(failure).Once()
	rows.On("Close").Return(nil).Once()
	iterator, err := NewDalCursorIterator(nil, rows, reflect.TypeOf(struct{ ID int }{}))
	require.NoError(t, err)
	staged, _, err := stageGraphqlInput(t.Context(), iterator)
	require.ErrorIs(t, err, failure)
	require.Nil(t, staged)
}
