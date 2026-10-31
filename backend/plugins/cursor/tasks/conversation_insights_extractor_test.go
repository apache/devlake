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
)

const sampleConversationInsightsResponse = `{
  "data": {
    "intents": {
      "distribution": [
        {"intent": "Write Code", "count": 18},
        {"intent": "Ask", "count": 7}
      ],
      "topValues": [
        {"intent": "Write Code", "count": 18}
      ],
      "timeSeries": [
        {"date": "2026-03-01", "intent": "Ask", "count": 2},
        {"date": "2026-03-02", "intent": "Write Code", "count": 6}
      ],
      "subcategories": {
        "askMode": [
          {"subcategory": "error_fix", "count": 4}
        ]
      }
    },
    "complexity": {
      "distribution": [
        {"complexity": "high", "count": 12}
      ],
      "timeSeries": [
        {"date": "2026-03-02", "complexity": "high", "count": 5}
      ]
    },
    "categories": {
      "distribution": [
        {"category": "New Features", "count": 9}
      ]
    },
    "guidanceLevels": {
      "distribution": [
        {"guidanceLevel": "high", "count": 8}
      ]
    },
    "workTypes": {
      "distribution": [
        {"workType": "new_feature", "count": 9}
      ]
    }
  },
  "params": {
    "metric": "conversation-insights",
    "startDate": "2026-03-01",
    "endDate": "2026-03-07"
  }
}`

func TestExtractConversationInsightsPayload(t *testing.T) {
	var payload conversationInsightsPayload
	if err := json.Unmarshal([]byte(sampleConversationInsightsResponse), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	periodStart, periodEnd, err := parseConversationInsightPeriod(payload.Params.StartDate, payload.Params.EndDate)
	if err != nil {
		t.Fatalf("parse period: %v", err)
	}

	specs := []insightSpec{
		{
			insightType:   "intents",
			valueField:    "intent",
			timeField:     "intent",
			block:         payload.Data.Intents,
			includeTop:    true,
			includeSubcat: true,
		},
		{
			insightType: "complexity",
			valueField:  "complexity",
			timeField:   "complexity",
			block:       payload.Data.Complexity,
		},
		{
			insightType: "categories",
			valueField:  "category",
			timeField:   "category",
			block:       payload.Data.Categories,
		},
		{
			insightType: "guidanceLevels",
			valueField:  "guidanceLevel",
			timeField:   "guidanceLevel",
			block:       payload.Data.GuidanceLevels,
		},
		{
			insightType: "workTypes",
			valueField:  "workType",
			timeField:   "workType",
			block:       payload.Data.WorkTypes,
		},
	}

	var total int
	for _, spec := range specs {
		rows, extractErr := flattenConversationInsightBlock(1, "team", periodStart, periodEnd, spec)
		if extractErr != nil {
			t.Fatalf("flatten %s: %v", spec.insightType, extractErr)
		}
		total += len(rows)
	}
	if total != 0 && total < 10 {
		t.Fatalf("expected at least 10 insight rows, got %d", total)
	}
}

func TestComputeConversationInsightRecordIdStable(t *testing.T) {
	start, err := parseConversationInsightDate("2026-03-01")
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}
	end, err := parseConversationInsightDate("2026-03-07")
	if err != nil {
		t.Fatalf("parse end: %v", err)
	}
	first := computeConversationInsightRecordId(start, end, "intents", "distribution", "Write Code", "", "")
	second := computeConversationInsightRecordId(start, end, "intents", "distribution", "Write Code", "", "")
	if first != second {
		t.Fatalf("record id should be stable")
	}
	if first == "" {
		t.Fatal("record id should not be empty")
	}
}
