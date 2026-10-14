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
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/teambition/impl"
	"github.com/apache/devlake/plugins/teambition/models"
	"github.com/apache/devlake/plugins/teambition/tasks"
	"testing"
)

func TestTeambitionTask(t *testing.T) {

	var teambition impl.Teambition
	dataflowTester := e2ehelper.NewDataFlowTester(t, "teambition", teambition)

	taskData := &tasks.TeambitionTaskData{
		Options: &tasks.TeambitionOptions{
			ConnectionId: 1,
			ProjectId:    "64132c94f0d59df1c9825ab8",
		},
	}

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_teambition_api_tasks.csv",
		"_raw_teambition_api_tasks")
	dataflowTester.FlushTabler(&models.TeambitionTask{})

	dataflowTester.FlushTabler(&models.TeambitionTaskTagTask{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_teambition_task_tag_tasks.csv",
		&models.TeambitionTaskTagTask{})

	dataflowTester.FlushTabler(&models.TeambitionProject{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_teambition_projects.csv",
		&models.TeambitionProject{})

	dataflowTester.FlushTabler(&models.TeambitionAccount{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_teambition_accounts.csv",
		&models.TeambitionAccount{})

	dataflowTester.FlushTabler(&models.TeambitionTaskScenario{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_teambition_task_scenarios.csv",
		&models.TeambitionTaskScenario{})

	dataflowTester.FlushTabler(&models.TeambitionTaskFlowStatus{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_teambition_task_flow_status.csv",
		&models.TeambitionTaskFlowStatus{})

	// verify extraction
	dataflowTester.Subtask(tasks.ExtractTasksMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.TeambitionTask{},
		e2ehelper.TableOptions{
			CSVRelPath:  "./snapshot_tables/_tool_teambition_tasks.csv",
			IgnoreTypes: []interface{}{common.NoPKModel{}},
			IgnoreFields: []string{
				"created",
				"updated",
				"start_date",
				"end_date",
				"create_time",
				"update_time",
				"date",
				"started_date",
				"due_date",
				"accomplish_time",
			},
		},
	)

	dataflowTester.FlushTabler(&ticket.IssueAssignee{})
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.FlushTabler(&ticket.SprintIssue{})
	dataflowTester.Subtask(tasks.ConvertTasksMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		ticket.Issue{},
		e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/issues.csv",
			IgnoreFields: []string{"created_date", "logged_date", "started_date", "updated_date", "resolution_date"},
			IgnoreTypes:  []interface{}{domainlayer.DomainEntity{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		ticket.IssueAssignee{},
		e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/issue_assignees.csv",
			IgnoreFields: []string{"created_date", "logged_date", "started_date", "updated_date", "resolution_date"},
			IgnoreTypes:  []interface{}{domainlayer.DomainEntity{}},
		},
	)
}
