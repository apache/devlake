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
	"testing"

	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/stretchr/testify/assert"
)

func TestMakeMetricPluginPipelinePlanV200SkipsEmptyRegexp(t *testing.T) {
	var linker Linker

	plan, err := linker.MakeMetricPluginPipelinePlanV200("project", json.RawMessage(`{}`))

	assert.Nil(t, err)
	assert.Nil(t, plan)
}

func TestMakeMetricPluginPipelinePlanV200BuildsPlan(t *testing.T) {
	var linker Linker
	options, err := json.Marshal(map[string]string{
		"prToIssueRegexp": `#(\d+)`,
	})
	assert.Nil(t, err)

	plan, err := linker.MakeMetricPluginPipelinePlanV200("project", options)

	assert.Nil(t, err)
	assert.Equal(t, coreModels.PipelinePlan{
		{
			{
				Plugin: "linker",
				Options: map[string]interface{}{
					"projectName":     "project",
					"prToIssueRegexp": `#(\d+)`,
				},
				Subtasks: []string{
					"LinkPrToIssue",
				},
			},
		},
	}, plan)
}

func TestPrepareTaskDataRejectsEmptyRegexp(t *testing.T) {
	var linker Linker

	taskData, err := linker.PrepareTaskData(nil, map[string]interface{}{
		"projectName":     "project",
		"prToIssueRegexp": "",
	})

	assert.NotNil(t, err)
	assert.Nil(t, taskData)
}
