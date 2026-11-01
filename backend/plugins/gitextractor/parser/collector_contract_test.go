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

package parser

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/apache/devlake/plugins/gitextractor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// These tests describe the collector contract: what the git command line
// collector writes for a known repository. The fixtures are built with the git
// CLI itself, so the tests do not depend on any git library.

func newTestCollector(t *testing.T, localDir, repoId string, store models.Store) RepoCollector {
	t.Helper()
	collector, err := NewGitcliRepoCollector(localDir, repoId, store, unithelper.DummyLogger())
	require.Nil(t, err)
	return collector
}

func forEachCollector(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	t.Run("gitcli", fn)
}

// recordStore is a models.Store that records everything the collectors write.
type recordStore struct {
	incremental bool
	closed      bool
	repoCommits []*code.RepoCommit
	commits     []*code.Commit
	refs        []*code.Ref
	commitFiles []*code.CommitFile
	parents     []*code.CommitParent
	components  []*code.CommitFileComponent
	lineChanges []*code.CommitLineChange
	snapshots   []*code.RepoSnapshot
}

var _ models.Store = (*recordStore)(nil)

func (s *recordStore) SetIncrementalMode(v bool) { s.incremental = v }

func (s *recordStore) RepoCommits(v *code.RepoCommit) errors.Error {
	s.repoCommits = append(s.repoCommits, v)
	return nil
}

func (s *recordStore) Commits(v *code.Commit) errors.Error {
	cp := *v
	s.commits = append(s.commits, &cp)
	return nil
}

func (s *recordStore) Refs(v *code.Ref) errors.Error {
	cp := *v
	s.refs = append(s.refs, &cp)
	return nil
}

func (s *recordStore) CommitFiles(v *code.CommitFile) errors.Error {
	cp := *v
	s.commitFiles = append(s.commitFiles, &cp)
	return nil
}

func (s *recordStore) CommitParents(v []*code.CommitParent) errors.Error {
	s.parents = append(s.parents, v...)
	return nil
}

func (s *recordStore) CommitFileComponents(v *code.CommitFileComponent) errors.Error {
	cp := *v
	s.components = append(s.components, &cp)
	return nil
}

func (s *recordStore) CommitLineChange(v *code.CommitLineChange) errors.Error {
	cp := *v
	s.lineChanges = append(s.lineChanges, &cp)
	return nil
}

func (s *recordStore) RepoSnapshot(v *code.RepoSnapshot) errors.Error {
	cp := *v
	s.snapshots = append(s.snapshots, &cp)
	return nil
}

func (s *recordStore) Close() errors.Error {
	s.closed = true
	return nil
}

func boolPtr(v bool) *bool { return &v }

func defaultOptions(repoId string) *GitExtractorOptions {
	return &GitExtractorOptions{
		GitExtractorApiParams: GitExtractorApiParams{RepoId: repoId},
		SkipCommitStat:        boolPtr(false),
		SkipCommitFiles:       boolPtr(false),
	}
}

// newCollectorContext returns a subtask context backed by a mocked DAL. The
// components are returned from Dal.All, which the collectors use to map files
// to components.
func newCollectorContext(t *testing.T, opts *GitExtractorOptions, components []code.Component) *mockplugin.SubTaskContext {
	t.Helper()
	dalMock := new(mockdal.Dal)
	dalMock.On("All", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		if len(components) == 0 {
			return
		}
		dst, ok := args.Get(0).(*[]code.Component)
		if !ok {
			return
		}
		*dst = append(*dst, components...)
	})
	dalMock.On("Delete", mock.Anything, mock.Anything).Return(nil)

	ctx := unithelper.DummySubTaskContext(dalMock)
	ctx.On("GetContext").Return(context.Background())
	ctx.On("GetData").Return(&GitExtractorTaskData{Options: opts})
	return ctx
}

func commitsBySha(commits []*code.Commit) map[string]*code.Commit {
	out := make(map[string]*code.Commit, len(commits))
	for _, commit := range commits {
		out[commit.Sha] = commit
	}
	return out
}

func refsByName(refs []*code.Ref) map[string]*code.Ref {
	out := make(map[string]*code.Ref, len(refs))
	for _, ref := range refs {
		out[ref.Name] = ref
	}
	return out
}

func componentsById(components []*code.CommitFileComponent) map[string]string {
	out := make(map[string]string, len(components))
	for _, component := range components {
		out[component.CommitFileId] = component.ComponentName
	}
	return out
}

