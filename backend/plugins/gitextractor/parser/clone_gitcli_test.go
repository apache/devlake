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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apache/devlake/helpers/unithelper"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runGit runs git in dir with a fixed identity (and date, if given), and returns its trimmed stdout.
func runGit(t *testing.T, dir, date string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=DevLake Test", "GIT_AUTHOR_EMAIL=test@devlake.apache.org",
		"GIT_COMMITTER_NAME=DevLake Test", "GIT_COMMITTER_EMAIL=test@devlake.apache.org",
	)
	if date != "" {
		cmd.Env = append(cmd.Env, "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	}
	output, err := cmd.Output()
	var stderr []byte
	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr = exitErr.Stderr
	}
	require.NoError(t, err, "git %v: %s", args, stderr)
	return strings.TrimSpace(string(output))
}

// buildMergeBoundaryRepo creates a bare repo with the history from issue #9189:
//
//	R ---- Q ---- P ---- M   main
//	 \                  /
//	  F ---------------+     feature
//
// The previous incremental run started at 12:00, after F (10:00) and before
// Q (13:00), P (14:00) and M (16:00).
func buildMergeBoundaryRepo(t *testing.T) (dir string, commits map[string]string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "origin.git")
	require.NoError(t, os.Mkdir(dir, 0o755))
	runGit(t, dir, "", "init", "-q", "--bare")
	runGit(t, dir, "", "symbolic-ref", "HEAD", "refs/heads/main")
	tree := runGit(t, dir, "", "mktree")
	commit := func(date, message string, parents ...string) string {
		args := []string{"commit-tree", tree, "-m", message}
		for _, parent := range parents {
			args = append(args, "-p", parent)
		}
		return runGit(t, dir, date, args...)
	}
	commits = map[string]string{}
	commits["R"] = commit("2024-01-01T00:00:00Z", "R")
	commits["F"] = commit("2024-01-02T10:00:00Z", "F", commits["R"])
	commits["Q"] = commit("2024-01-02T13:00:00Z", "Q", commits["R"])
	commits["P"] = commit("2024-01-02T14:00:00Z", "P", commits["Q"])
	commits["M"] = commit("2024-01-02T16:00:00Z", "M", commits["P"], commits["F"])
	runGit(t, dir, "", "update-ref", "refs/heads/main", commits["M"])
	runGit(t, dir, "", "update-ref", "refs/heads/feature", commits["F"])
	return dir, commits
}

func newTestCloner(t *testing.T, remoteDir string, since time.Time) *GitcliCloner {
	t.Helper()
	ctx := new(mockplugin.SubTaskContext)
	ctx.On("GetContext").Return(context.Background())
	return &GitcliCloner{
		ctx:       ctx,
		taskData:  &GitExtractorTaskData{Options: &GitExtractorOptions{}},
		logger:    unithelper.DummyLogger(),
		since:     &since,
		remoteUrl: "file://" + remoteDir,
		localDir:  filepath.Join(t.TempDir(), "clone.git"),
	}
}

func hasObject(dir, sha string) bool {
	cmd := exec.Command("git", "cat-file", "-e", sha)
	cmd.Dir = dir
	return cmd.Run() == nil
}

func TestIncrementalCloneKeepsCommitsBehindMergeBoundary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	origin, commits := buildMergeBoundaryRepo(t)
	cloner := newTestCloner(t, origin, time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC))

	require.NoError(t, cloner.CloneRepo())

	// Q, P and M are newer than since. Each must be fetched together with its
	// first parent, otherwise CollectCommits skips it and no later run collects it.
	for _, name := range []string{"Q", "P", "M"} {
		assert.True(t, hasObject(cloner.localDir, commits[name]), "%s should be fetched", name)
	}
	assert.True(t, hasObject(cloner.localDir, commits["R"]), "R, the first parent of Q, should be fetched")
	newest, err := cloner.newestShallowCommitTime()
	require.NoError(t, err)
	assert.True(t, newest.Before(*cloner.since), "shallow boundary %s should be older than since", newest)
}

func TestNewestShallowCommitTimeIgnoresMissingCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	origin, commits := buildMergeBoundaryRepo(t)
	cloner := newTestCloner(t, origin, time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC))
	runGit(t, "", "", "clone", "-q", "--bare", "--depth=1", cloner.remoteUrl, cloner.localDir)

	// a depth=1 clone of main leaves only M on the boundary
	newest, err := cloner.newestShallowCommitTime()
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 2, 16, 0, 0, 0, time.UTC), newest.UTC())

	// git may list a boundary commit it did not fetch; it must not fail the lookup
	shallowFile := filepath.Join(cloner.localDir, "shallow")
	shallow, e := os.ReadFile(shallowFile)
	require.NoError(t, e)
	require.NoError(t, os.WriteFile(shallowFile, append(shallow, commits["P"]+"\n"...), 0o644))
	require.False(t, hasObject(cloner.localDir, commits["P"]))
	newest, err = cloner.newestShallowCommitTime()
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 2, 16, 0, 0, 0, time.UTC), newest.UTC())

	// a full clone has no shallow file
	require.NoError(t, os.Remove(shallowFile))
	newest, err = cloner.newestShallowCommitTime()
	require.NoError(t, err)
	assert.True(t, newest.IsZero())
}
