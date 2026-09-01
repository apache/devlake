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

	"github.com/apache/incubator-devlake/core/config"
	"github.com/apache/incubator-devlake/core/runner"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/cursor/impl"
	"github.com/apache/incubator-devlake/plugins/cursor/models"
	"github.com/apache/incubator-devlake/plugins/cursor/tasks"
)

func TestCursorConversationInsightsExtractorsDataFlow(t *testing.T) {
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
				RestConnection:            helper.RestConnection{Endpoint: models.DefaultEndpoint},
				KeyTier:                   models.KeyTierEnterprise,
				HasConversationInsights:   true,
			},
		},
	}

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_cursor_conversation_insights.csv", "_raw_cursor_conversation_insights")
	dataflowTester.FlushTabler(&models.CursorConversationInsight{})

	dataflowTester.Subtask(tasks.ExtractConversationInsightsMeta, taskData)

	var count int64
	if err := dataflowTester.Db.Model(&models.CursorConversationInsight{}).Count(&count).Error; err != nil {
		t.Fatalf("count insights: %v", err)
	}
	if count < 10 {
		t.Fatalf("expected at least 10 conversation insight rows, got %d", count)
	}

	var intentCount int64
	if err := dataflowTester.Db.Model(&models.CursorConversationInsight{}).
		Where("insight_type = ? AND dimension_value = ?", models.ConversationInsightTypeIntents, "Write Code").
		Count(&intentCount).Error; err != nil {
		t.Fatalf("count intent rows: %v", err)
	}
	if intentCount == 0 {
		t.Fatal("expected Write Code intent rows")
	}
}

func TestCursorConversationInsightsExtractorsSkipWithoutAccess(t *testing.T) {
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
				RestConnection:          helper.RestConnection{Endpoint: models.DefaultEndpoint},
				KeyTier:                 models.KeyTierEnterprise,
				HasConversationInsights: false,
			},
		},
	}

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_cursor_conversation_insights.csv", "_raw_cursor_conversation_insights")
	dataflowTester.FlushTabler(&models.CursorConversationInsight{})

	dataflowTester.Subtask(tasks.ExtractConversationInsightsMeta, taskData)

	var count int64
	if err := dataflowTester.Db.Model(&models.CursorConversationInsight{}).Count(&count).Error; err != nil {
		t.Fatalf("count insights: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 insights without conversation insights access, got %d", count)
	}
}