// filesByCommit returns commit sha -> file path -> [additions, deletions].
func filesByCommit(files []*code.CommitFile) map[string]map[string][2]int {
	out := make(map[string]map[string][2]int)
	for _, file := range files {
		if out[file.CommitSha] == nil {
			out[file.CommitSha] = make(map[string][2]int)
		}
		out[file.CommitSha][file.FilePath] = [2]int{file.Additions, file.Deletions}
	}
	return out
}

func parentShas(parents []*code.CommitParent, commitSha string) []string {
	var out []string
	for _, parent := range parents {
		if parent.CommitSha == commitSha {
			out = append(out, parent.ParentCommitSha)
		}
	}
	sort.Strings(out)
	return out
}

func TestCollectorCountsAndRefs(t *testing.T) {
	repo := buildCanonicalRepo(t)

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, repo.Dir, "test-repo", store)
		ctx := newCollectorContext(t, defaultOptions("test-repo"), nil)

		commits, err := collector.CountCommits(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 5, commits)

		tags, err := collector.CountTags(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 2, tags)
		// HEAD points at main, which is not counted.
		branches, err := collector.CountBranches(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, branches)

		assert.NoError(t, collector.CollectTags(ctx))
		assert.NoError(t, collector.CollectBranches(ctx))

		refs := refsByName(store.refs)
		require.Len(t, refs, 4)

		// Tag names are stored as the full ref name.
		light := refs["refs/tags/v1.0.0"]
		require.NotNil(t, light, "lightweight tag v1.0.0")
		assert.Equal(t, "test-repo:refs/tags/v1.0.0", light.Id)
		assert.Equal(t, repo.C1, light.CommitSha)
		assert.Equal(t, TAG, light.RefType)
		assert.Equal(t, "test-repo", light.RepoId)

		annotated := refs["refs/tags/v2.0.0"]
		require.NotNil(t, annotated, "annotated tag v2.0.0 should resolve to its target commit")
		assert.Equal(t, repo.C3, annotated.CommitSha)
		assert.Equal(t, TAG, annotated.RefType)

		mainRef := refs["main"]
		require.NotNil(t, mainRef)
		assert.Equal(t, repo.C5, mainRef.CommitSha)
		assert.Equal(t, BRANCH, mainRef.RefType)
		assert.True(t, mainRef.IsDefault)

		featureRef := refs["feature"]
		require.NotNil(t, featureRef)
		assert.Equal(t, repo.C2, featureRef.CommitSha)
		assert.Equal(t, BRANCH, featureRef.RefType)
		assert.False(t, featureRef.IsDefault)
	})
}

