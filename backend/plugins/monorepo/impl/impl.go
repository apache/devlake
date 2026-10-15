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
	"github.com/apache/devlake/plugins/monorepo/models"
	"github.com/apache/devlake/plugins/monorepo/models/migrationscripts"
	"github.com/apache/devlake/plugins/monorepo/tasks"
)

// make sure interface is implemented
var _ interface {
	plugin.PluginMeta
	plugin.PluginTask
	plugin.PluginModel
	plugin.PluginMetric
	plugin.PluginMigration
	plugin.MetricPluginBlueprintV200
} = (*Monorepo)(nil)

type Monorepo struct{}

func (p Monorepo) Description() string {
	return "Split a monorepo into sub-projects and compute per-sub-project DORA metrics"
}

func (p Monorepo) Name() string {
	return "monorepo"
}

func (p Monorepo) Dashboards() []plugin.GrafanaDashboard {
	return nil
}

func (p Monorepo) SvgIcon() string {
	return `<svg viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
<path fill-rule="evenodd" clip-rule="evenodd" d="M1 1h6v6H1V1zm1 1v4h4V2H2zm7-1h6v6H9V1zm1 1v4h4V2h-4zM1 9h6v6H1V9zm1 1v4h4v-4H2zm7-1h6v6H9V9zm1 1v4h4v-4h-4z" fill="#444444"/>
</svg>`
}

// RequiredDataEntities declares that deployments must be recognisable as CI/CD tasks of
// type Deployment, which is what sub-project attribution matches job names against.
func (p Monorepo) RequiredDataEntities() (data []map[string]interface{}, err errors.Error) {
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

func (p Monorepo) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		// These deprecated tables are still owned and populated by this plugin
		// during the compat window; they must remain registered here so
		// migrations/deletion tooling can find them.
		&models.SubProjectDeployment{}, //nolint:staticcheck // SA1019
		&models.SubProjectPrMetric{},   //nolint:staticcheck // SA1019
	}
}

func (p Monorepo) IsProjectMetric() bool {
	return true
}

// RunAfter declares that this plugin should run after dora. NOTE: this is currently
// advisory metadata only (surfaced via the /plugins API) — core's blueprint plan builder
// (server/services/blueprint_makeplan_v200.go GeneratePlanJsonV200) merges all enabled
// metric plugins' plans with ParallelizePipelinePlans, which zips their stages together by
// index and does not consult RunAfter. Actual ordering against dora is enforced by stage
// padding in MakeMetricPluginPipelinePlanV200 below, not by this declaration.
func (p Monorepo) RunAfter() ([]string, errors.Error) {
	return []string{"dora"}, nil
}

func (p Monorepo) Settings() interface{} {
	return nil
}

func (p Monorepo) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.AttributeDeploymentsMeta,
		tasks.AttributePullRequestsMeta,
		tasks.UpdateProjectPrMetricsSubProjectMeta,
	}
}

func (p Monorepo) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}
	matcher, err := tasks.NewSubProjectMatcher(op.SubProjects)
	if err != nil {
		return nil, err
	}
	return &tasks.MonorepoTaskData{
		Options: op,
		Matcher: matcher,
	}, nil
}

// RootPkgPath information lost when compiled as plugin(.so)
func (p Monorepo) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/monorepo"
}

func (p Monorepo) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Monorepo) MakeMetricPluginPipelinePlanV200(projectName string, options json.RawMessage) (coreModels.PipelinePlan, errors.Error) {
	op := &tasks.MonorepoOptions{}
	if options != nil && string(options) != "\"\"" {
		if err := json.Unmarshal(options, op); err != nil {
			return nil, errors.Default.WrapRaw(err)
		}
	}
	if len(op.SubProjects) == 0 {
		return nil, errors.BadInput.New(
			"the monorepo plugin requires a subProjects list in its metric plugin options")
	}
	// Validate eagerly so a bad regex is reported when the project or blueprint is saved rather
	// than midway through a pipeline run.
	if _, err := tasks.NewSubProjectMatcher(op.SubProjects); err != nil {
		return nil, err
	}

	subProjects := make([]map[string]interface{}, 0, len(op.SubProjects))
	for _, sp := range op.SubProjects {
		subProjects = append(subProjects, map[string]interface{}{
			"name":             sp.Name,
			"prLabels":         sp.PrLabels,
			"deployJobPattern": sp.DeployJobPattern,
		})
	}
	// Preserve an explicit false; only default to true when the caller didn't set it at all.
	includeUnattributed := op.ShouldIncludeUnattributed()

	// attributeDeployments reads cicd_deployment_commits, and attributePullRequests reads
	// project_pr_metrics — both are written by dora's own multi-stage plan (currently 3
	// stages: generate deployments, refdiff, calculate change lead time). Core's
	// ParallelizePipelinePlans merges every enabled metric plugin's plan by stage index, so
	// without padding, our single stage would run concurrently with dora's stage 0 instead
	// of after its stage 2 — a real race that silently produces incomplete/nil-metric
	// output (no error) when dora and monorepo are enabled together, since core's RunAfter
	// contract above is not actually enforced by the scheduler. Padding with empty stages
	// through dora's stage count (and then some, for headroom against future growth) is a
	// workaround, not a fix: if dora's plan ever grows past this padding, the race returns.
	// Revisit if DevLake ever adds real cross-plugin dependency scheduling.
	const stagesToOutlastDora = 6
	plan := make(coreModels.PipelinePlan, stagesToOutlastDora+1)
	for i := 0; i < stagesToOutlastDora; i++ {
		plan[i] = coreModels.PipelineStage{}
	}
	plan[stagesToOutlastDora] = coreModels.PipelineStage{
		{
			Plugin: "monorepo",
			Options: map[string]interface{}{
				"projectName":         projectName,
				"subProjects":         subProjects,
				"includeUnattributed": includeUnattributed,
			},
			Subtasks: []string{
				tasks.AttributeDeploymentsMeta.Name,
				tasks.AttributePullRequestsMeta.Name,
				tasks.UpdateProjectPrMetricsSubProjectMeta.Name,
			},
		},
	}
	return plan, nil
}
