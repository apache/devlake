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

	"github.com/stretchr/testify/assert"
)

func TestCollectContainerImages_ReturnsSortedUniqueImages(t *testing.T) {
	op := &ArgocdApiSyncOperation{}
	op.Metadata.Images = []string{"registry.example.com/system:ops", " registry.example.com/sidecar:def456 "}
	op.Metadata.Resources = []ArgocdApiSyncOperationMetadataResource{
		{Images: []string{"registry.example.com/app:abc123", "", "registry.example.com/api:789xyz"}},
	}
	op.Operation.Metadata.Images = []string{"registry.example.com/worker:alpha"}
	op.Operation.Metadata.Resources = []ArgocdApiSyncOperationMetadataResource{
		{Images: []string{"registry.example.com/rollout:blue"}},
	}
	op.Operation.Sync.Resources = []ArgocdApiSyncResourceItem{
		{Images: []string{"registry.example.com/canary:latest"}},
	}
	op.SyncResult.Resources = []ArgocdApiSyncResourceItem{
		{Images: []string{"registry.example.com/worker:alpha", "registry.example.com/api:789xyz"}},
	}

	images := collectContainerImages(op)

	expected := []string{
		"registry.example.com/api:789xyz",
		"registry.example.com/app:abc123",
		"registry.example.com/canary:latest",
		"registry.example.com/rollout:blue",
		"registry.example.com/sidecar:def456",
		"registry.example.com/system:ops",
		"registry.example.com/worker:alpha",
	}
	assert.Equal(t, expected, images)
}

func TestCollectContainerImages_EmptyInputReturnsNil(t *testing.T) {
	op := &ArgocdApiSyncOperation{}

	assert.Nil(t, collectContainerImages(op))
	assert.Nil(t, collectContainerImages(nil))
}

func TestNormalizeImages(t *testing.T) {
	input := []string{" registry.example.com/app:1.0 ", "registry.example.com/app:1.0", "registry.example.com/api:2.0", ""}
	expected := []string{"registry.example.com/api:2.0", "registry.example.com/app:1.0"}
	assert.Equal(t, expected, normalizeImages(input))
}

// Fallback: no images in payload → expect none here (other fallbacks tested elsewhere)
func TestCollectContainerImages_FallbackRevisionAndSummary(t *testing.T) {
	revision := "abcdef1234567890"
	apiPayload := ArgocdApiSyncOperation{Revision: revision}
	assert.Nil(t, collectContainerImages(&apiPayload))

	// normalizeImages: dedupe + sort
	assert.Equal(t, []string{"a", "b"}, normalizeImages([]string{"b", "a", "b"}))
}

// ── resolveMultiSourceRevision ────────────────────────────────────────────────

func TestResolveMultiSourceRevision_GitHubSourceWins(t *testing.T) {
	// Multi-source pattern: Helm chart (GCS) + git values repo (GitHub).
	revisions := []string{"2.6.2", "5dd95b4efd7e9b668c361bbddb8d7f1e56c32ac1"}
	sources := []ArgocdApiSyncSource{
		{RepoURL: "gs://charts-example-net/infra/stable", Chart: "generic-service"},
		{RepoURL: "https://github.com/example/my-repo"},
	}
	got := resolveMultiSourceRevision(revisions, sources)
	assert.Equal(t, "5dd95b4efd7e9b668c361bbddb8d7f1e56c32ac1", got)
}

func TestResolveMultiSourceRevision_GitLabSourceWins(t *testing.T) {
	revisions := []string{"1.0.0", "aabbccdd11223344aabbccdd11223344aabbccdd"}
	sources := []ArgocdApiSyncSource{
		{RepoURL: "oci://registry.example.com/charts", Chart: "app"},
		{RepoURL: "https://gitlab.com/example/config"},
	}
	got := resolveMultiSourceRevision(revisions, sources)
	assert.Equal(t, "aabbccdd11223344aabbccdd11223344aabbccdd", got)
}

