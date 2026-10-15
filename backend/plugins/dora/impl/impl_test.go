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
	"github.com/apache/devlake/plugins/dora/tasks"
	"testing"

	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/stretchr/testify/assert"
)

func TestMakeMetricPluginPipelinePlanV200(t *testing.T) {
	var dora Dora
	const projectName = "TestMakePlanV200-project"
	// mock dora plugin as a metric plugin
	option := map[string]interface{}{
		"projectName": projectName,
	}

	optionJson, err := json.Marshal(option)
	assert.Nil(t, err)
	plan, err := dora.MakeMetricPluginPipelinePlanV200(projectName, optionJson)
	doraOutputPlan := coreModels.PipelinePlan{
		coreModels.PipelineStage{
			{
				Plugin: "dora",
				Subtasks: []string{
					"generateDeployments",
					"generateDeploymentCommits",
					"enrichPrevSuccessDeploymentCommits",
				},
				Options: map[string]interface{}{"projectName": projectName},
			},
		},
		coreModels.PipelineStage{
			{
				Plugin:   "refdiff",
				Subtasks: []string{"calculateDeploymentCommitsDiff"},
				Options:  map[string]interface{}{"projectName": projectName},
			},
		},
		coreModels.PipelineStage{
			{
				Plugin: "dora",
				Subtasks: []string{
					"calculateChangeLeadTime",
					tasks.IssuesToIncidentsMeta.Name,
					"ConnectIncidentToDeployment",
				},
				Options: map[string]interface{}{"projectName": projectName},
			},
		},
	}
	assert.Equal(t, doraOutputPlan, plan)
}

func TestMakeMetricPluginPipelinePlanV200WithPrHeadRefPattern(t *testing.T) {
	var dora Dora
	const projectName = "TestMakePlanV200-project"
	optionJson, err := json.Marshal(map[string]interface{}{
		"prHeadRefPattern": " ^(feat|fix)/ ",
	})
	assert.Nil(t, err)
	plan, err := dora.MakeMetricPluginPipelinePlanV200(projectName, optionJson)
	assert.Nil(t, err)
	assert.Len(t, plan, 3)
	assert.Equal(t, map[string]interface{}{"projectName": projectName}, plan[0][0].Options)
	assert.Equal(t, map[string]interface{}{"projectName": projectName}, plan[1][0].Options)
	assert.Equal(t, map[string]interface{}{
		"projectName":      projectName,
		"prHeadRefPattern": "^(feat|fix)/",
	}, plan[2][0].Options)
}

func TestMakeMetricPluginPipelinePlanV200WithBlankPrHeadRefPattern(t *testing.T) {
	var dora Dora
	const projectName = "TestMakePlanV200-project"
	plan, err := dora.MakeMetricPluginPipelinePlanV200(projectName, json.RawMessage(`{"prHeadRefPattern":"  "}`))
	assert.Nil(t, err)
	assert.Equal(t, map[string]interface{}{"projectName": projectName}, plan[2][0].Options)
}

func TestPrepareTaskDataPrHeadRefPattern(t *testing.T) {
	var dora Dora

	data, err := dora.PrepareTaskData(nil, map[string]interface{}{"projectName": "p", "prHeadRefPattern": "^feat/"})
	assert.Nil(t, err)
	if re := data.(*tasks.DoraTaskData).PrHeadRefRegexp; assert.NotNil(t, re) {
		assert.Equal(t, "^feat/", re.String())
	}

	data, err = dora.PrepareTaskData(nil, map[string]interface{}{"projectName": "p"})
	assert.Nil(t, err)
	assert.Nil(t, data.(*tasks.DoraTaskData).PrHeadRefRegexp)

	data, err = dora.PrepareTaskData(nil, map[string]interface{}{"projectName": "p", "prHeadRefPattern": "^(feat"})
	assert.Nil(t, data)
	if assert.NotNil(t, err) {
		assert.Equal(t, errors.BadInput, err.GetType())
	}
}

func TestMakeMetricPluginPipelinePlanV200WithInvalidPrHeadRefPattern(t *testing.T) {
	var dora Dora
	plan, err := dora.MakeMetricPluginPipelinePlanV200("p", json.RawMessage(`{"prHeadRefPattern":"^(feat"}`))
	assert.Nil(t, plan)
	if assert.NotNil(t, err) {
		assert.Equal(t, errors.BadInput, err.GetType())
	}
}
