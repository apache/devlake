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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNumstatStream(t *testing.T) {
	stream := strings.Join([]string{
		"\x01sha1\x00",
		"\n1\t0\ta.txt\x00",
		"-\t-\tbin.dat\x00",
		"\x01sha2\x00",
		"\x01sha3\x00",
		"\n2\t1\tdir with space/f.txt\x00",
		"1\t0\tüñï.txt\x00",
		"0\t0\tempty.txt\x00",
		"1\t0\tnew\nline.txt\x00",
	}, "")

	var commits []*commitStats
	err := parseNumstatStream(strings.NewReader(stream), func(commit *commitStats) error {
		commits = append(commits, commit)
		return nil
	})
	require.NoError(t, err)

	require.Len(t, commits, 3)
	assert.Equal(t, "sha1", commits[0].Sha)
	assert.Equal(t, []commitFileStat{
		{Path: "a.txt", Additions: 1},
		{Path: "bin.dat"},
	}, commits[0].Files)
	assert.Equal(t, "sha2", commits[1].Sha)
	assert.Empty(t, commits[1].Files)
	assert.Equal(t, []commitFileStat{
		{Path: "dir with space/f.txt", Additions: 2, Deletions: 1},
		{Path: "üñï.txt", Additions: 1},
		{Path: "empty.txt"},
		{Path: "new\nline.txt", Additions: 1},
	}, commits[2].Files)
}

func TestParseNumstatStreamRejectsBadInput(t *testing.T) {
	err := parseNumstatStream(strings.NewReader("1\t0\ta.txt\x00"), func(*commitStats) error { return nil })
	assert.Error(t, err, "an entry without a commit marker is invalid")

	err = parseNumstatStream(strings.NewReader("\x01sha\x00\nx\t0\ta.txt\x00"), func(*commitStats) error { return nil })
	assert.Error(t, err, "a non-numeric count is invalid")
}

func TestParsePatchStream(t *testing.T) {
	patch := strings.Join([]string{
		"\x01c1",
		"diff --git a/a.txt b/a.txt",
		"new file mode 100644",
		"index 0000000..1111111",
		"--- /dev/null",
		"+++ b/a.txt",
		"@@ -0,0 +1,3 @@",
		"+l1",
		"+l2",
		"+l3",
		"\x01c2",
		"diff --git a/a.txt b/a.txt",
		"index 1111111..2222222 100644",
		"--- a/a.txt",
		"+++ b/a.txt",
		"@@ -1,3 +1,4 @@ heading",
		" l1",
		"-l2",
		"+L2",
		" l3",
		"+l4",
		"\\ No newline at end of file",
		"diff --git a/bin.dat b/bin.dat",
		"index 3333333..4444444 100644",
		"Binary files a/bin.dat and b/bin.dat differ",
		"diff --git a/empty.txt b/empty.txt",
		"deleted file mode 100644",
		"index 5555555..0000000",
		"--- a/empty.txt",
		"+++ /dev/null",
		"@@ -1 +0,0 @@",
		"-x",
		"diff --git a/mode.txt b/mode.txt",
		"old mode 100644",
		"new mode 100755",
		"diff --git \"a/tab\\tname.txt\" \"b/tab\\tname.txt\"",
		"index 6666666..7777777 100644",
		"--- \"a/tab\\tname.txt\"",
		"+++ \"b/tab\\tname.txt\"",
		"@@ -1 +1 @@",
		"-a",
		"+b",
		"",
	}, "\n")

	var commits []*commitPatch
	err := parsePatchStream(strings.NewReader(patch), func(commit *commitPatch) error {
		commits = append(commits, commit)
		return nil
	})
	require.NoError(t, err)

	require.Len(t, commits, 2)

	c1 := commits[0]
	assert.Equal(t, "c1", c1.Sha)
	require.Len(t, c1.Files, 1)
	assert.Equal(t, patchFile{
		OldPath: "a.txt",
		NewPath: "a.txt",
		Lines: []patchLine{
			{Origin: '+', OldLine: -1, NewLine: 1, HunkNum: 1},
			{Origin: '+', OldLine: -1, NewLine: 2, HunkNum: 1},
			{Origin: '+', OldLine: -1, NewLine: 3, HunkNum: 1},
		},
	}, c1.Files[0])

	c2 := commits[1]
	assert.Equal(t, "c2", c2.Sha)
	require.Len(t, c2.Files, 5)

	assert.Equal(t, patchFile{
		OldPath: "a.txt",
		NewPath: "a.txt",
		Lines: []patchLine{
			{Origin: ' ', OldLine: 1, NewLine: 1, HunkNum: 1},
			{Origin: '-', OldLine: 2, NewLine: -1, HunkNum: 1},
			{Origin: '+', OldLine: -1, NewLine: 2, HunkNum: 1},
			{Origin: ' ', OldLine: 3, NewLine: 3, HunkNum: 1},
			{Origin: '+', OldLine: -1, NewLine: 4, HunkNum: 1},
		},
	}, c2.Files[0])

	// binary file: both paths are known, but there are no line changes
	assert.Equal(t, patchFile{OldPath: "bin.dat", NewPath: "bin.dat"}, c2.Files[1])

	// deleted file: libgit2 reports the path on both sides
	assert.Equal(t, patchFile{
		OldPath: "empty.txt",
		NewPath: "empty.txt",
		Lines:   []patchLine{{Origin: '-', OldLine: 1, NewLine: -1, HunkNum: 1}},
	}, c2.Files[2])

	// mode-only change
	assert.Equal(t, patchFile{OldPath: "mode.txt", NewPath: "mode.txt"}, c2.Files[3])

	// quoted path
	assert.Equal(t, patchFile{
		OldPath: "tab\tname.txt",
		NewPath: "tab\tname.txt",
		Lines: []patchLine{
			{Origin: '-', OldLine: 1, NewLine: -1, HunkNum: 1},
			{Origin: '+', OldLine: -1, NewLine: 1, HunkNum: 1},
		},
	}, c2.Files[4])
}

