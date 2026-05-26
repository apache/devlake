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

// TestTaigaIssueDataFlow verifies the full extract → convert pipeline for
// Taiga issues (bugs/enhancements/questions): raw API JSON →
// _tool_taiga_issues → ticket.Issue with correct DevLake type mapping.
// Specifically checks that "Bug" → BUG, "Enhancement" → REQUIREMENT.
//
// To regenerate snapshot CSVs from actual output, temporarily replace
// VerifyTableWithOptions with CreateSnapshot for each table.
func TestTaigaIssueDataFlow(t *testing.T) {
	var taiga impl.Taiga
	dataflowTester := e2ehelper.NewDataFlowTester(t, "taiga", taiga)

	taskData := &tasks.TaigaTaskData{
		Options: &tasks.TaigaOptions{
			ConnectionId: 1,
			ProjectId:    1,
		},
	}

	// ── Extraction: raw JSON → _tool_taiga_issues ────────────────────────────
	dataflowTester.ImportCsvIntoRawTable(
		"./raw_tables/_raw_taiga_api_issues.csv",
		"_raw_taiga_api_issues",
	)

	dataflowTester.FlushTabler(&models.TaigaIssue{})
	dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)
	// Verify all tool-layer columns, ignoring NoPKModel timestamps and raw-data provenance.
	dataflowTester.VerifyTableWithOptions(models.TaigaIssue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_taiga_issues.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// ── Conversion: _tool_taiga_issues → ticket.Issue / ticket.BoardIssue ────
	// Key assertion: taigaIssueTypeToDevLake mapping is exercised (Bug→BUG, Enhancement→REQUIREMENT).
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.Subtask(tasks.ConvertIssuesMeta, taskData)
	// Verify all domain-layer columns, ignoring NoPKModel timestamps and raw-data provenance.
	dataflowTester.VerifyTableWithOptions(ticket.Issue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issues_from_issues.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTableWithOptions(ticket.BoardIssue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/board_issues_from_issues.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
