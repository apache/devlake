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

package plugin

import (
	"encoding/json"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
)

/*
PluginBlueprintV200 for project support


step 1: blueprint.settings like
	{
		"version": "2.0.0",
		"connections": [
			{
				"plugin": "github",
				"connectionId": 123,
				"scopes": [
					{ "id": null, "name": "apache/incubator-devlake" }
				]
			}
		]
	}

step 2: call plugin PluginBlueprintV200.MakePipelinePlan with
	[
		{ "id": "1", "name": "apache/incubator-devlake" }
	]
	plugin would return PipelinePlan like the following json, and config-ui should use scopeName for displaying
	[
		[
			{ "plugin": "github", "options": { "scopeId": "1", "scopeName": "apache/incubator-devlake" } }
		]
	]
	and []Scope for project_mapping:
	[
		&Repo{ "id": "github:GithubRepo:1:1", "name": "apache/incubator-devlake" },
		&Board{ "id": "github:GithubRepo:1:1", "name": "apache/incubator-devlake" }
	]

step 3: framework should maintain the project_mapping table based on the []Scope array
	[
		{ "projectName": "xxx", "table": "repos", "rowId": "github:GithubRepo:1:1" },
		{ "projectName": "xxx", "table": "boards", "rowId": "github:GithubRepo:1:1" },
	]
*/

// DataSourcePluginBlueprintV200 extends the V100 to provide support for
// Project, so that complex metrics like DORA can be implemented based on a set
// of Data Scopes
type DataSourcePluginBlueprintV200 interface {
	MakeDataSourcePipelinePlanV200(
		connectionId uint64,
		scopes []*models.BlueprintScope,
	) (models.PipelinePlan, []Scope, errors.Error)
}

// BlueprintConnectionV200 contains the pluginName/connectionId  and related Scopes,

// MetricPluginBlueprintV200 is similar to the DataSourcePluginBlueprintV200
// but for Metric Plugin, take dora as an example, it doesn't have any scope,
// nor does it produce any, however, it does require other plugin to be
// executed beforehand, like calculating refdiff before it can connect PR to the
// right Deployment keep in mind it would be called IFF the plugin was enabled
// for the project.
type MetricPluginBlueprintV200 interface {
	MakeMetricPluginPipelinePlanV200(projectName string, options json.RawMessage) (models.PipelinePlan, errors.Error)
}

// ProjectMapper is implemented by the plugin org, which binding project and scopes
type ProjectMapper interface {
	MapProject(projectName string, scopes []Scope) (models.PipelinePlan, errors.Error)
}

// CompositeDataSourcePluginBlueprintV200 is for unit test
type CompositeDataSourcePluginBlueprintV200 interface {
	PluginMeta
	DataSourcePluginBlueprintV200
}

// CompositeMetricPluginBlueprintV200 is for unit test
type CompositeMetricPluginBlueprintV200 interface {
	PluginMeta
	MetricPluginBlueprintV200
}

// CompositePluginBlueprintV200 is for unit test
type CompositePluginBlueprintV200 interface {
	PluginMeta
	DataSourcePluginBlueprintV200
	MetricPluginBlueprintV200
}

// CompositeProjectMapper is for unit test
type CompositeProjectMapper interface {
	PluginMeta
	ProjectMapper
}
