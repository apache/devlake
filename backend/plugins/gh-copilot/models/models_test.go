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

package models

import "testing"

func TestGetTablesInfo(t *testing.T) {
	tables := GetTablesInfo()
	expected := map[string]bool{
		(&GhCopilotConnection{}).TableName():                   false,
		(&GhCopilotScope{}).TableName():                        false,
		(&GhCopilotScopeConfig{}).TableName():                  false,
		(&GhCopilotOrgMetrics{}).TableName():                   false,
		(&GhCopilotLanguageMetrics{}).TableName():              false,
		(&GhCopilotEnterpriseDailyMetrics{}).TableName():       false,
		(&GhCopilotMetricsByIde{}).TableName():                 false,
		(&GhCopilotMetricsByFeature{}).TableName():             false,
		(&GhCopilotMetricsByLanguageFeature{}).TableName():     false,
		(&GhCopilotMetricsByLanguageModel{}).TableName():       false,
		(&GhCopilotMetricsByModelFeature{}).TableName():        false,
		(&GhCopilotUserDailyMetrics{}).TableName():             false,
		(&GhCopilotUserMetricsByIde{}).TableName():             false,
		(&GhCopilotUserMetricsByFeature{}).TableName():         false,
		(&GhCopilotUserMetricsByLanguageFeature{}).TableName(): false,
		(&GhCopilotUserMetricsByLanguageModel{}).TableName():   false,
		(&GhCopilotUserMetricsByModelFeature{}).TableName():    false,
		(&GhCopilotSeat{}).TableName():                         false,
	}

	if len(tables) != len(expected) {
		t.Fatalf("unexpected number of tables: want %d, got %d", len(expected), len(tables))
	}

	for _, table := range tables {
		tableName := table.TableName()
		if _, ok := expected[tableName]; !ok {
			t.Fatalf("unexpected table registered: %s", tableName)
		}
		expected[tableName] = true
	}

	for name, seen := range expected {
		if !seen {
			t.Fatalf("table not registered: %s", name)
		}
	}
}
