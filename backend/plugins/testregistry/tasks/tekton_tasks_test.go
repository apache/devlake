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
	"regexp"
	"testing"

	"github.com/apache/devlake/core/errors"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	mocklog "github.com/apache/devlake/mocks/core/log"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/apache/devlake/plugins/testregistry/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSaveTektonTasks(t *testing.T) {
	t.Run("saves tasks successfully", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()
		mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()

		taskRuns := []TektonTaskRun{
			{Name: "build", Status: "Succeeded", Duration: "120s"},
			{Name: "test", Status: "Failed", Duration: "300s"},
		}
		err := saveTektonTasks(mockDal, mockLogger, 1, "job-1", taskRuns)
		assert.Nil(t, err)
		mockDal.AssertNumberOfCalls(t, "CreateOrUpdate", 2)
	})

	t.Run("empty task runs", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		err := saveTektonTasks(mockDal, mockLogger, 1, "job-1", []TektonTaskRun{})
		assert.Nil(t, err)
	})

	t.Run("skips task with empty name", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()

		taskRuns := []TektonTaskRun{{Name: "", Status: "Succeeded"}}
		err := saveTektonTasks(mockDal, mockLogger, 1, "job-1", taskRuns)
		assert.Nil(t, err)
		mockDal.AssertNotCalled(t, "CreateOrUpdate")
	})

	t.Run("unparseable duration uses 0", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()

		taskRuns := []TektonTaskRun{{Name: "task1", Status: "Succeeded", Duration: "invalid"}}
		err := saveTektonTasks(mockDal, mockLogger, 1, "job-1", taskRuns)
		assert.Nil(t, err)
	})

	t.Run("CreateOrUpdate error continues", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(errors.Default.New("db error"))
		mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()

		taskRuns := []TektonTaskRun{{Name: "task1", Status: "Failed"}}
		err := saveTektonTasks(mockDal, mockLogger, 1, "job-1", taskRuns)
		assert.Nil(t, err) // saveTektonTasks continues on error, returns nil
	})
}

// setupMockContext creates a mock SubTaskContext with logger and dal wired up.
// The dal mock is configured to allow CreateOrUpdate calls (for suite/test case saves).
func setupMockContext(t *testing.T) (*mockplugin.SubTaskContext, *mockdal.Dal, *mocklog.Logger) {
	t.Helper()
	mockCtx := new(mockplugin.SubTaskContext)
	mockDal := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)

	mockCtx.On("GetLogger").Return(mockLogger)
	mockCtx.On("GetDal").Return(mockDal)

	// Logger — the generated mock packs variadic args into a single slice arg
	mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()

	// Dal — CreateOrUpdate is called by saveSuiteRecursively and saveTestCase
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil).Maybe()

	return mockCtx, mockDal, mockLogger
}

const validJUnitXML = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="TestSuite1" tests="2" failures="1" time="1.5">
    <testcase name="TestPass" classname="pkg.Class" time="0.5"/>
    <testcase name="TestFail" classname="pkg.Class" time="1.0">
      <failure message="assertion failed">expected true got false</failure>
    </testcase>
  </testsuite>
</testsuites>`

func TestFindAndProcessJUnitFiles(t *testing.T) {
	ciJob := &models.TestRegistryCIJob{
		ConnectionId: 1,
		JobId:        "junit-test-job",
		JobName:      "e2e-tests",
		TriggerType:  "push",
		Result:       "FAILURE",
	}

	t.Run("single JUnit file found and processed", func(t *testing.T) {
		mockCtx, _, _ := setupMockContext(t)

		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "e2e-results.xml"), []byte(validJUnitXML), 0o644)
		assert.NoError(t, err)

		// Use a regex that matches the file we created
		re := regexp.MustCompile(`e2e-results\.xml`)
		result := findAndProcessJUnitFiles(mockCtx, dir, ciJob, "org", "repo", re)
		assert.True(t, result)
	})

	t.Run("no matching files returns false", func(t *testing.T) {
		mockCtx, _, _ := setupMockContext(t)

		dir := t.TempDir()
		// Empty directory — no files at all
		result := findAndProcessJUnitFiles(mockCtx, dir, ciJob, "org", "repo", regexp.MustCompile(`junit.*\.xml`))
		assert.False(t, result)
	})

	t.Run("multiple JUnit files", func(t *testing.T) {
		mockCtx, _, _ := setupMockContext(t)

		dir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "e2e-first.xml"), []byte(validJUnitXML), 0o644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "e2e-second.xml"), []byte(validJUnitXML), 0o644))

		re := regexp.MustCompile(`e2e-.*\.xml`)
		result := findAndProcessJUnitFiles(mockCtx, dir, ciJob, "org", "repo", re)
		assert.True(t, result)
	})

	t.Run("nil junitRegex uses default pattern", func(t *testing.T) {
		mockCtx, _, _ := setupMockContext(t)

		dir := t.TempDir()
		// File name must match the DefaultJUnitRegexPattern: (devlake-|e2e|qd-report-)[0-9a-z-]+\.(xml|junit)
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "e2e-abc123.xml"), []byte(validJUnitXML), 0o644))

		result := findAndProcessJUnitFiles(mockCtx, dir, ciJob, "org", "repo", nil)
		assert.True(t, result)
	})

	t.Run("unreadable file is skipped gracefully", func(t *testing.T) {
		mockCtx, _, _ := setupMockContext(t)

		dir := t.TempDir()
		filePath := filepath.Join(dir, "e2e-unreadable.xml")
		assert.NoError(t, os.WriteFile(filePath, []byte(validJUnitXML), 0o644))
		// Remove read permission so os.ReadFile fails
		assert.NoError(t, os.Chmod(filePath, 0o000))
		t.Cleanup(func() { os.Chmod(filePath, 0o644) })

		re := regexp.MustCompile(`e2e-.*\.xml`)
		result := findAndProcessJUnitFiles(mockCtx, dir, ciJob, "org", "repo", re)
		// The file is found but cannot be read, so no files are successfully processed
		assert.False(t, result)
	})

	t.Run("non-matching files ignored", func(t *testing.T) {
		mockCtx, _, _ := setupMockContext(t)

		dir := t.TempDir()
		// Create files that do NOT match the regex
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "output.log"), []byte("log data"), 0o644))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "report.json"), []byte("{}"), 0o644))

		re := regexp.MustCompile(`e2e-.*\.xml`)
		result := findAndProcessJUnitFiles(mockCtx, dir, ciJob, "org", "repo", re)
		assert.False(t, result)
	})

	t.Run("nonexistent directory returns false", func(t *testing.T) {
		mockCtx, _, _ := setupMockContext(t)

		result := findAndProcessJUnitFiles(mockCtx, "/nonexistent/path/to/artifacts", ciJob, "org", "repo", regexp.MustCompile(`.*\.xml`))
		assert.False(t, result)
	})

	t.Run("JUnit file in subdirectory is found", func(t *testing.T) {
		mockCtx, _, _ := setupMockContext(t)

		dir := t.TempDir()
		subDir := filepath.Join(dir, "artifacts", "test-output")
		assert.NoError(t, os.MkdirAll(subDir, 0o755))
		assert.NoError(t, os.WriteFile(filepath.Join(subDir, "e2e-deep.xml"), []byte(validJUnitXML), 0o644))

		re := regexp.MustCompile(`e2e-.*\.xml`)
		result := findAndProcessJUnitFiles(mockCtx, dir, ciJob, "org", "repo", re)
		assert.True(t, result)
	})
}
