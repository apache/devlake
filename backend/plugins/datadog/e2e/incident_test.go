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
	"github.com/apache/devlake/plugins/datadog/impl"
	"github.com/apache/devlake/plugins/datadog/models"
	"github.com/apache/devlake/plugins/datadog/tasks"
)

func TestIncidentDataFlow(t *testing.T) {
	var plugin impl.Datadog
	dataflowTester := e2ehelper.NewDataFlowTester(t, "datadog", plugin)
	options := tasks.DatadogOptions{
		ConnectionId:     1,
		IncidentTypeId:   "type_01",
		IncidentTypeName: "General Incident",
		// The organization decides which custom field carries the component;
		// here it is Datadog's own multi-select of affected services.
		ScopeConfig: &models.DatadogScopeConfig{ComponentField: "services"},
	}
	taskData := &tasks.DatadogTaskData{
		Options: &options,
		WebUrl:  "https://app.datadoghq.com",
	}

	// incident types, the collection scope
	dataflowTester.ImportCsvIntoRawTable(
		"./raw_tables/_raw_datadog_incident_types.csv",
		"_raw_datadog_incident_types",
	)
	dataflowTester.FlushTabler(&models.IncidentType{})
	dataflowTester.Subtask(tasks.ExtractIncidentTypesMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.IncidentType{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_datadog_incident_types.csv",
			IgnoreTypes: []any{common.Scope{}},
		},
	)

	// incidents
	dataflowTester.ImportCsvIntoRawTable(
		"./raw_tables/_raw_datadog_incidents.csv",
		"_raw_datadog_incidents",
	)
	dataflowTester.FlushTabler(&models.Incident{})
	dataflowTester.Subtask(tasks.ExtractIncidentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.Incident{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_datadog_incidents.csv",
			IgnoreTypes: []any{common.NoPKModel{}},
		},
	)

	// conversion
	dataflowTester.FlushTabler(&ticket.Board{})
	dataflowTester.Subtask(tasks.ConvertIncidentTypesMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		ticket.Board{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/boards.csv",
			IgnoreTypes: []any{common.NoPKModel{}},
		},
	)

	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.Subtask(tasks.ConvertIncidentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		ticket.Issue{},
		e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/issues.csv",
			IgnoreTypes:  []any{common.NoPKModel{}},
			IgnoreFields: []string{"original_project"},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		ticket.BoardIssue{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/board_issues.csv",
			IgnoreTypes: []any{common.NoPKModel{}},
		},
	)
}
