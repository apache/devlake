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
	"github.com/apache/incubator-devlake/plugins/linear/impl"
	"github.com/apache/incubator-devlake/plugins/linear/models"
	"github.com/apache/incubator-devlake/plugins/linear/tasks"
)

// TestLinearBoardDataFlow verifies that a Linear team scope is converted into a
// domain ticket.Board, keyed identically to the board_id that board_issues
// already reference (boardIdGen over LinearTeam). Without this, the boards table
// is empty and board-scoped dashboards return no data.
func TestLinearBoardDataFlow(t *testing.T) {
	var linear impl.Linear
	dataflowTester := e2ehelper.NewDataFlowTester(t, "linear", linear)

	taskData := &tasks.LinearTaskData{
		Options: &tasks.LinearOptions{
			ConnectionId: 1,
			TeamId:       "team-1",
		},
	}

	// the team scope lives in _tool_linear_teams (populated via the scope API)
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_linear_teams.csv", &models.LinearTeam{})

	// convert: team scope -> domain board
	dataflowTester.FlushTabler(&ticket.Board{})
	dataflowTester.Subtask(tasks.ConvertTeamsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(ticket.Board{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/boards.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