func TestParsePatchStreamMultipleHunksAndZeroStart(t *testing.T) {
	patch := strings.Join([]string{
		"\x01c1",
		"diff --git a/a.txt b/a.txt",
		"--- a/a.txt",
		"+++ b/a.txt",
		"@@ -2,1 +2,1 @@",
		"-old",
		"+new",
		"@@ -10,0 +11,2 @@",
		"+x",
		"+y",
		"",
	}, "\n")

	var commits []*commitPatch
	require.NoError(t, parsePatchStream(strings.NewReader(patch), func(commit *commitPatch) error {
		commits = append(commits, commit)
		return nil
	}))
	require.Len(t, commits, 1)
	assert.Equal(t, []patchLine{
		{Origin: '-', OldLine: 2, NewLine: -1, HunkNum: 1},
		{Origin: '+', OldLine: -1, NewLine: 2, HunkNum: 1},
		{Origin: '+', OldLine: -1, NewLine: 11, HunkNum: 2},
		{Origin: '+', OldLine: -1, NewLine: 12, HunkNum: 2},
	}, commits[0].Files[0].Lines)
}

func TestParseGitVersion(t *testing.T) {
	for _, tc := range []struct {
		output string
		want   int
	}{
		{"git version 2.43.0\n", 2*10000 + 43*100},
		{"git version 2.31.0", 2*10000 + 31*100},
		{"git version 2.39.2.windows.1", 2*10000 + 39*100 + 2},
		{"git version 2.56.0", 2*10000 + 56*100},
	} {
		got, err := parseGitVersion(tc.output)
		if assert.NoError(t, err, tc.output) {
			assert.Equal(t, tc.want, got, tc.output)
		}
	}

	_, err := parseGitVersion("not git")
	assert.Error(t, err)
}

func TestUnquoteGitPath(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{`a/plain.txt`, `a/plain.txt`},
		{`"a/tab\tname.txt"`, "a/tab\tname.txt"},
		{`"a/quote\"name.txt"`, `a/quote"name.txt`},
		{`"a/back\\slash"`, `a/back\slash`},
		{`"a/\303\274ber.txt"`, "a/über.txt"},
		{`"a/new\nline.txt"`, "a/new\nline.txt"},
	} {
		assert.Equal(t, tc.want, unquoteGitPath(tc.in), tc.in)
	}
}
