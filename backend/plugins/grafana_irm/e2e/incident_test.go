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

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/grafana_irm/impl"
	"github.com/apache/devlake/plugins/grafana_irm/models"
	"github.com/apache/devlake/plugins/grafana_irm/tasks"
	"github.com/stretchr/testify/require"
)

// The raw fixture is not hand-authored: it's the exact (compacted) JSON
// captured live from a real Grafana Cloud dev stack — the four real
// (non-drill) incidents present there as of 2026-09-20. See
// grafana_irm_plan.md §9:
//   - `4`: active, Major, one label (team_name:platform), no assignment
//   - `5`: resolved, Minor, no labels, no assignment
//   - `6`: active, Critical, two labels (service_name:orders-api,
//     team_name:platform), one real role assignment (commander)
//   - `7`: resolved, Minor, one label (team_name:payments, a different team
//     than 4/6), no assignment
//
// This mix exercises every field the extractor/converter touch: multiple
// severities, both statuses (with a real resolution timestamp), zero/one/two
// labels, and a real (non-placeholder) assignment — not just the "happy path"
// of a single unlabeled, unassigned incident.
func TestIncidentDataFlow(t *testing.T) {
	var plugin impl.GrafanaIrm
	dataflowTester := e2ehelper.NewDataFlowTester(t, "grafana_irm", plugin)

	options := tasks.GrafanaIrmOptions{
		ConnectionId: 1,
		ScopeId:      "default",
		ScopeConfig:  &models.GrafanaIrmScopeConfig{},
	}
	taskData := &tasks.GrafanaIrmTaskData{
		Options: &options,
		Connection: &models.GrafanaIrmConnection{
			GrafanaIrmConn: models.GrafanaIrmConn{
				RestConnection: helper.RestConnection{
					Endpoint: "https://grafana-irm-plugin-dev.grafana.net/",
				},
			},
		},
	}

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable(
		"./raw_tables/_raw_grafana_irm_incidents.csv",
		"_raw_grafana_irm_incidents",
	)

	// verify extraction
	dataflowTester.FlushTabler(&models.Incident{})
	dataflowTester.FlushTabler(&models.IncidentLabel{})
	dataflowTester.FlushTabler(&models.IncidentAssignment{})
	dataflowTester.Subtask(tasks.ExtractIncidentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.Incident{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_grafana_irm_incidents.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.IncidentLabel{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_grafana_irm_incident_labels.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.IncidentAssignment{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_grafana_irm_incident_assignments.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
		},
	)

	// verify conversion
	dataflowTester.FlushTabler(&models.GrafanaIrmScope{})
	dataflowTester.FlushTabler(&ticket.Board{})
	scope := models.GrafanaIrmScope{
		Scope: common.Scope{
			ConnectionId: options.ConnectionId,
		},
		Id:   options.ScopeId,
		Name: "All Incidents",
	}
	require.NoError(t, dataflowTester.Dal.CreateOrUpdate(&scope))
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.IssueLabel{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.Subtask(tasks.ConvertIncidentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		ticket.Issue{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/issues.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		ticket.BoardIssue{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/board_issues.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		ticket.IssueLabel{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/issue_labels.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		ticket.IssueAssignee{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/issue_assignees.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
		},
	)
}