func TestCollectorCommits(t *testing.T) {
	repo := buildCanonicalRepo(t)
	repoId := "test-repo"

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, repo.Dir, repoId, store)
		ctx := newCollectorContext(t, defaultOptions(repoId), nil)

		require.NoError(t, collector.CollectCommits(ctx))

		commits := commitsBySha(store.commits)
		require.Len(t, commits, 5)
		require.Len(t, store.repoCommits, 5)

		for _, tc := range []struct {
			sha  string
			when string
			msg  string
		}{
			{repo.C1, "2026-01-01T00:00:00Z", "c1: initial import"},
			{repo.C2, "2026-01-01T01:00:00Z", "c2: add feature file"},
			{repo.C3, "2026-01-01T02:00:00Z", "c3: reword and extend a.txt"},
			{repo.C4, "2026-01-01T03:00:00Z", "c4: mode and binary change"},
			{repo.C5, "2026-01-01T04:00:00Z", "c5: merge feature"},
		} {
			commit := commits[tc.sha]
			require.NotNil(t, commit, tc.msg)
			assert.Equal(t, tc.msg+"\n", commit.Message)
			assert.Equal(t, testAuthorName, commit.AuthorName)
			assert.Equal(t, testAuthorEmail, commit.AuthorEmail)
			assert.Equal(t, testAuthorEmail, commit.AuthorId)
			assert.Equal(t, testAuthorName, commit.CommitterName)
			assert.Equal(t, testAuthorEmail, commit.CommitterEmail)
			assert.Equal(t, testAuthorEmail, commit.CommitterId)
			assert.Equal(t, tc.when, commit.AuthoredDate.UTC().Format("2006-01-02T15:04:05Z"))
			assert.Equal(t, tc.when, commit.CommittedDate.UTC().Format("2006-01-02T15:04:05Z"))
		}

		assert.Empty(t, parentShas(store.parents, repo.C1))
		assert.Equal(t, []string{repo.C1}, parentShas(store.parents, repo.C2))
		assert.Equal(t, []string{repo.C1}, parentShas(store.parents, repo.C3))
		assert.Equal(t, []string{repo.C3}, parentShas(store.parents, repo.C4))
		assert.ElementsMatch(t, []string{repo.C2, repo.C4}, parentShas(store.parents, repo.C5))

		files := filesByCommit(store.commitFiles)
		expectedFiles := map[string]map[string][2]int{
			repo.C1: {
				"a.txt":                {3, 0},
				"bin.dat":              {0, 0},
				"dir with space/b.txt": {1, 0},
				"empty.txt":            {0, 0},
			},
			repo.C2: {"feature.txt": {1, 0}},
			repo.C3: {
				"a.txt":     {2, 1},
				"empty.txt": {0, 0},
			},
			repo.C4: {
				"a.txt":   {0, 0},
				"bin.dat": {0, 0},
			},
			// The merge is compared against its first parent, so only the file
			// the merge brought in is reported.
			repo.C5: {"feature.txt": {1, 0}},
		}
		for sha, want := range expectedFiles {
			assert.Equal(t, want, files[sha], sha)
		}

		assert.Equal(t, 4, commits[repo.C1].Additions)
		assert.Equal(t, 0, commits[repo.C1].Deletions)
		assert.Equal(t, 1, commits[repo.C2].Additions)
		assert.Equal(t, 2, commits[repo.C3].Additions)
		assert.Equal(t, 1, commits[repo.C3].Deletions)
		assert.Equal(t, 0, commits[repo.C4].Additions)
		assert.Equal(t, 0, commits[repo.C4].Deletions)
		assert.Equal(t, 1, commits[repo.C5].Additions)
		assert.Equal(t, 0, commits[repo.C5].Deletions)

		for _, file := range store.commitFiles {
			assert.Equal(t, genCommitFileId(file.CommitSha, file.FilePath), file.Id)
		}
		// This repository has no components configured, so every file would
		// only get a meaningless "Default" component row.
		assert.Empty(t, store.components)
	})
}

func TestCollectorUsesConfiguredComponents(t *testing.T) {
	repo := buildLinearRepo(t)

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, repo.Dir, "test-repo", store)
		ctx := newCollectorContext(t, defaultOptions("test-repo"), []code.Component{
			{RepoId: "test-repo", Name: "docs", PathRegex: `^c\.txt$`},
		})

		require.NoError(t, collector.CollectCommits(ctx))

		components := componentsById(store.components)
		files := filesByCommit(store.commitFiles)[repo.C4]
		require.Contains(t, files, "c.txt")
		assert.Equal(t, "docs", components[genCommitFileId(repo.C4, "c.txt")])
		assert.Equal(t, "Default", components[genCommitFileId(repo.C4, "b.txt")])
	})
}

func TestCollectorSkipCommitStat(t *testing.T) {
	repo := buildCanonicalRepo(t)

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, repo.Dir, "test-repo", store)
		opts := defaultOptions("test-repo")
		opts.SkipCommitStat = boolPtr(true)
		opts.SkipCommitFiles = boolPtr(true)
		ctx := newCollectorContext(t, opts, nil)

		require.NoError(t, collector.CollectCommits(ctx))

		require.Len(t, store.commits, 5)
		for _, commit := range store.commits {
			assert.Zero(t, commit.Additions)
			assert.Zero(t, commit.Deletions)
		}
		assert.Empty(t, store.commitFiles)
		assert.Empty(t, store.components)
	})
}

func TestCollectorSkipCommitFiles(t *testing.T) {
	repo := buildCanonicalRepo(t)

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, repo.Dir, "test-repo", store)
		opts := defaultOptions("test-repo")
		opts.SkipCommitFiles = boolPtr(true)
		ctx := newCollectorContext(t, opts, nil)

		require.NoError(t, collector.CollectCommits(ctx))

		assert.Empty(t, store.commitFiles)
		assert.Empty(t, store.components)
		commits := commitsBySha(store.commits)
		assert.Equal(t, 4, commits[repo.C1].Additions)
		assert.Equal(t, 1, commits[repo.C2].Additions)
	})
}

