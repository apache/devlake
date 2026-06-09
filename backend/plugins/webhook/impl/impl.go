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
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/webhook/api"
	"github.com/apache/incubator-devlake/plugins/webhook/models"
	"github.com/apache/incubator-devlake/plugins/webhook/models/migrationscripts"
)

// make sure interface is implemented
var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginMigration
	plugin.DataSourcePluginBlueprintV200
} = (*Webhook)(nil)

type Webhook struct{}

func (p Webhook) Description() string {
	return "collect some Webhook data"
}

func (p Webhook) Name() string {
	return "webhook"
}

func (p Webhook) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)

	return nil
}

func (p Webhook) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.WebhookConnection{},
	}
}

func (p Webhook) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	_ []*coreModels.BlueprintScope,
) (pp coreModels.PipelinePlan, sc []plugin.Scope, err errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(connectionId)
}

// RootPkgPath information lost when compiled as plugin(.so)
func (p Webhook) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/webhook"
}

func (p Webhook) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Webhook) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.ListConnections,
		},
		"connections/:connectionId": {
			"GET":    api.GetConnection,
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
		},
		"connections/:connectionId/deployments": {
			"POST": api.PostDeployments,
		},
		"connections/:connectionId/pull_requests": {
			"POST": api.PostPullRequests,
		},
		"connections/:connectionId/issues": {
			"POST": api.PostIssue,
		},
		"connections/:connectionId/issue/:issueKey/close": {
			"POST": api.CloseIssue,
		},
		":connectionId/deployments": {
			"POST": api.PostDeployments,
		},
		":connectionId/pull_requests": {
			"POST": api.PostPullRequests,
		},
		":connectionId/issues": {
			"POST": api.PostIssue,
		},
		":connectionId/issue/:issueKey/close": {
			"POST": api.CloseIssue,
		},
		"connections/by-name/:connectionName": {
			"GET":    api.GetConnectionByName,
			"PATCH":  api.PatchConnectionByName,
			"DELETE": api.DeleteConnectionByName,
		},
		"connections/by-name/:connectionName/deployments": {
			"POST": api.PostDeploymentsByName,
		},
		"connections/by-name/:connectionName/pull_requests": {
			"POST": api.PostPullRequestsByName,
		},
		"connections/by-name/:connectionName/issues": {
			"POST": api.PostIssueByName,
		},
		"connections/by-name/:connectionName/issue/:issueKey/close": {
			"POST": api.CloseIssueByName,
		},
		"projects/:projectName/deployments": {
			"POST": api.PostDeploymentsByProjectName,
		},
		"projects/:projectName/issues": {
			"POST": api.PostIssuesByProjectName,
		},
		"projects/:projectName/pull_requests": {
			"POST": api.PostPullRequestsByProjectName,
		},
	}
}