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
	"time"

	"github.com/apache/devlake/core/errors"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	mocklog "github.com/apache/devlake/mocks/core/log"
	"github.com/apache/devlake/plugins/testregistry/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestConvertTektonPipelineRunToCIJob(t *testing.T) {
	t.Run("full conversion with pull_request", func(t *testing.T) {
		pr := &TektonPipelineRun{
			PipelineRunName: "run-abc",
			Namespace:       "ci-ns",
			Duration:        "600s",
			Status:          "Succeeded",
			EventType:       "pull_request",
			Scenario:        "e2e-test",
			ConsoleUrl:      "https://console.example.com/run-abc",
			Git: TektonGitInfo{
				GitOrganization:   "konflux-ci",
				GitRepository:     "integration-service",
				PullRequestNumber: "42",
				CommitSha:         "abc123def",
				PullRequestAuthor: "developer1",
			},
			Timestamps: TektonTimestamps{
				CreatedAt:  "2024-06-15T10:00:00Z",
				StartedAt:  "2024-06-15T10:01:00Z",
				FinishedAt: "2024-06-15T10:11:00Z",
			},
		}

		ciJob, err := convertTektonPipelineRunToCIJob(pr, 1, "scope-1", "quay-org", "repo")
		assert.Nil(t, err)
		assert.Equal(t, "run-abc", ciJob.JobId)
		assert.Equal(t, "e2e-test", ciJob.JobName)
		assert.Equal(t, "tekton", ciJob.JobType)
		assert.Equal(t, "konflux-ci", ciJob.Organization)
		assert.Equal(t, "integration-service", ciJob.Repository)
		assert.Equal(t, "pull_request", ciJob.TriggerType)
		assert.Equal(t, "SUCCESS", ciJob.Result)
		assert.Equal(t, "ci-ns", ciJob.Namespace)
		assert.Equal(t, "abc123def", ciJob.CommitSHA)
		assert.NotNil(t, ciJob.PullRequestNumber)
		assert.Equal(t, 42, *ciJob.PullRequestNumber)
		assert.Equal(t, "developer1", ciJob.PullRequestAuthor)
		assert.Equal(t, "https://console.example.com/run-abc", ciJob.ViewURL)
		assert.NotNil(t, ciJob.DurationSec)
		assert.InDelta(t, 600.0, *ciJob.DurationSec, 0.001)
		assert.NotNil(t, ciJob.QueuedAt)
		assert.NotNil(t, ciJob.StartedAt)
		assert.NotNil(t, ciJob.FinishedAt)
		assert.NotNil(t, ciJob.QueuedDurationSec)
		assert.InDelta(t, 60.0, *ciJob.QueuedDurationSec, 0.001)
	})

	t.Run("push event type", func(t *testing.T) {
		pr := &TektonPipelineRun{
			PipelineRunName: "run-push",
			Status:          "Failed",
			EventType:       "push",
			Scenario:        "build",
			Git: TektonGitInfo{
				GitOrganization: "org",
				GitRepository:   "repo",
				CommitSha:       "sha999",
			},
		}

		ciJob, err := convertTektonPipelineRunToCIJob(pr, 1, "scope", "org", "repo")
		assert.Nil(t, err)
		assert.Equal(t, "push", ciJob.TriggerType)
		assert.Equal(t, "FAILURE", ciJob.Result)
		assert.Nil(t, ciJob.PullRequestNumber)
		assert.Empty(t, ciJob.PullRequestAuthor)
	})

	t.Run("status mapping", func(t *testing.T) {
		statuses := []struct {
			input    string
			expected string
		}{
			{"Succeeded", "SUCCESS"},
			{"Failed", "FAILURE"},
			{"Cancelled", "ABORTED"},
			{"Running", "OTHER"},
			{"Pending", "OTHER"},
			{"Unknown", "OTHER"},
		}
		for _, tt := range statuses {
			t.Run(tt.input, func(t *testing.T) {
				pr := &TektonPipelineRun{
					PipelineRunName: "run-" + tt.input,
					Status:          tt.input,
					Scenario:        "test",
				}
				ciJob, err := convertTektonPipelineRunToCIJob(pr, 1, "s", "o", "r")
				assert.Nil(t, err)
				assert.Equal(t, tt.expected, ciJob.Result)
			})
		}
	})

	t.Run("uses fallback org/repo when git info empty", func(t *testing.T) {
		pr := &TektonPipelineRun{
			PipelineRunName: "run-1",
			Status:          "Succeeded",
			Scenario:        "test",
		}

		ciJob, err := convertTektonPipelineRunToCIJob(pr, 1, "scope", "fallback-org", "fallback-repo")
		assert.Nil(t, err)
		assert.Equal(t, "fallback-org", ciJob.Organization)
		assert.Equal(t, "fallback-repo", ciJob.Repository)
	})

	t.Run("unparseable duration leaves DurationSec nil", func(t *testing.T) {
		pr := &TektonPipelineRun{
			PipelineRunName: "run-1",
			Status:          "Succeeded",
			Scenario:        "test",
			Duration:        "not-a-duration",
		}
		ciJob, err := convertTektonPipelineRunToCIJob(pr, 1, "s", "o", "r")
		assert.Nil(t, err)
		assert.Nil(t, ciJob.DurationSec)
	})

	t.Run("empty duration", func(t *testing.T) {
		pr := &TektonPipelineRun{
			PipelineRunName: "run-1",
			Status:          "Succeeded",
			Scenario:        "test",
		}
		ciJob, err := convertTektonPipelineRunToCIJob(pr, 1, "s", "o", "r")
		assert.Nil(t, err)
		assert.Nil(t, ciJob.DurationSec)
	})
}

