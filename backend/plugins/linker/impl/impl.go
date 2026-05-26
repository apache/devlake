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
	"regexp"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/linker/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/linker/tasks"
)

// make sure interface is implemented
var _ interface {
	plugin.PluginMeta
	plugin.PluginTask
	plugin.PluginModel
	plugin.PluginMetric
	plugin.PluginMigration
	plugin.MetricPluginBlueprintV200
} = (*Linker)(nil)

type Linker struct{}

func (p Linker) Description() string {
	return "link some cross table data together"
}

// RequiredDataEntities hasn't been used so far
func (p Linker) RequiredDataEntities() (data []map[string]interface{}, err errors.Error) {
	return []map[string]interface{}{}, nil
}

func (p Linker) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{}
}

func (p Linker) Name() string {
	return "linker"
}

func (p Linker) IsProjectMetric() bool {
	return true
}

func (p Linker) RunAfter() ([]string, errors.Error) {
	return []string{}, nil
}

func (p Linker) Settings() interface{} {
	return nil
}

func (p Linker) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.LinkPrToIssueMeta,
	}
}

func (p Linker) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}
	taskData := &tasks.LinkerTaskData{
		Options: op,
	}
	if op.PrToIssueRegexp != "" {
		re, err := regexp.Compile(op.PrToIssueRegexp)
		if err != nil {
			return taskData, errors.Convert(err)
		}
		taskData.PrToIssueRegexp = re
	}
	return taskData, nil
}

// RootPkgPath information lost when compiled as plugin(.so)
func (p Linker) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/linker"
}

func (p Linker) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Linker) MakeMetricPluginPipelinePlanV200(projectName string, options json.RawMessage) (coreModels.PipelinePlan, errors.Error) {
	op := &tasks.LinkerOptions{}
	err := json.Unmarshal(options, op)
	if err != nil {
		return nil, errors.Default.WrapRaw(err)
	}
	plan := coreModels.PipelinePlan{
		{
			{
				Plugin: "linker",
				Options: map[string]interface{}{
					"projectName":     projectName,
					"prToIssueRegexp": op.PrToIssueRegexp,
				},
				Subtasks: []string{
					"LinkPrToIssue",
				},
			},
		},
	}
	return plan, nil
}
