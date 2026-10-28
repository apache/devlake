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

	"github.com/apache/devlake/core/errors"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	mocklog "github.com/apache/devlake/mocks/core/log"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/apache/devlake/plugins/testregistry/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDetermineJobTypeForGCS(t *testing.T) {
	t.Run("pull_request maps to presubmit", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{TriggerType: "pull_request"}
		result, err := determineJobTypeForGCS(ciJob, &ProwJob{})
		assert.Nil(t, err)
		assert.Equal(t, "presubmit", result)
	})

	t.Run("push maps to postsubmit", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{TriggerType: "push"}
		result, err := determineJobTypeForGCS(ciJob, &ProwJob{})
		assert.Nil(t, err)
		assert.Equal(t, "postsubmit", result)
	})

	t.Run("periodic maps to periodic", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{TriggerType: "periodic"}
		result, err := determineJobTypeForGCS(ciJob, &ProwJob{})
		assert.Nil(t, err)
		assert.Equal(t, "periodic", result)
	})

	t.Run("unknown falls back to prow spec type", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{TriggerType: "custom"}
		prowJob := &ProwJob{Spec: ProwJobSpec{Type: "Presubmit"}}
		result, err := determineJobTypeForGCS(ciJob, prowJob)
		assert.Nil(t, err)
		assert.Equal(t, "presubmit", result)
	})

	t.Run("unknown with no fallback returns error", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{TriggerType: "custom"}
		_, err := determineJobTypeForGCS(ciJob, &ProwJob{})
		assert.NotNil(t, err)
	})
}

func TestExtractPullRequestNumber(t *testing.T) {
	t.Run("returns PR number for pull_request trigger", func(t *testing.T) {
		prNum := 42
		ciJob := &models.TestRegistryCIJob{
			TriggerType:       "pull_request",
			PullRequestNumber: &prNum,
		}
		assert.Equal(t, "42", extractPullRequestNumber(ciJob))
	})

	t.Run("returns empty for non-pull_request trigger", func(t *testing.T) {
		prNum := 42
		ciJob := &models.TestRegistryCIJob{
			TriggerType:       "push",
			PullRequestNumber: &prNum,
		}
		assert.Equal(t, "", extractPullRequestNumber(ciJob))
	})

	t.Run("returns empty when PR number is nil", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{
			TriggerType: "pull_request",
		}
		assert.Equal(t, "", extractPullRequestNumber(ciJob))
	})
}

func TestStringPtrOrNil(t *testing.T) {
	t.Run("non-empty returns pointer", func(t *testing.T) {
		result := stringPtrOrNil("hello")
		assert.NotNil(t, result)
		assert.Equal(t, "hello", *result)
	})

	t.Run("empty returns nil", func(t *testing.T) {
		result := stringPtrOrNil("")
		assert.Nil(t, result)
	})
}

func TestGenerateUID(t *testing.T) {
	t.Run("returns 16-char string", func(t *testing.T) {
		uid := generateUID()
		assert.Len(t, uid, 16)
	})

	t.Run("two calls produce different UIDs", func(t *testing.T) {
		uid1 := generateUID()
		uid2 := generateUID()
		assert.NotEqual(t, uid1, uid2)
	})

	t.Run("only contains valid characters", func(t *testing.T) {
		uid := generateUID()
		for _, c := range uid {
			assert.Contains(t, uidChars, string(c))
		}
	})
}

func TestIsJobAlreadyProcessed(t *testing.T) {
	t.Run("count > 0 returns true", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Count", mock.Anything).Return(int64(5), nil)
		assert.True(t, isJobAlreadyProcessed(mockDal, 1, "job-1"))
	})

	t.Run("count = 0 returns false", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Count", mock.Anything).Return(int64(0), nil)
		assert.False(t, isJobAlreadyProcessed(mockDal, 1, "job-1"))
	})

	t.Run("error returns false", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Count", mock.Anything).Return(int64(0), errors.Default.New("db error"))
		assert.False(t, isJobAlreadyProcessed(mockDal, 1, "job-1"))
	})
}

func TestLogSuiteInfo(t *testing.T) {
	mockLogger := new(mocklog.Logger)
	mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()

	suite := &TestSuite{
		Name:       "TestSuite1",
		NumTests:   10,
		NumFailed:  2,
		NumSkipped: 1,
		Duration:   5.5,
	}

	// Should not panic
	logSuiteInfo(mockLogger, suite, "job-1", 1, 0)
	mockLogger.AssertCalled(t, "Info", mock.Anything, mock.Anything)
}

func TestSaveTestCase(t *testing.T) {
	t.Run("passing test", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

		tc := &TestCase{Name: "TestFoo", Classname: "pkg.Foo", Duration: 1.5}
		err := saveTestCase(mockDal, mockLogger, tc, 1, "job-1", "suite-1")
		assert.Nil(t, err)
		mockDal.AssertCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything)
	})

	t.Run("failed test", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

		tc := &TestCase{
			Name: "TestBar",
			FailureOutput: &FailureOutput{Message: "assertion failed", Output: "expected true"},
		}
		err := saveTestCase(mockDal, mockLogger, tc, 1, "job-1", "suite-1")
		assert.Nil(t, err)
	})

	t.Run("skipped test", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

		tc := &TestCase{
			Name:        "TestSkipped",
			SkipMessage: &SkipMessage{Message: "not implemented"},
		}
		err := saveTestCase(mockDal, mockLogger, tc, 1, "job-1", "suite-1")
		assert.Nil(t, err)
	})

	t.Run("CreateOrUpdate error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(errors.Default.New("db error"))

		tc := &TestCase{Name: "TestErr"}
		err := saveTestCase(mockDal, mockLogger, tc, 1, "job-1", "suite-1")
		assert.NotNil(t, err)
	})
}

