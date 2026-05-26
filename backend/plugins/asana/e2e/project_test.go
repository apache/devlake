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

package e2e

import (
	"testing"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/asana/impl"
	"github.com/apache/incubator-devlake/plugins/asana/models"
	"github.com/apache/incubator-devlake/plugins/asana/tasks"
)

func TestAsanaProjectDataFlow(t *testing.T) {
	var asana impl.Asana
	dataflowTester := e2ehelper.NewDataFlowTester(t, "asana", asana)

	taskData := &tasks.AsanaTaskData{
		Options: &tasks.AsanaOptions{
			ConnectionId: 1,
			ProjectId:    "1234567890",
		},
	}

	// Import raw data for projects
	dataflowTester.ImportCsvIntoRawTable(rawTablePath("_raw_asana_projects.csv"), "_raw_asana_projects")

	// Verify project extraction
	dataflowTester.FlushTabler(&models.AsanaProject{})
	dataflowTester.Subtask(tasks.ExtractProjectMeta, taskData)

	dataflowTester.VerifyTableWithOptions(models.AsanaProject{}, e2ehelper.TableOptions{
		CSVRelPath:  snapshotPath("_tool_asana_projects.csv"),
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// Verify project conversion to domain layer board
	dataflowTester.FlushTabler(&ticket.Board{})
	dataflowTester.Subtask(tasks.ConvertProjectMeta, taskData)

	dataflowTester.VerifyTable(
		ticket.Board{},
		snapshotPath("boards.csv"),
		[]string{
			"id",
			"name",
			"description",
			"url",
			"created_date",
		},
	)
}

func TestAsanaProjectWithScopeConfig(t *testing.T) {
	var asana impl.Asana
	dataflowTester := e2ehelper.NewDataFlowTester(t, "asana", asana)

	taskData := &tasks.AsanaTaskData{
		Options: &tasks.AsanaOptions{
			ConnectionId:  1,
			ProjectId:     "1234567890",
			ScopeConfigId: 1,
		},
	}

	// Import project with scope config association
	dataflowTester.ImportCsvIntoRawTable(rawTablePath("_raw_asana_projects.csv"), "_raw_asana_projects")
	dataflowTester.ImportCsvIntoTabler(snapshotPath("_tool_asana_scope_configs.csv"), &models.AsanaScopeConfig{})

	// Extract project
	dataflowTester.FlushTabler(&models.AsanaProject{})
	dataflowTester.Subtask(tasks.ExtractProjectMeta, taskData)

	// Verify project has scope_config_id
	dataflowTester.VerifyTableWithOptions(models.AsanaProject{}, e2ehelper.TableOptions{
		CSVRelPath:  snapshotPath("_tool_asana_projects_with_scope_config.csv"),
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
