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

package srvhelper

import (
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

type checkpointScopePlugin struct{}

func (checkpointScopePlugin) Name() string                { return "checkpoint_scope" }
func (checkpointScopePlugin) Description() string         { return "checkpoint scope test" }
func (checkpointScopePlugin) RootPkgPath() string         { return "checkpoint_scope" }
func (checkpointScopePlugin) GetTablesInfo() []dal.Tabler { return nil }

type checkpointScope struct{ plugin.ToolLayerScope }

func (checkpointScope) ScopeParams() interface{} { return "repo" }
func TestGraphqlCheckpointScopeDeletion(t *testing.T) {
	require.NoError(t, plugin.RegisterPlugin("checkpoint_scope", checkpointScopePlugin{}))
	db := mockdal.NewDal(t)
	db.On("AllTables").Return([]string{}, nil)
	db.On("HasTable", mock.AnythingOfType("*models.GraphqlCollectorState")).Return(true)
	helper := &ScopeSrvHelper[plugin.ToolLayerConnection, plugin.ToolLayerScope, plugin.ToolLayerScopeConfig]{
		ModelSrvHelper: &ModelSrvHelper[plugin.ToolLayerScope]{db: db, log: unithelper.DummyLogger()}, pluginName: "checkpoint_scope",
	}
	tx := mockdal.NewTransaction(t)
	tx.On("Exec", "DELETE FROM _devlake_graphql_collector_states WHERE raw_data_table LIKE ? AND raw_data_params = ?", []interface{}{"_raw_checkpoint_scope%", `"repo"`}).Return(nil).Once()
	tx.On("Exec", mock.MatchedBy(func(sql string) bool {
		return sql != "DELETE FROM _devlake_graphql_collector_states WHERE raw_data_table LIKE ? AND raw_data_params = ?"
	}), mock.Anything).Return(nil)
	helper.deleteScopeData(checkpointScope{}, tx)
}
