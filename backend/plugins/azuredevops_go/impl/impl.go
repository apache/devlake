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

	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/helpers/pluginhelper/subtaskmeta/sorter"
	"github.com/apache/devlake/plugins/azuredevops_go/tasks"

	"github.com/apache/devlake/core/models/domainlayer/devops"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
	"github.com/apache/devlake/plugins/azuredevops_go/models/migrationscripts"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginSource
	plugin.DataSourcePluginBlueprintV200
	plugin.CloseablePluginTask
} = (*Azuredevops)(nil)

var sortedSubtaskMetas []plugin.SubTaskMeta

type Azuredevops struct{}

func init() {
	var err error
	// check subtask meta loop and gen subtask list when plugin init
	sortedSubtaskMetas, err = sorter.NewTableSorter(tasks.SubTaskMetaList).Sort()
	if err != nil {
		panic(err)
	}
}

func (p Azuredevops) Connection() dal.Tabler {
	return &models.AzuredevopsConnection{}
}

func (p Azuredevops) Scope() plugin.ToolLayerScope {
	return &models.AzuredevopsRepo{}
}

func (p Azuredevops) ScopeConfig() dal.Tabler {
	return &models.AzuredevopsScopeConfig{}
}

func (p Azuredevops) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)

	return nil
}

func (p Azuredevops) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.AzuredevopsBuild{},
		&models.AzuredevopsCommit{},
		&models.AzuredevopsConnection{},
		&models.AzuredevopsPrCommit{},
		&models.AzuredevopsPrLabel{},
		&models.AzuredevopsProject{},
		&models.AzuredevopsPullRequest{},
		&models.AzuredevopsRepo{},
		&models.AzuredevopsRepoCommit{},
		&models.AzuredevopsScopeConfig{},
		&models.AzuredevopsTimelineRecord{},
		&models.AzuredevopsUser{},
		&models.AzuredevopsIteration{},
		&models.AzuredevopsWorkItem{},
		&models.AzuredevopsWorkItemComment{},
		&models.AzuredevopsWorkItemUpdate{},
		&models.AzuredevopsTestPlan{},
		&models.AzuredevopsTestCase{},
		&models.AzuredevopsTestRun{},
		&models.AzuredevopsTestResult{},
	}
}

func (p Azuredevops) Description() string {
	return "To collect and enrich data from Azure DevOps"
}

func (p Azuredevops) Name() string {
	return "azuredevops_go"
}

func (p Azuredevops) SubTaskMetas() []plugin.SubTaskMeta {
	return sortedSubtaskMetas
}

func (p Azuredevops) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	logger := taskCtx.GetLogger()

	logger.Info("Initializing Azure DevOps Go Plugin")
	op, err := tasks.DecodeTaskOptions(options)
	if err != nil {
		return nil, err
	}
	connectionHelper := helper.NewConnectionHelper(
		taskCtx,
		nil,
		p.Name(),
	)
	connection := &models.AzuredevopsConnection{}
	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to retrieve an Azure DevOps connection from the database using the provided connection ID")
	}

	apiClient, err := tasks.CreateApiClient(taskCtx, connection)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to retrieve an Azure DevOps connection from the database using the provided connection ID")
	}

	if op.RepositoryId != "" {
		var scope *models.AzuredevopsRepo
		db := taskCtx.GetDal()
		err = db.First(&scope, dal.Where("connection_id = ? AND id = ?", op.ConnectionId, op.RepositoryId))
		if err == nil {
			if op.ScopeConfigId == 0 && scope.ScopeConfigId != 0 {
				op.ScopeConfigId = scope.ScopeConfigId
			}
		}

		if err != nil {
			return nil, errors.Default.Wrap(err, fmt.Sprintf("fail to find repositors: %s/%s", op.ProjectId, op.RepositoryId))
		}
	}

	if op.ScopeConfig == nil {
		if op.ScopeConfigId != 0 {
			logger.Info("scope config is nil. trying to read config with id %d from database", op.ScopeConfigId)
			var scopeConfig models.AzuredevopsScopeConfig
			db := taskCtx.GetDal()
			err = db.First(&scopeConfig, dal.Where("id = ?", op.ScopeConfigId))
			if err != nil {
				if db.IsErrorNotFound(err) {
					return nil, errors.Default.Wrap(err, fmt.Sprintf("can not find scopeConfigs by scopeConfigId [%d]", op.ScopeConfigId))
				}
				return nil, errors.Default.Wrap(err, fmt.Sprintf("fail to find scopeConfigs by scopeConfigId [%d]", op.ScopeConfigId))
			}
			op.ScopeConfig = &scopeConfig
		} else {
			logger.Info("scope config is nil. initializing as empty struct")
			op.ScopeConfig = &models.AzuredevopsScopeConfig{}
		}
	}
	logger.Info("scope config is taken from task options %v", op.ScopeConfig)
	regexEnricher := helper.NewRegexEnricher()
	if err = regexEnricher.TryAdd(devops.DEPLOYMENT, op.ScopeConfig.DeploymentPattern); err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid value for `deploymentPattern`")
	}
	if err = regexEnricher.TryAdd(devops.PRODUCTION, op.ScopeConfig.ProductionPattern); err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid value for `productionPattern`")
	}

	taskData := &tasks.AzuredevopsTaskData{
		Options:       op,
		ApiClient:     apiClient,
		RegexEnricher: regexEnricher,
	}

	if op.TimeAfter != "" {
		var timeAfter time.Time
		timeAfter, err = errors.Convert01(time.Parse(time.RFC3339, op.TimeAfter))
		if err != nil {
			return nil, errors.BadInput.Wrap(err, "invalid value for `timeAfter`")
		}
		taskData.TimeAfter = &timeAfter
		logger.Debug("collect data updated timeAfter %s", timeAfter)
	}
	return taskData, nil
}

func (p Azuredevops) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/azuredevops"
}

func (p Azuredevops) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Azuredevops) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"connections/:connectionId/scopes/:scopeId": {
			"GET":    api.GetScope,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopes,
			"PUT": api.PutScopes,
		},
		"connections/:connectionId/scope-configs": {
			"POST": api.PostScopeConfig,
			"GET":  api.GetScopeConfigList,
		},
		"connections/:connectionId/scope-configs/:scopeConfigId": {
			"PATCH":  api.PatchScopeConfig,
			"GET":    api.GetScopeConfig,
			"DELETE": api.DeleteScopeConfig,
		},
		"connections/:connectionId/remote-scopes": {
			"GET": api.RemoteScopes,
		},
		"connections/:connectionId/search-remote-scopes": {
			"GET": api.SearchRemoteScopes,
		},
		"connections/:connectionId/proxy/rest/*path": {
			"GET": api.Proxy,
		},
		"scope-config/:scopeConfigId/projects": {
			"GET": api.GetProjectsByScopeConfig,
		},
	}
}

func (p Azuredevops) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (pp coreModels.PipelinePlan, sc []plugin.Scope, err errors.Error) {
	return api.MakePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p Azuredevops) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.AzuredevopsTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	data.ApiClient.Release()
	return nil
}
