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
	"os"
	"path/filepath"
	"testing"

	mocklog "github.com/apache/devlake/mocks/core/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newMockLogger() *mocklog.Logger {
	logger := new(mocklog.Logger)
	logger.On("Info", mock.Anything, mock.Anything).Maybe()
	logger.On("Debug", mock.Anything, mock.Anything).Maybe()
	logger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	return logger
}

func TestExtractTektonPipelineRuns(t *testing.T) {
	ctx := context.Background()

	t.Run("valid pipeline-status.json", func(t *testing.T) {
		logger := newMockLogger()

		dir := t.TempDir()
		subDir := filepath.Join(dir, "run-1")
		assert.NoError(t, os.MkdirAll(subDir, 0o755))

		json := `{
			"pipelineRunName": "test-run-1",
			"status": "Succeeded",
			"namespace": "test-ns",
			"duration": "120s",
			"eventType": "push",
			"scenario": "e2e-test",
			"git": {
				"gitOrganization": "org",
				"gitRepository": "repo",
				"pullRequestNumber": "",
				"commitSha": "abc123"
			},
			"timestamps": {
				"createdAt": "2024-06-18T10:15:30Z",
				"startedAt": "2024-06-18T10:16:00Z",
				"finishedAt": "2024-06-18T11:20:06Z"
			}
		}`
		assert.NoError(t, os.WriteFile(filepath.Join(subDir, "pipeline-status.json"), []byte(json), 0o644))

		runs, err := extractTektonPipelineRuns(ctx, nil, dir, "/tmp/logs", logger)
		assert.Nil(t, err)
		assert.Len(t, runs, 1)
		assert.Equal(t, "test-run-1", runs[0].PipelineRunName)
		assert.Equal(t, "Succeeded", runs[0].Status)
		assert.Equal(t, "test-ns", runs[0].Namespace)
		assert.Equal(t, "120s", runs[0].Duration)
		assert.Equal(t, "org", runs[0].Git.GitOrganization)
		assert.Equal(t, "repo", runs[0].Git.GitRepository)
		assert.Equal(t, "abc123", runs[0].Git.CommitSha)
	})

	t.Run("missing required fields skips entry", func(t *testing.T) {
		logger := newMockLogger()

		dir := t.TempDir()
		subDir := filepath.Join(dir, "run-empty")
		assert.NoError(t, os.MkdirAll(subDir, 0o755))

		// PipelineRunName is empty — should be skipped
		json := `{"pipelineRunName": "", "status": "Failed"}`
		assert.NoError(t, os.WriteFile(filepath.Join(subDir, "pipeline-status.json"), []byte(json), 0o644))

		runs, err := extractTektonPipelineRuns(ctx, nil, dir, "/tmp/logs", logger)
		assert.Nil(t, err)
		assert.Empty(t, runs)
	})

	t.Run("missing status skips entry", func(t *testing.T) {
		logger := newMockLogger()

		dir := t.TempDir()
		subDir := filepath.Join(dir, "run-nostatus")
		assert.NoError(t, os.MkdirAll(subDir, 0o755))

		json := `{"pipelineRunName": "run-1", "status": ""}`
		assert.NoError(t, os.WriteFile(filepath.Join(subDir, "pipeline-status.json"), []byte(json), 0o644))

		runs, err := extractTektonPipelineRuns(ctx, nil, dir, "/tmp/logs", logger)
		assert.Nil(t, err)
		assert.Empty(t, runs)
	})

	t.Run("invalid JSON is skipped", func(t *testing.T) {
		logger := newMockLogger()

		dir := t.TempDir()
		subDir := filepath.Join(dir, "run-invalid")
		assert.NoError(t, os.MkdirAll(subDir, 0o755))

		assert.NoError(t, os.WriteFile(filepath.Join(subDir, "pipeline-status.json"), []byte("{not valid json"), 0o644))

		runs, err := extractTektonPipelineRuns(ctx, nil, dir, "/tmp/logs", logger)
		assert.Nil(t, err)
		assert.Empty(t, runs)
	})

	t.Run("no pipeline-status.json files", func(t *testing.T) {
		logger := newMockLogger()

		dir := t.TempDir()
		// Empty directory — no files
		runs, err := extractTektonPipelineRuns(ctx, nil, dir, "/tmp/logs", logger)
		assert.Nil(t, err)
		assert.Empty(t, runs)
	})

	t.Run("multiple pipeline-status.json files", func(t *testing.T) {
		logger := newMockLogger()

		dir := t.TempDir()

		// First pipeline run in subdir1
		sub1 := filepath.Join(dir, "subdir1")
		assert.NoError(t, os.MkdirAll(sub1, 0o755))
		json1 := `{"pipelineRunName": "run-alpha", "status": "Succeeded", "namespace": "ns1"}`
		assert.NoError(t, os.WriteFile(filepath.Join(sub1, "pipeline-status.json"), []byte(json1), 0o644))

		// Second pipeline run in subdir2
		sub2 := filepath.Join(dir, "subdir2")
		assert.NoError(t, os.MkdirAll(sub2, 0o755))
		json2 := `{"pipelineRunName": "run-beta", "status": "Failed", "namespace": "ns2"}`
		assert.NoError(t, os.WriteFile(filepath.Join(sub2, "pipeline-status.json"), []byte(json2), 0o644))

		runs, err := extractTektonPipelineRuns(ctx, nil, dir, "/tmp/logs", logger)
		assert.Nil(t, err)
		assert.Len(t, runs, 2)

		// Collect names since walk order may vary
		names := make(map[string]bool)
		for _, r := range runs {
			names[r.PipelineRunName] = true
		}
		assert.True(t, names["run-alpha"])
		assert.True(t, names["run-beta"])
	})

	t.Run("nonexistent directory returns error", func(t *testing.T) {
		logger := newMockLogger()

		runs, err := extractTektonPipelineRuns(ctx, nil, "/nonexistent/path", "/tmp/logs", logger)
		assert.NotNil(t, err)
		assert.Nil(t, runs)
	})

	t.Run("other files are ignored", func(t *testing.T) {
		logger := newMockLogger()

		dir := t.TempDir()
		// Write a file that is NOT pipeline-status.json
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "other-file.json"), []byte(`{"key":"val"}`), 0o644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "pipeline-status.txt"), []byte(`not json`), 0o644))

		runs, err := extractTektonPipelineRuns(ctx, nil, dir, "/tmp/logs", logger)
		assert.Nil(t, err)
		assert.Empty(t, runs)
	})
}
