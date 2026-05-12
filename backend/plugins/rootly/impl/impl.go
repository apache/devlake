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

package impl

import (
	"fmt"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/rootly/api"
	"github.com/apache/incubator-devlake/plugins/rootly/models"
	"github.com/apache/incubator-devlake/plugins/rootly/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/rootly/tasks"
)

// make sure interface is implemented

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginApi
	plugin.PluginModel
	plugin.DataSourcePluginBlueprintV200
	plugin.CloseablePluginTask
	plugin.PluginSource
} = (*Rootly)(nil)

type Rootly struct{}

func (p Rootly) Description() string {
	return "collect Rootly incident data"
}

func (p Rootly) Name() string {
	return "rootly"
}

func (p Rootly) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Rootly) Connection() dal.Tabler {
	return &models.RootlyConnection{}
}

func (p Rootly) Scope() plugin.ToolLayerScope {
	return &models.Service{}
}

func (p Rootly) ScopeConfig() dal.Tabler {
	return &models.RootlyScopeConfig{}
}

func (p Rootly) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{}
}

func (p Rootly) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.Service{},
		&models.Incident{},
		&models.User{},
		&models.Assignment{},
		&models.RootlyConnection{},
		&models.RootlyScopeConfig{},
	}
}

func (p Rootly) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}
	connectionHelper := helper.NewConnectionHelper(
		taskCtx,
		nil,
		p.Name(),
	)
	connection := &models.RootlyConnection{}
	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get Rootly connection by the given connection ID")
	}

	client, err := helper.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}
	asyncClient, err := helper.CreateAsyncApiClient(taskCtx, client, nil)
	if err != nil {
		return nil, err
	}
	return &tasks.RootlyTaskData{
		Options: op,
		Client:  asyncClient,
	}, nil
}

// RootPkgPath information lost when compiled as plugin(.so)
func (p Rootly) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/rootly"
}

func (p Rootly) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Rootly) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"test": {
			"POST": api.TestConnection,
		},
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.ListConnections,
		},
		"connections/:connectionId": {
			"GET":    api.GetConnection,
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
		},
		"connections/:connectionId/test": {
			"POST": api.TestExistingConnection,
		},
		"connections/:connectionId/remote-scopes": {
			"GET": api.RemoteScopes,
		},
		"connections/:connectionId/search-remote-scopes": {
			"GET": api.SearchRemoteScopes,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopeList,
			"PUT": api.PutScopes,
		},
		"connections/:connectionId/scopes/:scopeId": {
			"GET":    api.GetScope,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes/:scopeId/latest-sync-state": {
			"GET": api.GetScopeLatestSyncState,
		},
	}
}

func (p Rootly) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p Rootly) Close(taskCtx plugin.TaskContext) errors.Error {
	_, ok := taskCtx.GetData().(*tasks.RootlyTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	return nil
}
