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
	"testing"

	"github.com/apache/devlake/plugins/cursor/models"
	"github.com/stretchr/testify/require"
)

const sampleCommitDetailResponse = `{
  "commits": [
    {
      "commitHash": "abc123def456",
      "commitSource": "ide",
      "rangeAnnotations": [
        {
          "filePath": "backend/plugins/cursor/tasks/ai_code_commits_collector.go",
          "groups": [
            {
              "conversationId": "846952e0-0221-473b-bb1d-e5d016f071b1",
              "model": "composer-2.5-fast",
              "operationType": "insert",
              "ranges": [
                {"start": 10, "end": 25},
                {"start": 42, "end": 58}
              ]
            }
          ]
        }
      ]
    }
  ],
  "conversations": [
    {
      "id": "846952e0-0221-473b-bb1d-e5d016f071b1",
      "title": "Refactor analytics module",
      "tldr": "Extracted report generation into separate functions",
      "overview": "Refactored the analytics module to improve maintainability.",
      "summaryBullets": [
        "Created dedicated report generator class",
        "Added unit tests for new functions"
      ]
    }
  ]
}`

func TestFlattenAiCodeCommitDetailResponse(t *testing.T) {
	var payload aiCodeCommitDetailResponse
	require.NoError(t, json.Unmarshal([]byte(sampleCommitDetailResponse), &payload))

	rows, err := flattenAiCodeCommitDetailResponse(1, "team", &payload)
	require.NoError(t, err)
	require.Len(t, rows, 3)

	var conversations int
	var annotations int
	for _, row := range rows {
		switch typed := row.(type) {
		case *models.CursorAiCodeConversation:
			conversations++
			require.Equal(t, "846952e0-0221-473b-bb1d-e5d016f071b1", typed.ConversationId)
			require.Equal(t, "Refactor analytics module", typed.Title)
			require.Contains(t, typed.SummaryBullets, "Created dedicated report generator class")
		case *models.CursorAiCodeRangeAnnotation:
			annotations++
			require.NotEmpty(t, typed.AnnotationId)
			require.Equal(t, "abc123def456", typed.CommitHash)
			require.Equal(t, "composer-2.5-fast", typed.Model)
			require.Equal(t, "insert", typed.OperationType)
			require.Greater(t, typed.LineCount, 0)
		default:
			t.Fatalf("unexpected row type %T", row)
		}
	}
	require.Equal(t, 1, conversations)
	require.Equal(t, 2, annotations)
}

func TestParseCommitDetailsWholeBodyResponse(t *testing.T) {
	body := []byte(sampleCommitDetailResponse)
	items, err := parseCommitDetailsWholeBodyResponseBody(body)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func parseCommitDetailsWholeBodyResponseBody(body []byte) ([]json.RawMessage, error) {
	if len(body) == 0 {
		return nil, nil
	}
	return []json.RawMessage{body}, nil
}
