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
	"encoding/json"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/dora/models/migrationscripts"
	"github.com/apache/devlake/plugins/dora/tasks"
)

// make sure interface is implemented
var _ interface {
	plugin.PluginMeta
	plugin.PluginTask
	plugin.PluginModel
	plugin.PluginMetric
	plugin.PluginMigration
	plugin.MetricPluginBlueprintV200
} = (*Dora)(nil)

type Dora struct{}

func (p Dora) Description() string {
	return "collect some Dora data"
}

func (p Dora) Dashboards() []plugin.GrafanaDashboard {
	return nil
}

func (p Dora) SvgIcon() string {
	// FIXME replace it with correct icon
	return `<svg viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
<path fill-rule="evenodd" clip-rule="evenodd" d="M8 0C3.58 0 0 3.58 0 8C0 12.42 3.58 16 8 16C12.42 16 16 12.42 16 8C16 3.58 12.42 0 8 0ZM9 13H7V11H9V13ZM10.93 6.48C10.79 6.8 10.58 7.12 10.31 7.45L9.25 8.83C9.13 8.98 9.01 9.12 8.97 9.25C8.93 9.38 8.88 9.55 8.88 9.77V10H7.12V8.88C7.12 8.88 7.17 8.37 7.33 8.17L8.4 6.73C8.62 6.47 8.75 6.24 8.84 6.05C8.93 5.86 8.96 5.67 8.96 5.47C8.96 5.17 8.86 4.92 8.68 4.72C8.5 4.53 8.24 4.44 7.92 4.44C7.59 4.44 7.33 4.54 7.14 4.73C6.95 4.92 6.81 5.19 6.74 5.54C6.71 5.65 6.64 5.69 6.54 5.68L4.84 5.43C4.72 5.42 4.68 5.35 4.7 5.24C4.82 4.42 5.16 3.77 5.73 3.3C6.3 2.82 7.05 2.58 7.98 2.58C8.45 2.58 8.88 2.65 9.27 2.8C9.66 2.95 9.99 3.14 10.27 3.39C10.55 3.64 10.76 3.94 10.92 4.28C11.07 4.63 11.14 5 11.14 5.4C11.14 5.8 11.07 6.15 10.93 6.48Z" fill="#444444"/>
</svg>`
}

func (p Dora) RequiredDataEntities() (data []map[string]interface{}, err errors.Error) {
	return []map[string]interface{}{
		{
			"model": "cicd_tasks",
			"requiredFields": map[string]string{
				"column":        "type",
				"execptedValue": "Deployment",
			},
		},
	}, nil
}

func (p Dora) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{}
}

func (p Dora) Name() string {
	return "dora"
}

func (p Dora) IsProjectMetric() bool {
	return true
}

func (p Dora) RunAfter() ([]string, errors.Error) {
	return []string{}, nil
}

func (p Dora) Settings() interface{} {
	return nil
}

func (p Dora) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.DeploymentGeneratorMeta,
		tasks.DeploymentCommitsGeneratorMeta,
		tasks.EnrichPrevSuccessDeploymentCommitMeta,
		tasks.EnrichTaskEnvMeta,
		tasks.CalculateChangeLeadTimeMeta,
		tasks.IssuesToIncidentsMeta,
		tasks.ConnectIncidentToDeploymentMeta,
	}
}

func (p Dora) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}
	prHeadRefRegexp, err := tasks.CompilePrHeadRefPattern(op.PrHeadRefPattern)
	if err != nil {
		return nil, err
	}
	return &tasks.DoraTaskData{
		Options:         op,
		PrHeadRefRegexp: prHeadRefRegexp,
	}, nil
}

// RootPkgPath information lost when compiled as plugin(.so)
func (p Dora) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/dora"
}

func (p Dora) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Dora) MakeMetricPluginPipelinePlanV200(projectName string, options json.RawMessage) (coreModels.PipelinePlan, errors.Error) {
	op := &tasks.DoraOptions{}
	if options != nil && string(options) != "\"\"" {
		err := json.Unmarshal(options, op)
		if err != nil {
			return nil, errors.Default.WrapRaw(err)
		}
	}
	// validate early so that an invalid pattern fails when the plan is generated
	prHeadRefRegexp, err := tasks.CompilePrHeadRefPattern(op.PrHeadRefPattern)
	if err != nil {
		return nil, err
	}
	changeLeadTimeOptions := map[string]interface{}{
		"projectName": projectName,
	}
	if prHeadRefRegexp != nil {
		changeLeadTimeOptions["prHeadRefPattern"] = prHeadRefRegexp.String()
	}

	plan := coreModels.PipelinePlan{
		{
			{
				Plugin: "dora",
				Options: map[string]interface{}{
					"projectName": projectName,
				},
				Subtasks: []string{
					"generateDeployments",
					"generateDeploymentCommits",
					"enrichPrevSuccessDeploymentCommits",
				},
			},
		},
		{
			{
				Plugin: "refdiff",
				Options: map[string]interface{}{
					"projectName": projectName,
				},
				Subtasks: []string{
					"calculateDeploymentCommitsDiff",
				},
			},
		},
		{
			{
				Plugin:  "dora",
				Options: changeLeadTimeOptions,
				Subtasks: []string{
					"calculateChangeLeadTime",
					tasks.IssuesToIncidentsMeta.Name,
					"ConnectIncidentToDeployment",
				},
			},
		},
	}
	return plan, nil
}
