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
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareOptionsNormalizesLocalProjectPath(t *testing.T) {
	baseDir := t.TempDir()
	projectDir := filepath.Join(baseDir, "nested", "..", "project")

	op := &DbtOptions{ProjectPath: projectDir}
	if err := PrepareOptions(op, baseDir); err != nil {
		t.Fatalf("PrepareOptions returned error: %v", err)
	}

	want := filepath.Join(baseDir, "project")
	if op.ProjectPath != want {
		t.Fatalf("ProjectPath = %q, want %q", op.ProjectPath, want)
	}
	if op.ProjectBaseDir != baseDir {
		t.Fatalf("ProjectBaseDir = %q, want %q", op.ProjectBaseDir, baseDir)
	}
	if op.ManagedProjectDir {
		t.Fatal("ManagedProjectDir should be false for local projects")
	}
}

func TestPrepareOptionsRejectsPathTraversal(t *testing.T) {
	baseDir := t.TempDir()
	op := &DbtOptions{ProjectPath: filepath.Join(baseDir, "..", "outside")}

	if err := PrepareOptions(op, baseDir); err == nil {
		t.Fatal("PrepareOptions should reject projectPath outside the configured base directory")
	}
}

func TestPrepareOptionsRejectsUnsafeGitURLs(t *testing.T) {
	baseDir := t.TempDir()
	cases := []string{
		"file:///tmp/evil",
		"../relative/repo.git",
		"git@github.com:apache/incubator-devlake.git",
	}

	for _, rawURL := range cases {
		t.Run(rawURL, func(t *testing.T) {
			op := &DbtOptions{ProjectGitURL: rawURL}
			if err := PrepareOptions(op, baseDir); err == nil {
				t.Fatalf("PrepareOptions should reject %q", rawURL)
			}
		})
	}
}

func TestPrepareOptionsAllowsManagedGitClone(t *testing.T) {
	baseDir := t.TempDir()
	op := &DbtOptions{
		ProjectPath:   filepath.Join(baseDir, "ignored"),
		ProjectGitURL: "https://github.com/apache/incubator-devlake.git",
	}

	if err := PrepareOptions(op, baseDir); err != nil {
		t.Fatalf("PrepareOptions returned error: %v", err)
	}
	if !op.ManagedProjectDir {
		t.Fatal("ManagedProjectDir should be true for git-backed projects")
	}
	if op.ProjectPath != "" {
		t.Fatalf("ProjectPath = %q, want empty for managed git clones", op.ProjectPath)
	}
}

func TestCleanupManagedProjectDirRemovesManagedPath(t *testing.T) {
	baseDir := t.TempDir()
	projectDir := filepath.Join(baseDir, "clone")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	op := &DbtOptions{
		ProjectBaseDir:    baseDir,
		ProjectPath:       projectDir,
		ManagedProjectDir: true,
	}
	if err := CleanupManagedProjectDir(op); err != nil {
		t.Fatalf("CleanupManagedProjectDir returned error: %v", err)
	}
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		t.Fatalf("expected %q to be removed, stat err = %v", projectDir, err)
	}
}
