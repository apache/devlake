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
	"time"

	coreModels "github.com/apache/incubator-devlake/core/models"

	"github.com/apache/incubator-devlake/core/dal"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"

	"github.com/apache/incubator-devlake/plugins/sonarqube/api"
	"github.com/apache/incubator-devlake/plugins/sonarqube/models"
	"github.com/apache/incubator-devlake/plugins/sonarqube/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/sonarqube/tasks"
)

// make sure interface is implemented
var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginMigration
	plugin.DataSourcePluginBlueprintV200
	plugin.CloseablePluginTask
	plugin.PluginSource
} = (*Sonarqube)(nil)

type Sonarqube struct{}

func (p Sonarqube) Description() string {
	return "collect some Sonarqube data"
}

func (p Sonarqube) Name() string {
	return "sonarqube"
}

func (p Sonarqube) Init(br context.BasicRes) errors.Error {
	api.Init(br, p)

	return nil
}

func (p Sonarqube) Connection() dal.Tabler {
	return &models.SonarqubeConnection{}
}

func (p Sonarqube) Scope() plugin.ToolLayerScope {
	return &models.SonarqubeProject{}
}

func (p Sonarqube) ScopeConfig() dal.Tabler {
	return nil
}

func (p Sonarqube) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.SonarqubeConnection{},
		&models.SonarqubeProject{},
		&models.SonarqubeIssue{},
		&models.SonarqubeIssueImpact{},
		&models.SonarqubeIssueCodeBlock{},
		&models.SonarqubeHotspot{},
		&models.SonarqubeFileMetrics{},
		&models.SonarqubeAccount{},
		&models.SonarqubeScopeConfig{},
	}
}

func (p Sonarqube) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectAdditionalFilemetricsMeta,
		tasks.ExtractAdditionalFileMetricsMeta,
		tasks.CollectFilemetricsMeta,
		tasks.ExtractFilemetricsMeta,
		tasks.CollectIssuesMeta,
		tasks.ExtractIssuesMeta,
		tasks.CollectHotspotsMeta,
		tasks.ExtractHotspotsMeta,
		tasks.CollectAccountsMeta,
		tasks.ExtractAccountsMeta,
		tasks.ConvertProjectsMeta,
		tasks.ConvertIssuesMeta,
		tasks.ConvertIssueImpactsMeta,
		tasks.ConvertIssueCodeBlocksMeta,
		tasks.ConvertHotspotsMeta,
		tasks.ConvertFileMetricsMeta,
		tasks.ConvertAccountsMeta,
	}
}

func (p Sonarqube) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	logger := taskCtx.GetLogger()
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}
	connectionHelper := helper.NewConnectionHelper(
		taskCtx,
		nil,
		p.Name(),
	)
	connection := &models.SonarqubeConnection{}
	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get Sonarqube connection by the given connection ID")
	}

	apiClient, err := tasks.CreateApiClient(taskCtx, connection)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get Sonarqube API client instance")
	}
	taskData := &tasks.SonarqubeTaskData{
		Options:       op,
		ApiClient:     apiClient,
		TaskStartTime: time.Now(),
		IsCloud:       connection.IsCloud(),
	}
	// even we have project in _tool_sonaqube_projects, we still need to collect project to update LastAnalysisDate
	var apiProject *models.SonarqubeApiProject
	apiProject, err = api.GetApiProject(op.ProjectKey, apiClient)
	if err != nil {
		return nil, err
	}
	logger.Debug(fmt.Sprintf("Current project: %s", apiProject.ProjectKey))
	scope := apiProject.ConvertApiScope()
	scope.ConnectionId = op.ConnectionId
	err = taskCtx.GetDal().CreateOrUpdate(&scope)
	if err != nil {
		return nil, err
	}
	taskData.LastAnalysisDate = scope.LastAnalysisDate.ToNullableTime()

	return taskData, nil
}

// RootPkgPath information lost when compiled as plugin(.so)
func (p Sonarqube) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/sonarqube"
}

func (p Sonarqube) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Sonarqube) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"connections/:connectionId/scopes/:scopeId": {
			"GET":    api.GetScope,
			"PATCH":  api.UpdateScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopeList,
			"PUT": api.PutScope,
		},
		"connections/:connectionId/scopes/:scopeId/latest-sync-state": {
			"GET": api.GetScopeLatestSyncState,
		},

		"connections/:connectionId/proxy/rest/*path": {
			"GET": api.Proxy,
		},
	}
}

func (p Sonarqube) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (pp coreModels.PipelinePlan, sc []plugin.Scope, err errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p Sonarqube) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.SonarqubeTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	data.ApiClient.Release()
	return nil
}
