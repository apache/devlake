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

package tasks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
)

func TestProjectWorkItemsWIQL(t *testing.T) {
	created := time.Date(2026, 3, 4, 15, 0, 0, 0, time.FixedZone("ART", -3*3600))
	query := projectWorkItemsWIQL("O'Brien", &created, nil)
	assert.Contains(t, query, "[System.TeamProject] = 'O''Brien'")
	assert.Contains(t, query, "[System.CreatedDate] >= '2026-03-04'")
	assert.Contains(t, query, "[System.WorkItemType] <> 'Test Case'")
	assert.NotContains(t, query, "AND [System.ChangedDate]")
	assert.Contains(t, query, "ORDER BY [System.ChangedDate] DESC")
}

func TestIterationPathKey(t *testing.T) {
	assert.Equal(t, `Proj\Sprint 1`, iterationPathKey(`Proj\Iteration\Sprint 1`, "Proj"))
	assert.Equal(t, `Proj\Sprint 1`, iterationPathKey(`Proj/Sprint 1`, "Proj"))
	assert.Equal(t, `Proj\Area\Sprint 1`, iterationPathKey(`Proj\Iteration\Area\Sprint 1`, "Proj"))
	assert.Equal(t, "", iterationPathKey("Proj", "Proj"))
	assert.Equal(t, "", iterationPathKey(`Proj\Iteration`, "Proj"))
	assert.Equal(t, "", iterationPathKey(`Other\Sprint 1`, "Proj"))
}

func TestResolveWorkItemType(t *testing.T) {
	mappings := datatypes.JSONMap{
		"Task": map[string]interface{}{"standardType": "REQUIREMENT"},
	}
	assert.Equal(t, "REQUIREMENT", resolveWorkItemType("Agile", "Task", true, mappings))
	assert.Equal(t, "REQUIREMENT", resolveWorkItemType("Agile", "User Story", false, nil))
	assert.Equal(t, "BUG", resolveWorkItemType("Scrum", "Bug", false, nil))
	assert.Equal(t, "SUBTASK", resolveWorkItemType("Agile", "Task", true, nil))
	assert.Equal(t, "TASK", resolveWorkItemType("Agile", "Task", false, nil))
	assert.Equal(t, "TASK", resolveWorkItemType("", "Custom", false, nil))
	assert.False(t, shouldConvertWorkItem("Test Case"))
	assert.True(t, shouldConvertWorkItem("Bug"))
}

func TestResolveWorkItemStatus(t *testing.T) {
	mappings := datatypes.JSONMap{
		"Blocked": map[string]interface{}{"standardStatus": "IN_PROGRESS"},
	}
	categories := map[string]map[string]string{
		"Bug": {"Approved": "InProgress"},
	}
	assert.Equal(t, "IN_PROGRESS", resolveWorkItemStatus("Bug", "Blocked", mappings, categories))
	assert.Equal(t, "IN_PROGRESS", resolveWorkItemStatus("Bug", "Approved", nil, categories))
	assert.Equal(t, "DONE", resolveWorkItemStatus("Bug", "Closed", nil, nil))
	assert.Equal(t, "OTHER", resolveWorkItemStatus("Bug", "", nil, nil))
	assert.Equal(t, "OTHER", resolveWorkItemStatus("Bug", "Mystery", nil, nil))
}

func TestMapTestOutcome(t *testing.T) {
	assert.Equal(t, "SUCCESS", mapTestOutcome("Passed"))
	assert.Equal(t, "FAILED", mapTestOutcome("Blocked"))
	assert.Equal(t, "IN_PROGRESS", mapTestOutcome("In Progress"))
	assert.Equal(t, "PENDING", mapTestOutcome(""))
	assert.Equal(t, "PENDING", mapTestOutcome("Not Executed"))
}

func TestHoursToMinutes(t *testing.T) {
	minutes := hoursToMinutes(1.5)
	if assert.NotNil(t, minutes) {
		assert.Equal(t, int64(90), *minutes)
	}
	assert.Nil(t, hoursToMinutes(nil))
}

func TestCustomFieldsKeepDisplayNames(t *testing.T) {
	encoded := customFieldsJSON(map[string]interface{}{
		"System.Title":                   "skip",
		"Custom.Risk":                    "high",
		"Microsoft.VSTS.Common.Priority": 1,
	}, map[string]string{"Custom.Risk": "Risk"})
	assert.JSONEq(t, `{"Custom.Risk":"high","Risk":"high"}`, encoded)
}