func TestCollectorExcludeFileExtensions(t *testing.T) {
	repo := buildCanonicalRepo(t)

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, repo.Dir, "test-repo", store)
		opts := defaultOptions("test-repo")
		opts.ExcludeFileExtensions = []string{".DAT"}
		ctx := newCollectorContext(t, opts, nil)

		require.NoError(t, collector.CollectCommits(ctx))

		commits := commitsBySha(store.commits)
		assert.Equal(t, 4, commits[repo.C1].Additions)
		assert.Equal(t, 0, commits[repo.C4].Additions)
		assert.Equal(t, 0, commits[repo.C4].Deletions)

		files := filesByCommit(store.commitFiles)
		assert.NotContains(t, files[repo.C1], "bin.dat")
		assert.NotContains(t, files[repo.C4], "bin.dat")
		assert.Contains(t, files[repo.C1], "a.txt")
	})
}

func TestCollectorSkipsShallowBoundaryCommit(t *testing.T) {
	f := newGitFixture(t)
	f.writeFile("a.txt", "l1\n")
	c1 := f.commit(testTimeC1, "c1")
	f.writeFile("a.txt", "l1\nl2\n")
	c2 := f.commit(testTimeC2, "c2")
	require.NotEqual(t, c1, c2)

	dir := f.shallowBareClone()

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, dir, "test-repo", store)
		ctx := newCollectorContext(t, defaultOptions("test-repo"), nil)

		// The tip is a shallow boundary: its parent is not in the object
		// database, so the collector must not compute stats for it (a full-tree
		// diff would store an absurdly large addition count).
		require.NoError(t, collector.CollectCommits(ctx))
		assert.Empty(t, store.commits)
		assert.Empty(t, store.commitFiles)
	})
}

func TestCollectorSkipsShallowBoundaryEvenWhenItsParentExists(t *testing.T) {
	f := newGitFixture(t)
	f.writeFile("a.txt", "l1\n")
	c1 := f.commit(testTimeC1, "c1")
	f.writeFile("a.txt", "l1\nl2\n")
	c2 := f.commit(testTimeC2, "c2")
	// keep the parent reachable through another branch
	f.git("branch", "old", c1)

	dir := f.bareClone()
	// Mark the tip as a shallow boundary although its parent object is still
	// in the object database. git then renders the tip as a root commit, so
	// the collector must skip it based on the shallow file, not on the parent
	// being missing.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "shallow"), []byte(c2+"\n"), 0o644))

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, dir, "test-repo", store)
		ctx := newCollectorContext(t, defaultOptions("test-repo"), nil)

		require.NoError(t, collector.CollectCommits(ctx))

		commits := commitsBySha(store.commits)
		assert.NotContains(t, commits, c2, "the shallow boundary must not be collected")
		require.Contains(t, commits, c1)
		assert.Equal(t, 1, commits[c1].Additions)
	})
}

func TestCollectorEmptyRepo(t *testing.T) {
	dir := newGitFixture(t).bareClone()

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, dir, "test-repo", store)
		ctx := newCollectorContext(t, defaultOptions("test-repo"), nil)

		commits, err := collector.CountCommits(context.Background())
		require.NoError(t, err)
		assert.Zero(t, commits)
		tags, err := collector.CountTags(context.Background())
		require.NoError(t, err)
		assert.Zero(t, tags)
		branches, err := collector.CountBranches(context.Background())
		require.NoError(t, err)
		assert.Zero(t, branches)

		assert.NoError(t, collector.CollectTags(ctx))
		assert.NoError(t, collector.CollectBranches(ctx))
		assert.NoError(t, collector.CollectCommits(ctx))
		assert.Empty(t, store.refs)
		assert.Empty(t, store.commits)
	})
}

func TestCollectorHonoursCancelledContext(t *testing.T) {
	repo := buildCanonicalRepo(t)

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, repo.Dir, "test-repo", store)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := collector.CountCommits(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		_, err = collector.CountTags(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		_, err = collector.CountBranches(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestCollectorRejectsNonRepo(t *testing.T) {
	_, err := NewGitcliRepoCollector(t.TempDir(), "test-repo", nil, unithelper.DummyLogger())
	assert.Error(t, err, "the collector should reject a non-repository")
}

func TestCollectorCountsWhenHeadIsNotDefault(t *testing.T) {
	f := newGitFixture(t)
	f.writeFile("a.txt", "l1\n")
	c1 := f.commit(testTimeC1, "c1")
	f.checkout("feature", true)
	f.writeFile("a.txt", "l1\nl2\n")
	c2 := f.commit(testTimeC2, "c2")
	require.NotEqual(t, c1, c2)
	dir := f.bareClone()

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, dir, "test-repo", store)

		// HEAD points at feature, which is not counted but is the default ref.
		branches, err := collector.CountBranches(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, branches)

		ctx := newCollectorContext(t, defaultOptions("test-repo"), nil)
		require.NoError(t, collector.CollectBranches(ctx))

		refs := refsByName(store.refs)
		require.Len(t, refs, 2)
		assert.True(t, refs["feature"].IsDefault)
		assert.False(t, refs["main"].IsDefault)
	})
}