func TestResolveMultiSourceRevision_FallbackToAnySHA(t *testing.T) {
	// Neither source matches a known git hosting service (no github/gitlab/gitea/etc.
	// prefix). The function should still return the 40-hex SHA via the fallback
	// pass that accepts any commit-SHA-shaped revision regardless of source type.
	revisions := []string{"1.2.3", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}
	sources := []ArgocdApiSyncSource{
		{RepoURL: "gs://bucket/charts"},
		{RepoURL: "https://git.acme-corp.internal/team/config"},
	}
	got := resolveMultiSourceRevision(revisions, sources)
	assert.Equal(t, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", got)
}

func TestResolveMultiSourceRevision_EmptyRevisions(t *testing.T) {
	assert.Equal(t, "", resolveMultiSourceRevision(nil, nil))
	assert.Equal(t, "", resolveMultiSourceRevision([]string{}, []ArgocdApiSyncSource{}))
}

func TestResolveMultiSourceRevision_AllSemver(t *testing.T) {
	// All revisions are semver tags; nothing looks like a commit SHA.
	revisions := []string{"1.0.0", "2.3.4"}
	sources := []ArgocdApiSyncSource{
		{RepoURL: "oci://registry.example.com/charts"},
		{RepoURL: "oci://registry.example.com/other"},
	}
	assert.Equal(t, "", resolveMultiSourceRevision(revisions, sources))
}

func TestResolveMultiSourceRevision_SingleGitSHA(t *testing.T) {
	// Single-source multi-source edge case.
	revisions := []string{"abcdef1234567890abcdef1234567890abcdef12"}
	sources := []ArgocdApiSyncSource{{RepoURL: "https://github.com/example/repo"}}
	got := resolveMultiSourceRevision(revisions, sources)
	assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef12", got)
}

// ── isCommitSHA ───────────────────────────────────────────────────────────────

func TestIsCommitSHA(t *testing.T) {
	assert.True(t, isCommitSHA("5dd95b4efd7e9b668c361bbddb8d7f1e56c32ac1"))
	assert.True(t, isCommitSHA("AABBCCDD11223344AABBCCDD11223344AABBCCDD"))
	assert.False(t, isCommitSHA("2.6.2"))
	assert.False(t, isCommitSHA(""))
	assert.False(t, isCommitSHA("5dd95b4efd7e9b668c361bbddb8d7f1e56c32ac"))   // 39 chars
	assert.False(t, isCommitSHA("5dd95b4efd7e9b668c361bbddb8d7f1e56c32ac12")) // 41 chars
}

// ── resolveGitRepoURL ─────────────────────────────────────────────────────────

func TestResolveGitRepoURL_SingleSource(t *testing.T) {
	// Single-source app: singleSourceURL is used directly, sources ignored.
	got := resolveGitRepoURL("https://github.com/example/my-app", nil)
	assert.Equal(t, "https://github.com/example/my-app", got)
}

func TestResolveGitRepoURL_MultiSourceGitHubWins(t *testing.T) {
	// Multi-source pattern: GCS chart + GitHub values ref.
	sources := []ArgocdApiSyncSource{
		{RepoURL: "gs://charts-example-net/infra/stable", Chart: "generic-service"},
		{RepoURL: "https://github.com/example/my-app"},
	}
	got := resolveGitRepoURL("", sources)
	assert.Equal(t, "https://github.com/example/my-app", got)
}

func TestResolveGitRepoURL_MultiSourceOCIChart(t *testing.T) {
	// OCI chart + GitLab values repo.
	sources := []ArgocdApiSyncSource{
		{RepoURL: "oci://registry.example.com/charts", Chart: "app"},
		{RepoURL: "https://gitlab.com/org/config"},
	}
	got := resolveGitRepoURL("", sources)
	assert.Equal(t, "https://gitlab.com/org/config", got)
}

func TestResolveGitRepoURL_FallbackNonChartURL(t *testing.T) {
	// No known git host but a non-chart HTTPS URL is still better than nothing.
	sources := []ArgocdApiSyncSource{
		{RepoURL: "gs://bucket/charts"},
		{RepoURL: "https://git.acme-corp.internal/team/config"},
	}
	got := resolveGitRepoURL("", sources)
	assert.Equal(t, "https://git.acme-corp.internal/team/config", got)
}

func TestResolveGitRepoURL_AllChartSources(t *testing.T) {
	// All sources are chart registries — returns empty string.
	sources := []ArgocdApiSyncSource{
		{RepoURL: "gs://charts-example-net/infra/stable"},
		{RepoURL: "oci://registry.example.com/charts"},
	}
	got := resolveGitRepoURL("", sources)
	assert.Equal(t, "", got)
}

func TestResolveGitRepoURL_EmptySources(t *testing.T) {
	assert.Equal(t, "", resolveGitRepoURL("", nil))
	assert.Equal(t, "", resolveGitRepoURL("", []ArgocdApiSyncSource{}))
}

// ── isGitHostedURL ────────────────────────────────────────────────────────────

func TestIsGitHostedURL(t *testing.T) {
	assert.True(t, isGitHostedURL("https://github.com/org/repo"))
	assert.True(t, isGitHostedURL("git@github.com:org/repo.git"))
	assert.True(t, isGitHostedURL("https://gitlab.com/org/repo"))
	assert.True(t, isGitHostedURL("https://bitbucket.org/org/repo"))
	assert.True(t, isGitHostedURL("https://dev.azure.com/org/proj/_git/repo"))
	assert.True(t, isGitHostedURL("https://gitea.internal.corp/team/config"))
	assert.True(t, isGitHostedURL("https://example.com/repo.git"))

	assert.False(t, isGitHostedURL("gs://charts-example-net/infra/stable"))
	assert.False(t, isGitHostedURL("oci://registry.example.com/charts"))
	assert.False(t, isGitHostedURL("s3://my-bucket/charts"))
	assert.False(t, isGitHostedURL(""))
}

// ── isRedundantOperationState ─────────────────────────────────────────────────

func TestIsRedundantOperationState_SucceededIsSkipped(t *testing.T) {
	assert.True(t, isRedundantOperationState("Succeeded"),
		"Succeeded operationState always duplicates the latest history entry")
}

func TestIsRedundantOperationState_NonTerminalPhasesAreKept(t *testing.T) {
	for _, phase := range []string{"Running", "Terminating"} {
		assert.False(t, isRedundantOperationState(phase),
			"in-flight phase %q is not yet in history and must be kept", phase)
	}
}

func TestIsRedundantOperationState_FailurePhasesAreKept(t *testing.T) {
	for _, phase := range []string{"Failed", "Error"} {
		assert.False(t, isRedundantOperationState(phase),
			"failure phase %q may never reach history and must be kept", phase)
	}
}

func TestIsRedundantOperationState_HistoryEntryHasNoPhase(t *testing.T) {
	// History entries have phase="" — isOperationState is false before this
	// function is called, but double-check the function itself is safe with
	// an empty string so the guard can never accidentally suppress history rows.
	assert.False(t, isRedundantOperationState(""))
}
