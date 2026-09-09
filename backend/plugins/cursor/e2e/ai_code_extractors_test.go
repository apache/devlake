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
	"time"

	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/runner"
	"github.com/apache/devlake/helpers/e2ehelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/impl"
	"github.com/apache/devlake/plugins/cursor/models"
	"github.com/apache/devlake/plugins/cursor/tasks"
)

func TestCursorAiCodeExtractorsDataFlow(t *testing.T) {
	cfg := config.GetConfig()
	dbUrl := cfg.GetString("E2E_DB_URL")
	if dbUrl == "" {
		t.Skip("skipping e2e test: E2E_DB_URL is not set")
	}
	if err := runner.CheckDbConnection(dbUrl, 10*time.Second); err != nil {
		t.Skipf("skipping e2e test: cannot connect to E2E_DB_URL: %v", err)
	}

	var cursorPlugin impl.Cursor
	dataflowTester := e2ehelper.NewDataFlowTester(t, "cursor", cursorPlugin)

	taskData := &tasks.CursorTaskData{
		Options: &tasks.CursorOptions{
			ConnectionId: 1,
			ScopeId:      "team",
		},
		Connection: &models.CursorConnection{
			CursorConn: models.CursorConn{
				RestConnection: helper.RestConnection{Endpoint: models.DefaultEndpoint},
				KeyTier:        models.KeyTierEnterprise,
			},
		},
	}

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_cursor_ai_code_commits.csv", "_raw_cursor_ai_code_commits")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_cursor_ai_code_changes.csv", "_raw_cursor_ai_code_changes")

	dataflowTester.FlushTabler(&models.CursorAiCodeCommit{})
	dataflowTester.FlushTabler(&models.CursorAiCodeChange{})

	dataflowTester.Subtask(tasks.ExtractAiCodeCommitsMeta, taskData)
	dataflowTester.Subtask(tasks.ExtractAiCodeChangesMeta, taskData)

	dataflowTester.VerifyTableWithOptions(&models.CursorAiCodeCommit{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_cursor_ai_code_commits.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	dataflowTester.VerifyTableWithOptions(&models.CursorAiCodeChange{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_cursor_ai_code_changes.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}

func TestCursorAiCodeExtractorsSkipForTeamKey(t *testing.T) {
	cfg := config.GetConfig()
	dbUrl := cfg.GetString("E2E_DB_URL")
	if dbUrl == "" {
		t.Skip("skipping e2e test: E2E_DB_URL is not set")
	}
	if err := runner.CheckDbConnection(dbUrl, 10*time.Second); err != nil {
		t.Skipf("skipping e2e test: cannot connect to E2E_DB_URL: %v", err)
	}

	var cursorPlugin impl.Cursor
	dataflowTester := e2ehelper.NewDataFlowTester(t, "cursor", cursorPlugin)

	taskData := &tasks.CursorTaskData{
		Options: &tasks.CursorOptions{
			ConnectionId: 1,
			ScopeId:      "team",
		},
		Connection: &models.CursorConnection{
			CursorConn: models.CursorConn{
				RestConnection: helper.RestConnection{Endpoint: models.DefaultEndpoint},
				KeyTier:        models.KeyTierTeam,
			},
		},
	}

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_cursor_ai_code_commits.csv", "_raw_cursor_ai_code_commits")
	dataflowTester.FlushTabler(&models.CursorAiCodeCommit{})

	dataflowTester.Subtask(tasks.ExtractAiCodeCommitsMeta, taskData)

	var count int64
	if err := dataflowTester.Db.Model(&models.CursorAiCodeCommit{}).Count(&count).Error; err != nil {
		t.Fatalf("count commits: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 commits for team key tier, got %d", count)
	}
}