func TestSaveSuiteRecursively(t *testing.T) {
	t.Run("nil suite returns 0,0", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		s, tc := saveSuiteRecursively(mockDal, mockLogger, nil, 1, "job-1", nil)
		assert.Equal(t, 0, s)
		assert.Equal(t, 0, tc)
	})

	t.Run("empty name returns 0,0", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		suite := &TestSuite{Name: ""}
		s, tc := saveSuiteRecursively(mockDal, mockLogger, suite, 1, "job-1", nil)
		assert.Equal(t, 0, s)
		assert.Equal(t, 0, tc)
	})

	t.Run("suite with one test case", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()
		mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()

		suite := &TestSuite{
			Name:     "MySuite",
			NumTests: 1,
			TestCases: []*TestCase{
				{Name: "TestFoo", Duration: 1.0},
			},
		}
		s, tc := saveSuiteRecursively(mockDal, mockLogger, suite, 1, "job-1", nil)
		assert.Equal(t, 1, s)
		assert.Equal(t, 1, tc)
	})

	t.Run("suite with nested child", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()
		mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()

		child := &TestSuite{
			Name: "ChildSuite",
			TestCases: []*TestCase{
				{Name: "ChildTest"},
			},
		}
		suite := &TestSuite{
			Name:     "ParentSuite",
			Children: []*TestSuite{child},
		}
		s, tc := saveSuiteRecursively(mockDal, mockLogger, suite, 1, "job-1", nil)
		assert.Equal(t, 2, s)
		assert.Equal(t, 1, tc)
	})

	t.Run("suite with properties", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()

		suite := &TestSuite{
			Name: "PropSuite",
			Properties: []*TestSuiteProperty{
				{Name: "key1", Value: "val1"},
			},
		}
		s, tc := saveSuiteRecursively(mockDal, mockLogger, suite, 1, "job-1", nil)
		assert.Equal(t, 1, s)
		assert.Equal(t, 0, tc)
	})

	t.Run("CreateOrUpdate error on suite", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(errors.Default.New("db error"))
		mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()

		suite := &TestSuite{Name: "FailSuite"}
		s, tc := saveSuiteRecursively(mockDal, mockLogger, suite, 1, "job-1", nil)
		assert.Equal(t, 0, s)
		assert.Equal(t, 0, tc)
	})
}

func TestParseAndSaveJUnitSuites(t *testing.T) {
	t.Run("valid XML with one suite", func(t *testing.T) {
		mockCtx := new(mockplugin.SubTaskContext)
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)

		mockCtx.On("GetDal").Return(mockDal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)
		mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()

		xmlData := []byte(`<testsuites>
			<testsuite name="TestSuite1" tests="1" failures="0">
				<testcase name="TestFoo" classname="pkg.Foo" time="1.5"/>
			</testsuite>
		</testsuites>`)

		ciJob := &models.TestRegistryCIJob{ConnectionId: 1, JobId: "job-1", JobName: "test", TriggerType: "push", Result: "SUCCESS"}
		result := parseAndSaveJUnitSuites(mockCtx, mockLogger, xmlData, "junit.xml", ciJob, "org", "repo")
		assert.True(t, result)
	})

	t.Run("empty suites bytes", func(t *testing.T) {
		mockCtx := new(mockplugin.SubTaskContext)
		mockLogger := new(mocklog.Logger)
		mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()

		ciJob := &models.TestRegistryCIJob{JobId: "job-1", JobName: "test"}
		result := parseAndSaveJUnitSuites(mockCtx, mockLogger, []byte{}, "junit.xml", ciJob, "org", "repo")
		assert.False(t, result)
	})

	t.Run("invalid XML", func(t *testing.T) {
		mockCtx := new(mockplugin.SubTaskContext)
		mockLogger := new(mocklog.Logger)
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()

		ciJob := &models.TestRegistryCIJob{JobId: "job-1", JobName: "test"}
		result := parseAndSaveJUnitSuites(mockCtx, mockLogger, []byte("not xml"), "junit.xml", ciJob, "org", "repo")
		assert.False(t, result)
	})

	t.Run("bare testsuite root element returns false", func(t *testing.T) {
		mockCtx := new(mockplugin.SubTaskContext)
		mockLogger := new(mocklog.Logger)

		// xml.Unmarshal returns an error when root is <testsuite> instead of <testsuites>
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()

		xmlData := []byte(`<testsuite name="BareSuite" tests="1"><testcase name="Test1"/></testsuite>`)
		ciJob := &models.TestRegistryCIJob{ConnectionId: 1, JobId: "job-1", JobName: "test", Result: "SUCCESS"}
		result := parseAndSaveJUnitSuites(mockCtx, mockLogger, xmlData, "junit.xml", ciJob, "org", "repo")
		assert.False(t, result)
	})

	t.Run("testsuites with empty suites and bare fallback", func(t *testing.T) {
		mockCtx := new(mockplugin.SubTaskContext)
		mockLogger := new(mocklog.Logger)

		mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()

		// Empty testsuites wrapper triggers the fallback path, but since it's a valid
		// <testsuites/> with no children, the single suite fallback won't match either
		xmlData := []byte(`<testsuites></testsuites>`)
		ciJob := &models.TestRegistryCIJob{ConnectionId: 1, JobId: "job-1", JobName: "test", Result: "SUCCESS"}
		result := parseAndSaveJUnitSuites(mockCtx, mockLogger, xmlData, "junit.xml", ciJob, "org", "repo")
		assert.False(t, result)
	})
}
