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
	"bufio"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rawCommitObject = `tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904
parent 1111111111111111111111111111111111111111
parent 2222222222222222222222222222222222222222
author Jane Doe <jane@example.com> 1704067200 +0800
committer DevLake Test <test@devlake.apache.org> 1704070800 -0530
gpgsig -----BEGIN PGP SIGNATURE-----
 
 fakesignature
 -----END PGP SIGNATURE-----

subject line

body line 1
body line 2
`

func TestParseCommitObject(t *testing.T) {
	commit, err := parseCommitObject("abc", []byte(rawCommitObject))
	require.NoError(t, err)

	assert.Equal(t, "abc", commit.Sha)
	assert.Equal(t, []string{
		"1111111111111111111111111111111111111111",
		"2222222222222222222222222222222222222222",
	}, commit.Parents)

	assert.Equal(t, "Jane Doe", commit.Author.Name)
	assert.Equal(t, "jane@example.com", commit.Author.Email)
	assert.Equal(t, int64(1704067200), commit.Author.When.Unix())
	_, offset := commit.Author.When.Zone()
	assert.Equal(t, 8*3600, offset)

	assert.Equal(t, "DevLake Test", commit.Committer.Name)
	assert.Equal(t, "test@devlake.apache.org", commit.Committer.Email)
	_, offset = commit.Committer.When.Zone()
	assert.Equal(t, -(5*3600 + 30*60), offset)

	assert.Equal(t, "subject line\n\nbody line 1\nbody line 2\n", commit.Message)
}

func TestParseCommitObjectWithoutMessage(t *testing.T) {
	content := "tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n" +
		"author A <a@x> 1704067200 +0000\n" +
		"committer A <a@x> 1704067200 +0000\n"

	commit, err := parseCommitObject("abc", []byte(content))
	require.NoError(t, err)
	assert.Empty(t, commit.Parents)
	assert.Empty(t, commit.Message)
	assert.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), commit.Author.When.UTC())
}

func TestParseSignature(t *testing.T) {
	signature, err := parseSignature("A U Thor <a@example.com> 1704067200 +0000")
	require.NoError(t, err)
	assert.Equal(t, "A U Thor", signature.Name)
	assert.Equal(t, "a@example.com", signature.Email)

	// a name containing '<' still resolves to the last email-like part
	signature, err = parseSignature("Weird <Name> <weird@example.com> 1704067200 +0000")
	require.NoError(t, err)
	assert.Equal(t, "Weird <Name>", signature.Name)
	assert.Equal(t, "weird@example.com", signature.Email)

	_, err = parseSignature("no timestamp")
	assert.Error(t, err)
}

func TestReadCatFileObject(t *testing.T) {
	stream := "abc commit 5\nhello\n" + "def commit 0\n\n"
	reader := bufio.NewReader(strings.NewReader(stream))

	sha, content, err := readCatFileObject(reader)
	require.NoError(t, err)
	assert.Equal(t, "abc", sha)
	assert.Equal(t, "hello", string(content))

	sha, content, err = readCatFileObject(reader)
	require.NoError(t, err)
	assert.Equal(t, "def", sha)
	assert.Empty(t, content)

	_, _, err = readCatFileObject(reader)
	assert.ErrorIs(t, err, io.EOF)
}