// TestCollectorDiffLineSnapshot checks the repo snapshot against git blame,
// which is an independent oracle for "which commit introduced each line".
func TestCollectorDiffLineSnapshot(t *testing.T) {
	repo := buildLinearRepo(t)
	expected := blameAtHEAD(t, repo.Dir)

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, repo.Dir, "test-repo", store)
		ctx := newCollectorContext(t, defaultOptions("test-repo"), nil)

		require.NoError(t, collector.CollectDiffLine(ctx))

		got := make(map[string]map[int]string)
		for _, snapshot := range store.snapshots {
			if got[snapshot.FilePath] == nil {
				got[snapshot.FilePath] = make(map[int]string)
			}
			got[snapshot.FilePath][snapshot.LineNo] = snapshot.CommitSha
		}

		// b.txt is deleted in c4, so it has no lines in HEAD and no snapshot rows.
		assert.NotContains(t, got, "b.txt")
		for file, lines := range expected {
			assert.Equal(t, lines, got[file], file)
		}

		assert.NotEmpty(t, store.lineChanges)
	})
}

func TestCollectorCollectAllAndClose(t *testing.T) {
	repo := buildCanonicalRepo(t)

	forEachCollector(t, func(t *testing.T) {
		store := &recordStore{}
		collector := newTestCollector(t, repo.Dir, "test-repo", store)
		ctx := newCollectorContext(t, defaultOptions("test-repo"), nil)

		cleanedUp := false
		require.NoError(t, collector.SetCleanUp(func() { cleanedUp = true }))

		require.NoError(t, collector.CollectAll(ctx))
		assert.Len(t, store.commits, 5)
		assert.Len(t, store.repoCommits, 5)
		assert.Len(t, store.refs, 4)
		assert.NotEmpty(t, store.commitFiles)
		assert.NotEmpty(t, store.snapshots)
		assert.NotEmpty(t, store.lineChanges)

		require.NoError(t, collector.Close(context.Background()))
		assert.True(t, store.closed)
		assert.True(t, cleanedUp)
	})
}

func TestCollectorCommitLineChanges(t *testing.T) {
	repo := buildLinearRepo(t)

	store := &recordStore{}
	collector := newTestCollector(t, repo.Dir, "test-repo", store)
	ctx := newCollectorContext(t, defaultOptions("test-repo"), nil)

	require.NoError(t, collector.CollectDiffLine(ctx))
	require.NotEmpty(t, store.lineChanges)

	// c3 appends line 4 to a.txt, so a.txt gets one addition at new line 4.
	var additions []*code.CommitLineChange
	for _, change := range store.lineChanges {
		if change.CommitSha == repo.C3 && change.NewFilePath == "a.txt" && change.ChangedType == "Addition" {
			additions = append(additions, change)
		}
	}
	require.Len(t, additions, 1)
	assert.Equal(t, 4, additions[0].LineNoNew)
	assert.Equal(t, -1, additions[0].LineNoOld)
	assert.Equal(t, 1, additions[0].HunkNum)
	assert.Equal(t, "", additions[0].PrevCommit)
	assert.Equal(t, repo.C3+":a.txt:-1:4", additions[0].Id)

	// c2 rewrites line 2 of a.txt; the deletion must be attributed to c1.
	var deletions []*code.CommitLineChange
	for _, change := range store.lineChanges {
		if change.CommitSha == repo.C2 && change.NewFilePath == "a.txt" && change.ChangedType == "Deletion" {
			deletions = append(deletions, change)
		}
	}
	require.Len(t, deletions, 1)
	assert.Equal(t, 2, deletions[0].LineNoOld)
	assert.Equal(t, repo.C1, deletions[0].PrevCommit)
}
