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
	"time"

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/pagerduty/impl"
	"github.com/apache/devlake/plugins/pagerduty/models"
	"github.com/apache/devlake/plugins/pagerduty/tasks"
	"github.com/stretchr/testify/require"
)

func TestIncidentDataFlow(t *testing.T) {
	var plugin impl.PagerDuty
	dataflowTester := e2ehelper.NewDataFlowTester(t, "pagerduty", plugin)
	options := tasks.PagerDutyOptions{
		ConnectionId: 1,
		ServiceId:    "PIKL83L",
		ServiceName:  "DevService",
		ScopeConfig:  nil,
	}
	taskData := &tasks.PagerDutyTaskData{
		Options: &options,
	}

	dataflowTester.FlushTabler(&models.Service{})
	// tx-rule
	service := models.Service{
		Scope: common.Scope{
			ConnectionId: options.ConnectionId,
		},
		Url:  fmt.Sprintf("https://keon-test.pagerduty.com/service-directory/%s", options.ServiceId),
		Id:   options.ServiceId,
		Name: options.ServiceName,
	}
	// scope
	require.NoError(t, dataflowTester.Dal.CreateOrUpdate(&service))

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_pagerduty_incidents.csv", "_raw_pagerduty_incidents")

	// verify worklog extraction
	dataflowTester.FlushTabler(&models.Incident{})
	dataflowTester.FlushTabler(&models.User{})
	dataflowTester.FlushTabler(&models.Assignment{})
	dataflowTester.Subtask(tasks.ExtractIncidentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.Incident{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_pagerduty_incidents.csv",
			IgnoreTypes: []any{common.Model{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.User{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_pagerduty_users.csv",
			IgnoreTypes: []any{common.Model{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.Assignment{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_pagerduty_assignments.csv",
			IgnoreTypes: []any{common.Model{}},
		},
	)
	// another connection (a different PagerDuty account) reuses the incident
	// numbers and has a user with the same id; none of it may leak into the
	// issues of connection 1
	otherConnectionAssignedAt := time.Date(2022, 11, 3, 8, 0, 0, 0, time.UTC)
	require.NoError(t, dataflowTester.Dal.CreateOrUpdate([]*models.Assignment{
		{ConnectionId: 2, UserId: "PCONN2A", IncidentNumber: 5, AssignedAt: otherConnectionAssignedAt},
		{ConnectionId: 2, UserId: "PCONN2B", IncidentNumber: 6, AssignedAt: otherConnectionAssignedAt},
	}))
	require.NoError(t, dataflowTester.Dal.CreateOrUpdate([]*models.User{
		{ConnectionId: 2, Id: "PCONN2A", Name: "Other Account User A"},
		{ConnectionId: 2, Id: "PCONN2B", Name: "Other Account User B"},
		{ConnectionId: 2, Id: "P25K520", Name: "Other Account Kian"},
	}))
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
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
	dataflowTester.VerifyTableWithOptions(ticket.IssueAssignee{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issue_assignees.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
