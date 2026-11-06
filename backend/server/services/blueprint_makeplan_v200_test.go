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
	"encoding/json"
	"testing"

	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMakePlanV200(t *testing.T) {
	const projectName = "TestMakePlanV200-project"
	githubName := "TestMakePlanV200-github" // mimic github
	// mock github plugin as a data source plugin
	githubConnId := uint64(1)
	githubScopes := []*coreModels.BlueprintScope{
		{ScopeId: "github:GithubRepo:1:123"},
		{ScopeId: "github:GithubRepo:1:321"},
	}
	githubOutputPlan := coreModels.PipelinePlan{
		{
			{Plugin: githubName, Options: map[string]interface{}{"name": "apache/devlake"}},
			{Plugin: "gitextractor", Options: map[string]interface{}{"url": "http://gihub.com/apache/devlake.git"}},
		},
		{
			{Plugin: githubName, Options: map[string]interface{}{"name": "apache/devlake-website"}},
			{Plugin: "gitextractor", Options: map[string]interface{}{"url": "http://gihub.com/apache/devlake-website.git"}},
		},
	}
	githubOutputScopes := []plugin.Scope{
		&code.Repo{DomainEntity: domainlayer.DomainEntity{Id: "github:GithubRepo:1:123"}, Name: "apache/devlake"},
		&ticket.Board{DomainEntity: domainlayer.DomainEntity{Id: "github:GithubRepo:1:123"}, Name: "apache/devlake"},
	}
	github := new(mockplugin.CompositeDataSourcePluginBlueprintV200)
	github.On("MakeDataSourcePipelinePlanV200", githubConnId, githubScopes).Return(githubOutputPlan, githubOutputScopes, nil)

	// mock dora plugin as a metric plugin
	doraName := "TestMakePlanV200-dora"
	doraOutputPlan := coreModels.PipelinePlan{
		{
			{Plugin: "refdiff", Subtasks: []string{"calculateProjectDeploymentCommitsDiff"}, Options: map[string]interface{}{"projectName": projectName}},
			{Plugin: doraName},
		},
	}
	dora := new(mockplugin.CompositeMetricPluginBlueprintV200)
	dora.On("MakeMetricPluginPipelinePlanV200", projectName, json.RawMessage("{}")).Return(doraOutputPlan, nil)

	// mock org plugin
	org := new(mockplugin.CompositeProjectMapper)
	// The org plugin normally builds a tasks.ProjectMapping here. This test
	// mocks the plugin, so the value only has to round-trip through the plan:
	// keep it local rather than importing plugins/org/tasks, which would make
	// the framework depend on a plugin.
	projectMapping := map[string]interface{}{
		"projectName": projectName,
		"scopes":      []interface{}{},
	}
	orgPlan := coreModels.PipelinePlan{
		{
			{Plugin: "org", Subtasks: []string{"setProjectMapping"}, Options: map[string]interface{}{"projectMappings": []interface{}{projectMapping}}},
		},
	}
	org.On("MapProject", projectName, githubOutputScopes).Return(orgPlan, nil)

	// expectation, establish expectation before any code being launch to avoid unwanted modification
	expectedPlan := make(coreModels.PipelinePlan, 0)
	expectedPlan = append(expectedPlan, orgPlan...)
	expectedPlan = append(expectedPlan, githubOutputPlan...)
	expectedPlan = append(expectedPlan, doraOutputPlan...)

	// plugin registration
	plugin.RegisterPlugin(githubName, github)
	plugin.RegisterPlugin(doraName, dora)
	plugin.RegisterPlugin("org", org)

	// put them together and call GeneratePlanJsonV200
	connections := []*coreModels.BlueprintConnection{
		{PluginName: githubName, ConnectionId: githubConnId, Scopes: githubScopes},
	}
	metrics := map[string]json.RawMessage{
		doraName: nil,
	}

	plan, err := GeneratePlanJsonV200(projectName, connections, metrics, false)
	assert.Nil(t, err)

	assert.Equal(t, expectedPlan, plan)
}

func TestGeneratePlanJsonV200_InjectsProjectName(t *testing.T) {
	const projectName = "TestInjectsProjectName-project"
	const pluginName = "TestInjectsProjectName-datasource"

	connId := uint64(42)
	scopes := []*coreModels.BlueprintScope{
		{ScopeId: "datasource:Board:42:1"},
	}

	// The data-source plugin returns a single task with boardId but no projectName.
	// GeneratePlanJsonV200 must inject projectName into the task options.
	pluginOutputPlan := coreModels.PipelinePlan{
		{
			{Plugin: pluginName, Options: map[string]interface{}{"boardId": float64(1)}},
		},
	}
	pluginOutputScopes := []plugin.Scope{}

	ds := new(mockplugin.CompositeDataSourcePluginBlueprintV200)
	ds.On("MakeDataSourcePipelinePlanV200", connId, scopes).Return(pluginOutputPlan, pluginOutputScopes, nil)

	// Re-register "org" with a permissive mock because GeneratePlanJsonV200
	// calls org.MapProject whenever projectName is non-empty.
	org := new(mockplugin.CompositeProjectMapper)
	org.On("MapProject", mock.Anything, mock.Anything).Return(coreModels.PipelinePlan{}, nil)

	plugin.RegisterPlugin(pluginName, ds)
	plugin.RegisterPlugin("org", org)

	connections := []*coreModels.BlueprintConnection{
		{PluginName: pluginName, ConnectionId: connId, Scopes: scopes},
	}

	plan, err := GeneratePlanJsonV200(projectName, connections, map[string]json.RawMessage{}, false)
	assert.Nil(t, err)

	// The produced plan contains at least one stage with the data-source task.
	// Find the first task that belongs to our data-source plugin and verify its options.
	var dsTask *coreModels.PipelineTask
	for _, stage := range plan {
		for _, task := range stage {
			if task.Plugin == pluginName {
				dsTask = task
				break
			}
		}
		if dsTask != nil {
			break
		}
	}

	assert.NotNil(t, dsTask, "expected to find a task for plugin %q in the plan", pluginName)
	if dsTask != nil {
		// projectName must have been injected
		assert.Equal(t, projectName, dsTask.Options["projectName"],
			"GeneratePlanJsonV200 should inject projectName into task options")
		// original options must be preserved
		assert.Equal(t, float64(1), dsTask.Options["boardId"],
			"original task options (boardId) must be preserved alongside the injected key")
	}
}
