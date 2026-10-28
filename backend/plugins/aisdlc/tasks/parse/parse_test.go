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

package parse

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClassifyTimeKeepsDateOnly(t *testing.T) {
	entry := ClassifyTime("2026-07-21")
	if entry.Precision != "date" || entry.TimezoneStatus != "unspecified" || entry.Normalized != "" {
		t.Fatalf("date-only time = %+v", entry)
	}
	if entry.Normalized == "2026-07-21T00:00:00Z" {
		t.Fatal("date-only value was converted to midnight UTC")
	}
}

func TestClassifyTimeKeepsTimezoneFree(t *testing.T) {
	entry := ClassifyTime("2026-07-25T01:55:16.703430")
	if entry.Precision != "second" || entry.Normalized != "" || entry.TimezoneStatus != "unspecified" {
		t.Fatalf("timezone-free time = %+v", entry)
	}
	folder := ClassifyTime("20260719-041307")
	if folder.Precision != "second" || folder.Normalized != "" {
		t.Fatalf("folder time = %+v", folder)
	}
	zulu := ClassifyTime("2026-07-21T15:37:59Z")
	if zulu.Normalized != "2026-07-21T15:37:59Z" || zulu.TimezoneStatus != "utc" {
		t.Fatalf("zulu time = %+v", zulu)
	}
}

func TestSelectPaths(t *testing.T) {
	entries := []TreeEntry{
		{Path: "RHAIRFE/current/RHAIRFE-1.result.md", Type: "blob"},
		{Path: "RHAIRFE/20260719-041307/scores.csv", Type: "blob"},
		{Path: "RHAIRFE/20260719-041307/RHAIRFE-2402.result.md", Type: "blob"},
		{Path: "RHAIRFE/20260719-041307", Type: "tree"},
		{Path: "RHAISTRAT/current/pipeline-data.json", Type: "blob", Mode: "120000"},
		{Path: "RHAISTRAT/20260725-015518/pipeline-data.json", Type: "blob"},
		{Path: "RHAISTRAT/20260725-015518/strat-tasks/RHAISTRAT-1.md", Type: "blob"},
		{Path: "RHAISTRAT/20260414-180837/strat-tasks/STRAT-1397.md", Type: "blob"},
		{Path: "RHAISTRAT/summary.json", Type: "blob"},
	}
	rfe := SelectPaths(ParserRFE, "", entries)
	if len(rfe) != 1 || rfe[0].Path != "RHAIRFE/20260719-041307/RHAIRFE-2402.result.md" {
		t.Fatalf("rfe paths = %+v", rfe)
	}
	strat := SelectPaths(ParserStrat, "RHAISTRAT", entries)
	if len(strat) != 2 {
		t.Fatalf("strat paths = %+v", strat)
	}
	for _, entry := range strat {
		if entry.Path == "RHAISTRAT/20260725-015518/strat-tasks/RHAISTRAT-1.md" {
			t.Fatal("strat-tasks kept beside pipeline-data.json")
		}
	}
}

func TestParseRFEFixture(t *testing.T) {
	file := readFixture(t, "rfe/RHAIRFE/20260719-041307/RHAIRFE-2402.result.md")
	file.BlobID = "blob-1"
	file.CommitSHA = "commit-1"
	cfg := testConfig(ParserRFE)
	firstAt := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	secondAt := firstAt.Add(time.Hour)
	file.CollectedAt = firstAt
	first := Parse(cfg, []File{file})
	file.CollectedAt = secondAt
	second := Parse(cfg, []File{file})
	if len(first.FileErrors) != 0 {
		t.Fatal(first.FileErrors)
	}
	if len(first.Observations) != 1 || len(first.Executions) != 0 {
		t.Fatalf("observations=%d executions=%d", len(first.Observations), len(first.Executions))
	}
	obs := first.Observations[0]
	if obs.Kind != KindAssessment || obs.SubjectKey != "RHAIRFE-2402" {
		t.Fatalf("observation = %+v", obs)
	}
	if obs.AssessmentTotal == nil || *obs.AssessmentTotal != 10 || obs.AssessmentScale == nil || *obs.AssessmentScale != 10 || obs.RawVerdict != "PASS" {
		t.Fatalf("assessment = total %v scale %v verdict %s", obs.AssessmentTotal, obs.AssessmentScale, obs.RawVerdict)
	}
	if obs.AttemptStatus != AttemptUnknown || !hasUnknown(obs, "attempt_ref") || !hasUnknown(obs, "run_ref") {
		t.Fatal("expected attempt and run to stay unknown")
	}
	if len(obs.Times) != 1 || obs.Times[0].Normalized != "" || obs.Times[0].Precision != "second" {
		t.Fatalf("times = %+v", obs.Times)
	}
	if len(obs.Evidence) != 1 || obs.Evidence[0].Revision != "blob-1" {
		t.Fatal("missing evidence")
	}
	store := NewStore()
	store.Apply(first)
	store.Apply(second)
	if len(store.Observations) != 1 {
		t.Fatalf("second parse added a row: %d", len(store.Observations))
	}
	if store.Observations[obs.ID].CollectedAt != secondAt {
		t.Fatal("collected_at was not updated")
	}
	if second.Observations[0].ID != obs.ID {
		t.Fatal("observation id changed for the same blob")
	}
}

