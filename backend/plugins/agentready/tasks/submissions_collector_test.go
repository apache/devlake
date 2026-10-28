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
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSubmissionEntries(t *testing.T) {
	tree := []GithubTreeEntry{
		{Path: "submissions/org1/repo1/assessment-20260207-144231.json", Type: "blob"},
		{Path: "submissions/org1/repo1/assessment-20260301-100000.json", Type: "blob"},
		{Path: "submissions/org1/repo1/assessment-latest.json", Type: "blob"},
		{Path: "submissions/org2/repoA/assessment-20260115-090000.json", Type: "blob"},
		{Path: "submissions/org2/repoA/assessment-latest.json", Type: "blob"},
		{Path: "submissions/.trigger", Type: "blob"},
		{Path: "submissions/org1", Type: "tree"},
		{Path: "submissions/org1/repo1", Type: "tree"},
		{Path: "README.md", Type: "blob"},
		{Path: "submissions/org1/repo1/notes.txt", Type: "blob"},
		{Path: "submissions/deep/nested/too/far/file.json", Type: "blob"},
	}

	entries := ParseSubmissionEntries(tree, "submissions")

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Org != "org1" {
		t.Errorf("entries[0].Org = %q, want %q", entries[0].Org, "org1")
	}
	if entries[0].Repo != "repo1" {
		t.Errorf("entries[0].Repo = %q, want %q", entries[0].Repo, "repo1")
	}
	if entries[0].Filename != "assessment-latest.json" {
		t.Errorf("entries[0].Filename = %q, want %q", entries[0].Filename, "assessment-latest.json")
	}
	if entries[0].TreePath != "submissions/org1/repo1/assessment-latest.json" {
		t.Errorf("entries[0].TreePath = %q, want %q", entries[0].TreePath, "submissions/org1/repo1/assessment-latest.json")
	}

	if entries[1].Org != "org2" {
		t.Errorf("entries[1].Org = %q, want %q", entries[1].Org, "org2")
	}
	if entries[1].Repo != "repoA" {
		t.Errorf("entries[1].Repo = %q, want %q", entries[1].Repo, "repoA")
	}
	if entries[1].Filename != "assessment-latest.json" {
		t.Errorf("entries[1].Filename = %q, want %q", entries[1].Filename, "assessment-latest.json")
	}
	if entries[1].TreePath != "submissions/org2/repoA/assessment-latest.json" {
		t.Errorf("entries[1].TreePath = %q, want %q", entries[1].TreePath, "submissions/org2/repoA/assessment-latest.json")
	}
}

func TestParseSubmissionEntries_SkipsTimestampedFiles(t *testing.T) {
	tree := []GithubTreeEntry{
		{Path: "submissions/org1/repo1/assessment-20260207-144231.json", Type: "blob"},
		{Path: "submissions/org1/repo1/assessment-20260301-100000.json", Type: "blob"},
	}

	entries := ParseSubmissionEntries(tree, "submissions")

	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for timestamped-only directory, got %d", len(entries))
	}
}

func TestParseSubmissionEntries_CustomPath(t *testing.T) {
	tree := []GithubTreeEntry{
		{Path: "data/assessments/org1/repo1/assessment-latest.json", Type: "blob"},
		{Path: "submissions/org1/repo1/assessment-latest.json", Type: "blob"},
	}

	entries := ParseSubmissionEntries(tree, "data/assessments")

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Org != "org1" {
		t.Errorf("Org = %q, want %q", entries[0].Org, "org1")
	}
	if entries[0].Repo != "repo1" {
		t.Errorf("Repo = %q, want %q", entries[0].Repo, "repo1")
	}
	if entries[0].TreePath != "data/assessments/org1/repo1/assessment-latest.json" {
		t.Errorf("TreePath = %q, want %q", entries[0].TreePath, "data/assessments/org1/repo1/assessment-latest.json")
	}
}

