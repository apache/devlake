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
	"fmt"
	"testing"

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/opsgenie/impl"
	"github.com/apache/devlake/plugins/opsgenie/models"
	"github.com/apache/devlake/plugins/opsgenie/tasks"
	"github.com/stretchr/testify/require"
)

func TestIncidentDataFlow(t *testing.T) {
	var plugin impl.Opsgenie
	dataflowTester := e2ehelper.NewDataFlowTester(t, "opsgenie", plugin)
	options := tasks.OpsgenieOptions{
		ConnectionId: 1,
		ServiceId:    "695bce3d-4621-4630-8ae1-24eb89c22d6e",
		ServiceName:  "TestService",
		Tasks:        nil,
	}
	taskData := &tasks.OpsgenieTaskData{
		Options: &options,
	}

	dataflowTester.FlushTabler(&models.Service{})

	service := models.Service{
		Url:  fmt.Sprintf("https://sandesvitor.app.opsgenie.com/service/%s", options.ServiceId),
		Id:   options.ServiceId,
		Name: options.ServiceName,
	}
	// scope
	require.NoError(t, dataflowTester.Dal.CreateOrUpdate(&service))

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_opsgenie_incidents.csv", "_raw_opsgenie_incidents")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_opsgenie_users.csv", "_raw_opsgenie_users")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_opsgenie_teams.csv", "_raw_opsgenie_teams")

	dataflowTester.FlushTabler(&models.User{})
	dataflowTester.FlushTabler(&models.Team{})
	dataflowTester.FlushTabler(&models.Incident{})
	dataflowTester.FlushTabler(&models.Responder{})
	dataflowTester.FlushTabler(&models.Assignment{})

	dataflowTester.FlushTabler(crossdomain.User{})
	dataflowTester.FlushTabler(crossdomain.Team{})
	dataflowTester.Subtask(tasks.ExtractUsersMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.User{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_opsgenie_users.csv",
			IgnoreTypes: []any{common.Model{}},
		},
	)
	dataflowTester.Subtask(tasks.ConvertUsersMeta, taskData)
	dataflowTester.VerifyTableWithOptions(crossdomain.User{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/users.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	dataflowTester.Subtask(tasks.ExtractTeamsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.Team{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_opsgenie_teams.csv",
			IgnoreTypes: []any{common.Model{}},
		},
	)
	dataflowTester.Subtask(tasks.ConvertTeamsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(crossdomain.Team{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/teams.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	dataflowTester.Subtask(tasks.ExtractIncidentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.Incident{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_opsgenie_incidents.csv",
			IgnoreTypes: []any{common.Model{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.Responder{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_opsgenie_responders.csv",
			IgnoreTypes: []any{common.Model{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.Assignment{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_opsgenie_assignments.csv",
			IgnoreTypes: []any{common.Model{}},
		},
	)
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.Subtask(tasks.ConvertIncidentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		ticket.Issue{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/issues.csv",
			IgnoreTypes: []any{common.NoPKModel{}},
			IgnoreFields: []string{
				"creator_id",
				"creator_name",
				"assignee_id",
				"assignee_name",
				"severity",
				"component",
				"original_project",
			},
		},
	)
	dataflowTester.VerifyTableWithOptions(ticket.IssueAssignee{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issue_assignees.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
