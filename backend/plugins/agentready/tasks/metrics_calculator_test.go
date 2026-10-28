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
	"testing"

	"github.com/apache/devlake/plugins/agentready/models"
)

func TestCalculateMetricsFromFindings(t *testing.T) {
	score100 := 100.0
	score30 := 30.0

	findings := []*models.AgentReadyFinding{
		{Tier: 1, Status: "pass", Score: &score100, Category: "Docs", DefaultWeight: 0.5},
		{Tier: 1, Status: "fail", Score: &score30, Category: "Docs", DefaultWeight: 0.5},
		{Tier: 2, Status: "pass", Score: &score100, Category: "Security", DefaultWeight: 0.8},
		{Tier: 3, Status: "pass", Score: &score100, Category: "Quality", DefaultWeight: 0.3},
		{Tier: 3, Status: "skipped", Score: nil, Category: "Quality", DefaultWeight: 0.3},
	}

	metric, err := CalculateMetricsFromFindings(findings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metric.PassCount != 3 {
		t.Errorf("PassCount = %d, want 3", metric.PassCount)
	}
	if metric.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", metric.FailCount)
	}
	if metric.SkipCount != 1 {
		t.Errorf("SkipCount = %d, want 1", metric.SkipCount)
	}
	if metric.Tier1PassRate != 50.0 {
		t.Errorf("Tier1PassRate = %v, want 50.0", metric.Tier1PassRate)
	}
	if metric.Tier2PassRate != 100.0 {
		t.Errorf("Tier2PassRate = %v, want 100.0", metric.Tier2PassRate)
	}
	if metric.Tier3PassRate != 100.0 {
		t.Errorf("Tier3PassRate = %v, want 100.0", metric.Tier3PassRate)
	}
	if metric.Tier4PassRate != 0.0 {
		t.Errorf("Tier4PassRate = %v, want 0.0", metric.Tier4PassRate)
	}
	if metric.CategoryScores == "" {
		t.Error("CategoryScores should not be empty")
	}
}
