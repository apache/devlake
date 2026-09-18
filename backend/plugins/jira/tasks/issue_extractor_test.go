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

package tasks

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks/apiv2models"
)

func TestExtractIssuesEpicKeyField(t *testing.T) {
	const customField = "customfield_10014"
	mappings := &typeMappings{
		TypeIdMappings:         map[string]string{},
		StdTypeMappings:        map[string]string{},
		StandardStatusMappings: map[string]models.StatusMappings{},
	}

	tests := []struct {
		name         string
		epicKeyField string
		fieldsJSON   string
		wantEpicKey  string
	}{
		{
			name:         "string custom field",
			epicKeyField: customField,
			fieldsJSON:   fmt.Sprintf(`"%s": "EPIC-1"`, customField),
			wantEpicKey:  "EPIC-1",
		},
		{
			name:         "object custom field",
			epicKeyField: customField,
			fieldsJSON:   fmt.Sprintf(`"%s": {"key":"EPIC-2","id":"123"}`, customField),
			wantEpicKey:  "EPIC-2",
		},
		{
			name:         "unset custom field keeps legacy epic",
			epicKeyField: customField,
			fieldsJSON:   `"epic": {"key": "LEGACY-1"}`,
			wantEpicKey:  "LEGACY-1",
		},
		{
			name:         "empty EpicKeyField does not apply custom field",
			epicKeyField: "",
			fieldsJSON:   fmt.Sprintf(`"%s": "EPIC-1"`, customField),
			wantEpicKey:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := minimalIssueJSON(tt.fieldsJSON)
			var apiIssue apiv2models.Issue
			if err := json.Unmarshal(raw, &apiIssue); err != nil {
				t.Fatalf("unmarshal issue: %v", err)
			}

			data := &JiraTaskData{
				Options: &JiraOptions{
					ConnectionId: 1,
					BoardId:      1,
					ScopeConfig:  &models.JiraScopeConfig{EpicKeyField: tt.epicKeyField},
				},
			}
			results, err := extractIssues(data, mappings, &apiIssue, &api.RawData{Data: raw}, nil)
			if err != nil {
				t.Fatalf("extractIssues() error = %v", err)
			}
			issue := jiraIssueFromResults(results)
			if issue == nil {
				t.Fatal("extractIssues() did not return a JiraIssue")
			}
			if issue.EpicKey != tt.wantEpicKey {
				t.Errorf("EpicKey = %q, want %q", issue.EpicKey, tt.wantEpicKey)
			}
		})
	}
}

func minimalIssueJSON(fieldsExtra string) []byte {
	if fieldsExtra != "" {
		fieldsExtra = "," + fieldsExtra
	}
	return []byte(fmt.Sprintf(`{
		"id": "10001",
		"key": "TEST-1",
		"self": "https://example.atlassian.net/rest/agile/1.0/issue/10001",
		"fields": {
			"created": "2024-01-01T00:00:00.000+0000",
			"updated": "2024-01-01T00:00:00.000+0000",
			"summary": "test",
			"issuetype": {"id": "1", "name": "Story", "subtask": false},
			"status": {"name": "To Do", "statusCategory": {"key": "new"}}
			%s
		}
	}`, fieldsExtra))
}

func jiraIssueFromResults(results []interface{}) *models.JiraIssue {
	for _, r := range results {
		if issue, ok := r.(*models.JiraIssue); ok {
			return issue
		}
	}
	return nil
}

func TestExtractIssuesWorklog(t *testing.T) {
	mappings := &typeMappings{
		TypeIdMappings:         map[string]string{},
		StdTypeMappings:        map[string]string{},
		StandardStatusMappings: map[string]models.StatusMappings{},
	}
	data := &JiraTaskData{
		Options: &JiraOptions{
			ConnectionId: 1,
			BoardId:      1,
		},
	}

	t.Run("extracts embedded worklog when fields contain worklog", func(t *testing.T) {
		worklogJSON := `"worklog": {
			"startAt": 0,
			"maxResults": 20,
			"total": 1,
			"worklogs": [{
				"id": "1001",
				"issueId": "10001",
				"timeSpent": "1h",
				"timeSpentSeconds": 3600,
				"created": "2024-01-01T00:00:00.000+0000",
				"updated": "2024-01-01T00:00:00.000+0000",
				"started": "2024-01-01T00:00:00.000+0000"
			}]
		}`
		raw := minimalIssueJSON(worklogJSON)
		var apiIssue apiv2models.Issue
		if err := json.Unmarshal(raw, &apiIssue); err != nil {
			t.Fatalf("unmarshal issue: %v", err)
		}
		results, err := extractIssues(data, mappings, &apiIssue, &api.RawData{Data: raw}, nil)
		if err != nil {
			t.Fatalf("extractIssues() error = %v", err)
		}
		issue := jiraIssueFromResults(results)
		if issue == nil {
			t.Fatal("extractIssues() did not return a JiraIssue")
		}
		if issue.WorklogTotal != 1 {
			t.Errorf("WorklogTotal = %d, want 1", issue.WorklogTotal)
		}
		var extractedWorklogs []*models.JiraWorklog
		for _, r := range results {
			if w, ok := r.(*models.JiraWorklog); ok {
				extractedWorklogs = append(extractedWorklogs, w)
			}
		}
		if len(extractedWorklogs) != 1 {
			t.Fatalf("len(extractedWorklogs) = %d, want 1", len(extractedWorklogs))
		}
		if extractedWorklogs[0].WorklogId != "1001" {
			t.Errorf("WorklogId = %s, want 1001", extractedWorklogs[0].WorklogId)
		}
	})

	t.Run("no worklog extracted when worklog field is absent", func(t *testing.T) {
		raw := minimalIssueJSON("")
		var apiIssue apiv2models.Issue
		if err := json.Unmarshal(raw, &apiIssue); err != nil {
			t.Fatalf("unmarshal issue: %v", err)
		}
		results, err := extractIssues(data, mappings, &apiIssue, &api.RawData{Data: raw}, nil)
		if err != nil {
			t.Fatalf("extractIssues() error = %v", err)
		}
		issue := jiraIssueFromResults(results)
		if issue == nil {
			t.Fatal("extractIssues() did not return a JiraIssue")
		}
		if issue.WorklogTotal != 0 {
			t.Errorf("WorklogTotal = %d, want 0", issue.WorklogTotal)
		}
		for _, r := range results {
			if _, ok := r.(*models.JiraWorklog); ok {
				t.Errorf("unexpected JiraWorklog extracted when worklog field is absent")
			}
		}
	})
}
