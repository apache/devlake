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
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	pluginhelper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/testregistry/api"
	"github.com/apache/devlake/plugins/testregistry/models"
	"github.com/apache/devlake/plugins/testregistry/models/migrationscripts"
	"github.com/apache/devlake/plugins/testregistry/tasks"
)

// make sure interface is implemented
var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginMigration
	plugin.PluginTask
	plugin.DataSourcePluginBlueprintV200
} = (*TestRegistry)(nil)

type TestRegistry struct{}

func (p TestRegistry) Description() string {
	return "Test Registry plugin for isolated development"
}

func (p TestRegistry) Name() string {
	return "testregistry"
}

func (p TestRegistry) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p TestRegistry) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.TestRegistryConnection{},
		&models.TestRegistryScope{},
		&models.TestRegistryScopeConfig{},
		&models.TestRegistryCIJob{},
		&models.TestSuite{},
		&models.TestCase{},
	}
}

func (p TestRegistry) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectProwJobsMeta,
		tasks.CollectTektonJobsMeta,
		// Add more tasks here as needed (extractors, converters, etc.)
	}
}

func (p TestRegistry) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.TestRegistryOptions
	if err := pluginhelper.Decode(options, &op, nil); err != nil {
		return nil, err
	}

	connectionHelper := pluginhelper.NewConnectionHelper(
		taskCtx,
		nil,
		p.Name(),
	)
	connection := &models.TestRegistryConnection{}
	err := connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, err
	}

	// Initialize the JUnit regex from connection configuration
	// Uses default regex if JUnitRegex is empty or invalid
	logger := taskCtx.GetLogger()
	junitRegex := tasks.GetJUnitRegexOrDefault(connection.JUnitRegex, logger)
	if connection.JUnitRegex != "" {
		logger.Info("Using custom JUnit regex pattern: %s", connection.JUnitRegex)
	} else {
		logger.Debug("Using default JUnit regex pattern: %s", tasks.DefaultJUnitRegexPattern)
	}

	taskData := &tasks.TestRegistryTaskData{
		Options:    &op,
		Connection: connection,
		JUnitRegex: junitRegex,
	}

	return taskData, nil
}

func (p TestRegistry) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (pp coreModels.PipelinePlan, sc []plugin.Scope, err errors.Error) {
	return api.MakeDataSourcePipelinePlanV200Impl(p.SubTaskMetas(), connectionId, bpScopes)
}

// RootPkgPath information lost when compiled as plugin(.so)
func (p TestRegistry) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/testregistry"
}

func (p TestRegistry) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p TestRegistry) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"connections/:connectionId/scopes/*scopeId": {
			// Behind 'GetScopeDispatcher', there are two paths so far:
			// GetScopeLatestSyncState "connections/:connectionId/scopes/:scopeId/latest-sync-state"
			// GetScope "connections/:connectionId/scopes/:scopeId"
			// Because there may be slash in scopeId (fullName), so we handle it manually.
			"GET":    api.GetScopeDispatcher,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopeList,
			"PUT": api.PutScopes,
		},
		"connections/:connectionId/scope-configs": {
			"POST": api.CreateScopeConfig,
			"GET":  api.GetScopeConfigList,
		},
		"connections/:connectionId/scope-configs/:scopeConfigId": {
			"PATCH":  api.UpdateScopeConfig,
			"GET":    api.GetScopeConfig,
			"DELETE": api.DeleteScopeConfig,
		},
		"scope-config/:scopeConfigId/projects": {
			"GET": api.GetProjectsByScopeConfig,
		},
		// Push endpoints: external CI systems POST JUnit XML results here.
		"connections/:connectionId/test_results": {
			"POST": api.PostTestResults,
		},
		":connectionId/test_results": {
			"POST": api.PostTestResults,
		},
		"connections/by-name/:connectionName/test_results": {
			"POST": api.PostTestResultsByName,
		},
	}
}