func TestParseStrategyFixture(t *testing.T) {
	file := readFixture(t, "strat/RHAISTRAT/20260725-015518/pipeline-data.json")
	file.BlobID = "blob-strat"
	file.CommitSHA = "commit-strat"
	cfg := testConfig(ParserStrat)
	set := Parse(cfg, []File{file})
	if len(set.FileErrors) != 0 {
		t.Fatal(set.FileErrors)
	}
	if len(set.Observations) != 2 {
		t.Fatalf("observations = %d", len(set.Observations))
	}
	if len(set.Executions) != 1 || set.Executions[0].SourceRunID != "20260725-015518" || set.Executions[0].SourceRunID == "artifacts" {
		t.Fatalf("execution = %+v", set.Executions)
	}
	var withScore, without *Observation
	for i := range set.Observations {
		obs := &set.Observations[i]
		if obs.ExecutionID == "" || obs.AttemptStatus != AttemptUnknown || !hasUnknown(*obs, "run_ref.pipeline_id") {
			t.Fatalf("observation missing execution or unknowns: %+v", obs)
		}
		switch obs.SubjectKey {
		case "RHAISTRAT-2371":
			withScore = obs
		case "RHAISTRAT-2370":
			without = obs
		}
	}
	if withScore == nil || without == nil {
		t.Fatal("missing strategy observations")
	}
	if withScore.RawResult != "approve" || withScore.AssessmentTotal == nil || *withScore.AssessmentTotal != 6 {
		t.Fatalf("scored strategy = %+v", withScore)
	}
	if len(withScore.Links) != 1 || withScore.Links[0].Relation != "source_work_item" {
		t.Fatalf("links = %+v", withScore.Links)
	}
	linked := false
	for _, item := range set.WorkItems {
		if item.ItemKey == "RHAIRFE-2892" && item.EntityKind == "rfe" {
			linked = item.ID == withScore.Links[0].WorkItemID
		}
	}
	if !linked {
		t.Fatal("source_rfe was not stored as a work item link")
	}
	if without.AssessmentTotal != nil || without.RawResult != "Draft" {
		t.Fatalf("unscored strategy = %+v", without)
	}
	if withScore.Times[0].Normalized != "" {
		t.Fatalf("strategy timestamp was normalized: %+v", withScore.Times[0])
	}
}

func TestParseStratTaskWithoutRun(t *testing.T) {
	file := readFixture(t, "strat/RHAISTRAT/20260414-180837/strat-tasks/STRAT-1397.md")
	file.BlobID = "blob-task"
	cfg := testConfig(ParserStrat)
	set := Parse(cfg, []File{file})
	if len(set.FileErrors) != 0 || len(set.Observations) != 1 || len(set.Executions) != 0 {
		t.Fatalf("errors=%v observations=%d executions=%d", set.FileErrors, len(set.Observations), len(set.Executions))
	}
	obs := set.Observations[0]
	if obs.SubjectKey != "STRAT-1397" || !hasUnknown(obs, "subject_ref") || !hasUnknown(obs, "run_ref") {
		t.Fatalf("task observation = %+v unknowns=%+v", obs, obs.Unknowns)
	}
	if len(obs.Links) != 1 || obs.Links[0].Relation != "source_work_item" {
		t.Fatal("missing source_rfe link")
	}
}

func TestApplyIssueMatch(t *testing.T) {
	tests := []struct {
		name   string
		ids    []string
		status string
		domain string
	}{
		{"none", nil, MatchUnresolved, ""},
		{"one", []string{"jira:JiraIssue:1:10"}, MatchUnique, "jira:JiraIssue:1:10"},
		{"many", []string{"a", "b"}, MatchAmbiguous, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := WorkItem{ID: "item"}
			ApplyIssueMatch(&item, tt.ids)
			if item.MatchStatus != tt.status || item.DomainIssueID != tt.domain {
				t.Fatalf("match = %s %s", item.MatchStatus, item.DomainIssueID)
			}
		})
	}
}

func testConfig(parser string) Config {
	return Config{
		TenantID:    "rhai",
		SourceID:    "fixture",
		StationID:   "station",
		DisplayName: "Station",
		Parser:      parser,
		ProjectPath: "redhat/rhel-ai/agentic-ci/fixture",
		WebBase:     "https://gitlab.com",
	}
}

func readFixture(t *testing.T, rel string) File {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "testdata", rel))
	if err != nil {
		t.Fatal(err)
	}
	return File{Path: relPath(rel), Body: string(body)}
}

func relPath(rel string) string {
	// Fixture directories include the parser root. The GitLab path starts at RHAIRFE or RHAISTRAT.
	for _, root := range []string{"RHAIRFE/", "RHAISTRAT/"} {
		if i := indexOf(rel, root); i >= 0 {
			return rel[i:]
		}
	}
	return rel
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func hasUnknown(obs Observation, field string) bool {
	for _, unknown := range obs.Unknowns {
		if unknown.Field == field {
			return true
		}
	}
	return false
}
