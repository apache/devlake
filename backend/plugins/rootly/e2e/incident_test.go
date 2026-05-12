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

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/rootly/impl"
	"github.com/apache/incubator-devlake/plugins/rootly/models"
	"github.com/apache/incubator-devlake/plugins/rootly/tasks"
	"github.com/stretchr/testify/require"
)

func TestIncidentDataFlow(t *testing.T) {
	var plugin impl.Rootly
	dataflowTester := e2ehelper.NewDataFlowTester(t, "rootly", plugin)
	options := tasks.RootlyOptions{
		ConnectionId: 1,
		ServiceId:    "svc_01",
		ServiceName:  "Payments",
	}
	taskData := &tasks.RootlyTaskData{
		Options: &options,
	}

	// Seed the service scope as a prereq: the service is the scope unit,
	// not test data, so we populate _tool_rootly_services directly
	// instead of running the services extractor. Mirrors the pagerduty
	// e2e pattern.
	dataflowTester.FlushTabler(&models.Service{})
	service := models.Service{
		Scope: common.Scope{
			ConnectionId: options.ConnectionId,
		},
		Url:  fmt.Sprintf("https://rootly.com/account/services/%s", options.ServiceId),
		Id:   options.ServiceId,
		Name: options.ServiceName,
	}
	require.NoError(t, dataflowTester.Dal.CreateOrUpdate(&service))

	// Import the raw incidents fixture that drives the extractor.
	dataflowTester.ImportCsvIntoRawTable(
		"./raw_tables/_raw_rootly_incidents.csv",
		"_raw_rootly_incidents",
	)

	// Extract incidents. The extractor writes to _tool_rootly_incidents
	// and _tool_rootly_users (inline users from nested attributes).
	dataflowTester.FlushTabler(&models.Incident{})
	dataflowTester.FlushTabler(&models.User{})
	dataflowTester.Subtask(tasks.ExtractIncidentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.Service{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_rootly_services.csv",
			IgnoreTypes: []any{common.Scope{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.Incident{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_rootly_incidents.csv",
			IgnoreTypes: []any{common.NoPKModel{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.User{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_rootly_users.csv",
			IgnoreTypes: []any{common.NoPKModel{}},
		},
	)

	// Convert: services -> boards, incidents -> issues + assignees + board
	// membership.
	dataflowTester.FlushTabler(&ticket.Board{})
	dataflowTester.Subtask(tasks.ConvertServicesMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		ticket.Board{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/boards.csv",
			IgnoreTypes: []any{common.NoPKModel{}},
		},
	)

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
	dataflowTester.VerifyTableWithOptions(
		ticket.IssueAssignee{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/issue_assignees.csv",
			IgnoreTypes: []any{common.NoPKModel{}},
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
