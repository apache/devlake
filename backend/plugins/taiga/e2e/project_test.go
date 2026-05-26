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
	"github.com/apache/incubator-devlake/plugins/taiga/impl"
	"github.com/apache/incubator-devlake/plugins/taiga/models"
	"github.com/apache/incubator-devlake/plugins/taiga/tasks"
)

// TestTaigaProjectDataFlow verifies the full extract → convert pipeline for
// Taiga projects: raw API JSON → _tool_taiga_projects → ticket.Board.
//
// To regenerate snapshot CSVs from actual output, temporarily replace
// VerifyTableWithOptions with CreateSnapshot for each table.
func TestTaigaProjectDataFlow(t *testing.T) {
	var taiga impl.Taiga
	dataflowTester := e2ehelper.NewDataFlowTester(t, "taiga", taiga)

	taskData := &tasks.TaigaTaskData{
		Options: &tasks.TaigaOptions{
			ConnectionId: 1,
			ProjectId:    1,
		},
	}

	// ── Extraction: raw JSON → _tool_taiga_projects ─────────────────────────
	dataflowTester.ImportCsvIntoRawTable(
		"./raw_tables/_raw_taiga_api_projects.csv",
		"_raw_taiga_api_projects",
	)

	dataflowTester.FlushTabler(&models.TaigaProject{})
	dataflowTester.Subtask(tasks.ExtractProjectsMeta, taskData)
	// Verify all tool-layer columns, ignoring NoPKModel timestamps and raw-data provenance.
	dataflowTester.VerifyTableWithOptions(models.TaigaProject{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_taiga_projects.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// ── Conversion: _tool_taiga_projects → ticket.Board ──────────────────────
	dataflowTester.FlushTabler(&ticket.Board{})
	dataflowTester.Subtask(tasks.ConvertProjectsMeta, taskData)
	// Verify all domain-layer columns, ignoring NoPKModel timestamps and raw-data provenance.
	dataflowTester.VerifyTableWithOptions(ticket.Board{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/boards.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
