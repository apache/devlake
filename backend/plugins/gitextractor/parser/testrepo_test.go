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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This file builds deterministic git repositories with the git command line.
// The collector tests use them as fixtures so that the fixtures do not depend
// on any of the git libraries under test.

const (
	testAuthorName  = "DevLake Test"
	testAuthorEmail = "test@devlake.apache.org"
)

var (
	testTimeBase = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	testTimeC1 = testTimeBase
	testTimeC2 = testTimeBase.Add(1 * time.Hour)
	testTimeC3 = testTimeBase.Add(2 * time.Hour)
	testTimeC4 = testTimeBase.Add(3 * time.Hour)
	testTimeC5 = testTimeBase.Add(4 * time.Hour)
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// gitFixture is a scriptable git repository. Commands run with an isolated
// git configuration and a fixed identity so the fixtures are reproducible.
type gitFixture struct {
	t   *testing.T
	dir string
}

func newGitFixture(t *testing.T) *gitFixture {
	t.Helper()
	requireGit(t)
	f := newGitFixtureAt(t, t.TempDir())
	f.git("init", "-q", "-b", "main")
	return f
}

func newGitFixtureAt(t *testing.T, dir string) *gitFixture {
	t.Helper()
	requireGit(t)
	return &gitFixture{t: t, dir: dir}
}

func (f *gitFixture) env(when time.Time) []string {
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+testAuthorName,
		"GIT_AUTHOR_EMAIL="+testAuthorEmail,
		"GIT_COMMITTER_NAME="+testAuthorName,
		"GIT_COMMITTER_EMAIL="+testAuthorEmail,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
		"TZ=UTC",
		// keep the fixtures independent of the host user configuration
		"HOME="+f.dir,
	)
	if !when.IsZero() {
		date := when.UTC().Format(time.RFC3339)
		env = append(env, "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	}
	return env
}

func (f *gitFixture) git(args ...string) string {
	f.t.Helper()
	return f.gitAt(time.Time{}, args...)
}

func (f *gitFixture) gitAt(when time.Time, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.dir
	cmd.Env = f.env(when)
	output, err := cmd.CombinedOutput()
	require.NoError(f.t, err, "git %v in %s: %s", args, f.dir, output)
	return strings.TrimSpace(string(output))
}

func (f *gitFixture) head() string {
	f.t.Helper()
	return f.git("rev-parse", "HEAD")
}

func (f *gitFixture) writeFile(rel, content string) {
	f.t.Helper()
	full := filepath.Join(f.dir, rel)
	require.NoError(f.t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(f.t, os.WriteFile(full, []byte(content), 0o644))
}

func (f *gitFixture) chmod(rel string, mode os.FileMode) {
	f.t.Helper()
	require.NoError(f.t, os.Chmod(filepath.Join(f.dir, rel), mode))
}

func (f *gitFixture) removeFile(rel string) {
	f.t.Helper()
	require.NoError(f.t, os.Remove(filepath.Join(f.dir, rel)))
}

// commit stages everything and commits with the fixture identity at when.
func (f *gitFixture) commit(when time.Time, message string) string {
	f.t.Helper()
	f.git("add", "-A")
	f.gitAt(when, "commit", "-q", "-m", message)
	return f.head()
}

func (f *gitFixture) checkout(branch string, create bool) {
	f.t.Helper()
	if create {
		f.git("checkout", "-q", "-b", branch)
		return
	}
	f.git("checkout", "-q", branch)
}

func (f *gitFixture) merge(when time.Time, message, branch string) string {
	f.t.Helper()
	f.gitAt(when, "merge", "-q", "--no-ff", "-m", message, branch)
	return f.head()
}

func (f *gitFixture) tag(name, target string, annotated bool) {
	f.t.Helper()
	if annotated {
		f.git("tag", "-a", name, target, "-m", "release "+name)
		return
	}
	f.git("tag", name, target)
}

// bareClone clones the fixture into a new bare repository, which is the shape
// the gitextractor produces before collecting.
func (f *gitFixture) bareClone() string {
	f.t.Helper()
	dir := filepath.Join(f.t.TempDir(), "repo.git")
	f.git("clone", "-q", "--bare", f.dir, dir)
	return dir
}

// shallowBareClone creates a depth-1 bare clone, so the tip commit is a
// shallow boundary whose parent is missing from the object database.
func (f *gitFixture) shallowBareClone() string {
	f.t.Helper()
	dir := filepath.Join(f.t.TempDir(), "shallow.git")
	f.git("clone", "-q", "--bare", "--depth=1", "file://"+f.dir, dir)
	return dir
}

// canonicalRepo is the repository used by the collector contract tests.
//
//	main:    c1 ---- c3 ---- c4 ---- c5 (merge)
//	           \                    /
//	feature:    c2 ----------------
//
// c1 adds a.txt (3 lines), a file in a directory with a space, a binary file
// and an empty file. c2 adds a file on the feature branch. c3 rewrites a line
// in a.txt, appends a line and deletes the empty file. c4 only changes the file
// mode of a.txt and rewrites the binary file. c5 merges feature.
// c1 carries the lightweight tag v1.0.0, c3 the annotated tag v2.0.0.
type canonicalRepo struct {
	Dir string
	C1  string
	C2  string
	C3  string
	C4  string
	C5  string
}

func buildCanonicalRepo(t *testing.T) canonicalRepo {
	t.Helper()
	f := newGitFixture(t)

	f.writeFile("a.txt", "l1\nl2\nl3\n")
	f.writeFile("dir with space/b.txt", "b1\n")
	f.writeFile("bin.dat", "before\x00binary\n")
	f.writeFile("empty.txt", "")
	c1 := f.commit(testTimeC1, "c1: initial import")

	f.checkout("feature", true)
	f.writeFile("feature.txt", "f1\n")
	c2 := f.commit(testTimeC2, "c2: add feature file")

	f.checkout("main", false)
	f.writeFile("a.txt", "l1\nL2\nl3\nl4\n")
	f.removeFile("empty.txt")
	c3 := f.commit(testTimeC3, "c3: reword and extend a.txt")

	f.chmod("a.txt", 0o755)
	f.writeFile("bin.dat", "after\x00binary\n")
	c4 := f.commit(testTimeC4, "c4: mode and binary change")

	c5 := f.merge(testTimeC5, "c5: merge feature", "feature")

	f.tag("v1.0.0", c1, false)
	f.tag("v2.0.0", c3, true)

	return canonicalRepo{
		Dir: f.bareClone(),
		C1:  c1,
		C2:  c2,
		C3:  c3,
		C4:  c4,
		C5:  c5,
	}
}

// buildLinearRepo creates a repository without merges for the diff line tests.
//
//	c1 adds a.txt and b.txt, c2 rewrites a line in a.txt, c3 appends a line to
//	a.txt and rewrites a line in b.txt, c4 deletes b.txt and adds c.txt.
type linearRepo struct {
	Dir string
	C1  string
	C2  string
	C3  string
	C4  string
}

func buildLinearRepo(t *testing.T) linearRepo {
	t.Helper()
	f := newGitFixture(t)

	f.writeFile("a.txt", "l1\nl2\nl3\n")
	f.writeFile("b.txt", "b1\nb2\n")
	c1 := f.commit(testTimeC1, "c1")

	f.writeFile("a.txt", "l1\nL2\nl3\n")
	c2 := f.commit(testTimeC2, "c2")

	f.writeFile("a.txt", "l1\nL2\nl3\nl4\n")
	f.writeFile("b.txt", "b1\nB2\n")
	c3 := f.commit(testTimeC3, "c3")

	f.removeFile("b.txt")
	f.writeFile("c.txt", "c1\nc2\n")
	c4 := f.commit(testTimeC4, "c4")

	return linearRepo{
		Dir: f.bareClone(),
		C1:  c1,
		C2:  c2,
		C3:  c3,
		C4:  c4,
	}
}

// blameAtHEAD returns file -> line number -> commit sha for every file in HEAD,
// using the git command line as an independent oracle.
func blameAtHEAD(t *testing.T, dir string) map[string]map[int]string {
	t.Helper()
	repo := newGitFixtureAt(t, dir)
	blames := make(map[string]map[int]string)
	files := strings.Split(strings.TrimRight(repo.git("ls-tree", "-r", "--name-only", "-z", "HEAD"), "\x00"), "\x00")
	for _, file := range files {
		if file == "" {
			continue
		}
		out := repo.git("blame", "-l", "--line-porcelain", "HEAD", "--", file)
		lines := make(map[int]string)
		for _, line := range strings.Split(out, "\n") {
			// The header of the first line of each group carries the group
			// size, later headers do not. Content lines are tab-prefixed.
			if strings.HasPrefix(line, "\t") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 3 || len(fields[0]) != 40 {
				continue
			}
			finalLine, err := strconv.Atoi(fields[2])
			if err != nil {
				continue
			}
			lines[finalLine] = fields[0]
		}
		blames[file] = lines
	}
	return blames
}
