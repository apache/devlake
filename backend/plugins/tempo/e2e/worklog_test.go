/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain the License at

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

	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/tempo/impl"
	"github.com/apache/incubator-devlake/plugins/tempo/models"
	"github.com/apache/incubator-devlake/plugins/tempo/tasks"
)

func TestWorklogDataFlow(t *testing.T) {
	var plugin impl.Tempo
	dataflowTester := e2ehelper.NewDataFlowTester(t, "tempo", plugin)

	taskData := &tasks.TempoTaskData{
		Options: &tasks.TempoOptions{
			ConnectionId: 1,
		},
	}

	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_tempo_worklogs.csv", &models.TempoWorklog{})
	dataflowTester.FlushTabler(&ticket.IssueWorklog{})
	dataflowTester.Subtask(tasks.ConvertWorklogsMeta, taskData)
	dataflowTester.VerifyTable(
		ticket.IssueWorklog{},
		"./snapshot_tables/issue_worklogs.csv",
		[]string{"id", "author_id", "time_spent_minutes", "issue_id", "started_date", "logged_date"},
	)
}

func TestTeamDataFlow(t *testing.T) {
	var plugin impl.Tempo
	dataflowTester := e2ehelper.NewDataFlowTester(t, "tempo", plugin)

	taskData := &tasks.TempoTaskData{
		Options: &tasks.TempoOptions{
			ConnectionId: 1,
		},
	}

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_tempo_api_teams.csv", "_raw_tempo_api_teams")
	dataflowTester.FlushTabler(&models.TempoTeam{})
	dataflowTester.Subtask(tasks.ExtractTeamsMeta, taskData)
	dataflowTester.VerifyTable(
		models.TempoTeam{},
		"./snapshot_tables/_tool_tempo_teams.csv",
		e2ehelper.ColumnWithRawData(
			"connection_id",
			"team_id",
			"name",
			"summary",
			"key",
		),
	)
}