func TestValidateRequiredCIJobFields(t *testing.T) {
	t.Run("fully valid job returns no missing fields", func(t *testing.T) {
		prNum := 1
		started := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
		finished := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
		ciJob := &models.TestRegistryCIJob{
			ConnectionId:      1,
			JobId:             "job-1",
			JobName:           "e2e-test",
			JobType:           "tekton",
			Organization:      "org",
			Repository:        "repo",
			TriggerType:       "pull_request",
			Result:            "SUCCESS",
			ScopeId:           "scope-1",
			PullRequestNumber: &prNum,
			StartedAt:         &started,
			FinishedAt:        &finished,
			CommitSHA:         "abc123",
		}
		missing := validateRequiredCIJobFields(ciJob)
		assert.Empty(t, missing)
	})

	t.Run("empty job returns all missing fields", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		missing := validateRequiredCIJobFields(ciJob)
		assert.Contains(t, missing, "JobId")
		assert.Contains(t, missing, "JobName")
		assert.Contains(t, missing, "JobType")
		assert.Contains(t, missing, "Organization")
		assert.Contains(t, missing, "Repository")
		assert.Contains(t, missing, "TriggerType")
		assert.Contains(t, missing, "Result")
		assert.Contains(t, missing, "ScopeId")
		assert.Contains(t, missing, "FinishedAt")
		assert.Contains(t, missing, "StartedAt")
		assert.Contains(t, missing, "CommitSHA")
	})

	t.Run("pull_request without PR number reports it", func(t *testing.T) {
		started := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
		finished := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
		ciJob := &models.TestRegistryCIJob{
			ConnectionId: 1,
			JobId:        "job-1",
			JobName:      "test",
			JobType:      "prow",
			Organization: "org",
			Repository:   "repo",
			TriggerType:  "pull_request",
			Result:       "SUCCESS",
			ScopeId:      "s",
			StartedAt:    &started,
			FinishedAt:   &finished,
			CommitSHA:    "abc",
		}
		missing := validateRequiredCIJobFields(ciJob)
		found := false
		for _, m := range missing {
			if m == "PullRequestNumber (required for pull_request)" {
				found = true
			}
		}
		assert.True(t, found, "should report missing PullRequestNumber for pull_request trigger")
	})

	t.Run("push trigger without PR number is valid", func(t *testing.T) {
		started := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
		finished := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
		ciJob := &models.TestRegistryCIJob{
			ConnectionId: 1,
			JobId:        "job-1",
			JobName:      "test",
			JobType:      "prow",
			Organization: "org",
			Repository:   "repo",
			TriggerType:  "push",
			Result:       "SUCCESS",
			ScopeId:      "s",
			StartedAt:    &started,
			FinishedAt:   &finished,
			CommitSHA:    "abc",
		}
		missing := validateRequiredCIJobFields(ciJob)
		assert.Empty(t, missing)
	})

	t.Run("ConnectionId zero is reported", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{
			JobId: "j",
		}
		missing := validateRequiredCIJobFields(ciJob)
		assert.Contains(t, missing, "ConnectionId")
	})
}

func TestIsTektonJobAlreadyProcessed(t *testing.T) {
	t.Run("count > 0 returns true", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Count", mock.Anything).Return(int64(3), nil)
		assert.True(t, isTektonJobAlreadyProcessed(mockDal, 1, "job-1"))
	})

	t.Run("count = 0 returns false", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Count", mock.Anything).Return(int64(0), nil)
		assert.False(t, isTektonJobAlreadyProcessed(mockDal, 1, "job-1"))
	})

	t.Run("error returns false", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Count", mock.Anything).Return(int64(0), errors.Default.New("db error"))
		assert.False(t, isTektonJobAlreadyProcessed(mockDal, 1, "job-1"))
	})
}

func TestSaveRawTektonData(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("Create", mock.Anything, mock.Anything).Return(nil)

		pr := &TektonPipelineRun{
			PipelineRunName: "run-1",
			Status:          "Succeeded",
		}
		err := saveRawTektonData(mockDal, mockLogger, pr, `{"ConnectionId":1}`, "raw_table", "https://api.example.com")
		assert.Nil(t, err)
	})

	t.Run("error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockDal.On("Create", mock.Anything, mock.Anything).Return(errors.Default.New("db error"))

		pr := &TektonPipelineRun{PipelineRunName: "run-err"}
		err := saveRawTektonData(mockDal, mockLogger, pr, `{}`, "raw_table", "url")
		assert.NotNil(t, err)
	})
}