func TestParseSubmissionEntries_Empty(t *testing.T) {
	entries := ParseSubmissionEntries(nil, "submissions")

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestFetchGithubTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/git/trees/main" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("recursive") != "1" {
			t.Errorf("expected recursive=1, got %s", r.URL.Query().Get("recursive"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}

		resp := githubTreeResponse{
			SHA:       "abc123",
			Truncated: false,
			Tree: []GithubTreeEntry{
				{Path: "submissions/org1/repo1/test.json", Mode: "100644", Type: "blob", SHA: "def456", Size: 1024},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := FetchGithubTree(context.Background(), server.URL, "owner/repo", "main", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SHA != "abc123" {
		t.Errorf("SHA = %q, want %q", result.SHA, "abc123")
	}
	if len(result.Tree) != 1 {
		t.Fatalf("expected 1 tree entry, got %d", len(result.Tree))
	}
	if result.Tree[0].Path != "submissions/org1/repo1/test.json" {
		t.Errorf("Tree[0].Path = %q, want %q", result.Tree[0].Path, "submissions/org1/repo1/test.json")
	}
}

func TestFetchGithubTree_NoToken(t *testing.T) {
	_, err := FetchGithubTree(context.Background(), "https://api.github.com", "owner/repo", "main", "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestFetchGithubTree_EmptyBranch(t *testing.T) {
	_, err := FetchGithubTree(context.Background(), "https://api.github.com", "owner/repo", "", "test-token")
	if err == nil {
		t.Fatal("expected error for empty branch")
	}
	if !strings.Contains(err.Error(), "branch is required") {
		t.Errorf("expected error about branch required, got: %v", err)
	}
}

func TestFetchGithubTree_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "API rate limit exceeded"}`))
	}))
	defer server.Close()

	_, err := FetchGithubTree(context.Background(), server.URL, "owner/repo", "main", "test-token")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error containing '403', got %v", err)
	}
}

func TestFetchGithubTree_BranchWithSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/repos/owner/repo/git/trees/feature%2Fsubmissions"
		if r.URL.RawPath != expectedPath {
			t.Errorf("expected RawPath %q, got %q", expectedPath, r.URL.RawPath)
		}
		resp := githubTreeResponse{SHA: "abc", Tree: []GithubTreeEntry{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	_, err := FetchGithubTree(context.Background(), server.URL, "owner/repo", "feature/submissions", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchGithubAssessment_CustomBranch(t *testing.T) {
	assessmentJSON := `{"overall_score": 70.0}`
	encoded := base64.StdEncoding.EncodeToString([]byte(assessmentJSON))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ref := r.URL.Query().Get("ref"); ref != "test" {
			t.Errorf("expected ref=test, got ref=%s", ref)
		}
		resp := map[string]any{
			"content":  encoded,
			"encoding": "base64",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := FetchGithubAssessment(context.Background(), server.URL, "org/submissions-repo", "submissions/myorg/myrepo/assessment.json", "test", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != assessmentJSON {
		t.Errorf("expected %q, got %q", assessmentJSON, result)
	}
}

func TestFetchDefaultBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}

		resp := map[string]any{
			"default_branch": "develop",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := FetchDefaultBranch(context.Background(), server.URL, "owner/repo", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "develop" {
		t.Errorf("expected %q, got %q", "develop", result)
	}
}

func TestFetchDefaultBranch_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	_, err := FetchDefaultBranch(context.Background(), server.URL, "owner/repo", "test-token")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error containing '404', got %v", err)
	}
}

func TestFetchDefaultBranch_EmptyDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"default_branch": ""}`))
	}))
	defer server.Close()

	_, err := FetchDefaultBranch(context.Background(), server.URL, "owner/repo", "test-token")
	if err == nil {
		t.Fatal("expected error for empty default_branch")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected error containing 'empty', got %v", err)
	}
}

func TestFetchDefaultBranch_NoToken(t *testing.T) {
	_, err := FetchDefaultBranch(context.Background(), "https://api.github.com", "owner/repo", "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Errorf("expected error containing 'token is required', got %v", err)
	}
}
