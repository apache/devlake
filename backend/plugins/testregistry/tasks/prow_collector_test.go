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

func TestMatchesScope(t *testing.T) {
	t.Run("matches via labels", func(t *testing.T) {
		job := &ProwJob{
			Labels: map[string]string{
				"prow.k8s.io/refs.org":  "openshift",
				"prow.k8s.io/refs.repo": "console",
			},
			Status: ProwJobStatus{State: "success"},
		}
		assert.True(t, matchesScope(job, "openshift", "console"))
	})

	t.Run("matches via main refs", func(t *testing.T) {
		job := &ProwJob{
			Spec: ProwJobSpec{
				Refs: &ProwJobRefs{Org: "openshift", Repo: "console"},
			},
			Status: ProwJobStatus{State: "failure"},
		}
		assert.True(t, matchesScope(job, "openshift", "console"))
	})

	t.Run("matches via extra refs", func(t *testing.T) {
		job := &ProwJob{
			Spec: ProwJobSpec{
				ExtraRefs: []*ProwJobRefs{
					{Org: "openshift", Repo: "console"},
				},
			},
			Status: ProwJobStatus{State: "success"},
		}
		assert.True(t, matchesScope(job, "openshift", "console"))
	})

	t.Run("no match returns false", func(t *testing.T) {
		job := &ProwJob{
			Labels: map[string]string{
				"prow.k8s.io/refs.org":  "other-org",
				"prow.k8s.io/refs.repo": "other-repo",
			},
			Status: ProwJobStatus{State: "success"},
		}
		assert.False(t, matchesScope(job, "openshift", "console"))
	})

	t.Run("aborted state excluded", func(t *testing.T) {
		job := &ProwJob{
			Labels: map[string]string{
				"prow.k8s.io/refs.org":  "openshift",
				"prow.k8s.io/refs.repo": "console",
			},
			Status: ProwJobStatus{State: "aborted"},
		}
		assert.False(t, matchesScope(job, "openshift", "console"))
	})

	t.Run("pending state excluded", func(t *testing.T) {
		job := &ProwJob{
			Labels: map[string]string{
				"prow.k8s.io/refs.org":  "openshift",
				"prow.k8s.io/refs.repo": "console",
			},
			Status: ProwJobStatus{State: "pending"},
		}
		assert.False(t, matchesScope(job, "openshift", "console"))
	})

	t.Run("triggered state excluded", func(t *testing.T) {
		job := &ProwJob{
			Labels: map[string]string{
				"prow.k8s.io/refs.org":  "openshift",
				"prow.k8s.io/refs.repo": "console",
			},
			Status: ProwJobStatus{State: "triggered"},
		}
		assert.False(t, matchesScope(job, "openshift", "console"))
	})

	t.Run("nil labels falls back to refs", func(t *testing.T) {
		job := &ProwJob{
			Spec: ProwJobSpec{
				Refs: &ProwJobRefs{Org: "openshift", Repo: "console"},
			},
			Status: ProwJobStatus{State: "success"},
		}
		assert.True(t, matchesScope(job, "openshift", "console"))
	})
}

func TestIsValidJobState(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{"success", true},
		{"failure", true},
		{"error", true},
		{"aborted", false},
		{"pending", false},
		{"triggered", false},
		{"Aborted", false},
		{"PENDING", false},
		{"Triggered", false},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidJobState(tt.state))
		})
	}
}

func TestExtractJobID(t *testing.T) {
	t.Run("uses BuildID when available", func(t *testing.T) {
		job := &ProwJob{
			Status: ProwJobStatus{BuildID: "build-123", PodName: "pod-456"},
		}
		assert.Equal(t, "build-123", extractJobID(job))
	})

	t.Run("falls back to PodName", func(t *testing.T) {
		job := &ProwJob{
			Status: ProwJobStatus{PodName: "pod-456"},
		}
		assert.Equal(t, "pod-456", extractJobID(job))
	})

	t.Run("generates fallback ID from job name", func(t *testing.T) {
		job := &ProwJob{
			Spec: ProwJobSpec{Job: "e2e-test"},
		}
		id := extractJobID(job)
		assert.Contains(t, id, "e2e-test-")
		assert.NotEmpty(t, id)
	})
}

func TestExtractOrgRepo(t *testing.T) {
	t.Run("extracts from labels", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{
			Labels: map[string]string{
				"prow.k8s.io/refs.org":  "openshift",
				"prow.k8s.io/refs.repo": "console",
			},
		}
		extractOrgRepo(ciJob, prowJob)
		assert.Equal(t, "openshift", ciJob.Organization)
		assert.Equal(t, "console", ciJob.Repository)
	})

	t.Run("falls back to spec refs", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{
			Spec: ProwJobSpec{
				Refs: &ProwJobRefs{Org: "org-from-refs", Repo: "repo-from-refs"},
			},
		}
		extractOrgRepo(ciJob, prowJob)
		assert.Equal(t, "org-from-refs", ciJob.Organization)
		assert.Equal(t, "repo-from-refs", ciJob.Repository)
	})

	t.Run("labels take precedence over refs", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{
			Labels: map[string]string{
				"prow.k8s.io/refs.org":  "label-org",
				"prow.k8s.io/refs.repo": "label-repo",
			},
			Spec: ProwJobSpec{
				Refs: &ProwJobRefs{Org: "ref-org", Repo: "ref-repo"},
			},
		}
		extractOrgRepo(ciJob, prowJob)
		assert.Equal(t, "label-org", ciJob.Organization)
		assert.Equal(t, "label-repo", ciJob.Repository)
	})

	t.Run("keeps defaults when no labels or refs", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{
			Organization: "default-org",
			Repository:   "default-repo",
		}
		prowJob := &ProwJob{}
		extractOrgRepo(ciJob, prowJob)
		assert.Equal(t, "default-org", ciJob.Organization)
		assert.Equal(t, "default-repo", ciJob.Repository)
	})
}

func TestExtractGitInfo(t *testing.T) {
	t.Run("extracts PR SHA and info", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{
			Spec: ProwJobSpec{
				Refs: &ProwJobRefs{
					BaseSHA: "base-sha-abc",
					Pulls: []ProwJobPull{
						{Number: 42, SHA: "pr-sha-xyz", Author: "dev1"},
					},
				},
			},
		}
		extractGitInfo(ciJob, prowJob)
		assert.Equal(t, "pr-sha-xyz", ciJob.CommitSHA)
		assert.NotNil(t, ciJob.PullRequestNumber)
		assert.Equal(t, 42, *ciJob.PullRequestNumber)
		assert.Equal(t, "dev1", ciJob.PullRequestAuthor)
	})

	t.Run("uses base SHA when no pulls", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{
			Spec: ProwJobSpec{
				Refs: &ProwJobRefs{BaseSHA: "base-sha-abc"},
			},
		}
		extractGitInfo(ciJob, prowJob)
		assert.Equal(t, "base-sha-abc", ciJob.CommitSHA)
		assert.Nil(t, ciJob.PullRequestNumber)
	})

	t.Run("nil refs is safe", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{}
		extractGitInfo(ciJob, prowJob)
		assert.Empty(t, ciJob.CommitSHA)
		assert.Nil(t, ciJob.PullRequestNumber)
	})
}

func TestMapTriggerType(t *testing.T) {
	tests := []struct {
		name     string
		prowType string
		hasPR    bool
		want     string
	}{
		{"presubmit maps to pull_request", "presubmit", false, "pull_request"},
		{"postsubmit maps to push", "postsubmit", false, "push"},
		{"periodic maps to periodic", "periodic", false, "periodic"},
		{"unknown with PR infers pull_request", "", true, "pull_request"},
		{"unknown without PR defaults to push", "", false, "push"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciJob := &models.TestRegistryCIJob{}
			if tt.hasPR {
				prNum := 1
				ciJob.PullRequestNumber = &prNum
			}
			prowJob := &ProwJob{Spec: ProwJobSpec{Type: tt.prowType}}
			mapTriggerType(ciJob, prowJob)
			assert.Equal(t, tt.want, ciJob.TriggerType)
		})
	}
}

func TestMapJobStatus(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"success", "SUCCESS"},
		{"failure", "FAILURE"},
		{"error", "FAILURE"},
		{"aborted", "ABORTED"},
		{"Success", "SUCCESS"},
		{"unknown", "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			ciJob := &models.TestRegistryCIJob{}
			prowJob := &ProwJob{Status: ProwJobStatus{State: tt.state}}
			mapJobStatus(ciJob, prowJob)
			assert.Equal(t, tt.want, ciJob.Result)
		})
	}
}

func TestCalculateDurations(t *testing.T) {
	t.Run("calculates both durations", func(t *testing.T) {
		queued := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		started := time.Date(2024, 1, 1, 10, 1, 0, 0, time.UTC)
		finished := time.Date(2024, 1, 1, 10, 11, 0, 0, time.UTC)
		ciJob := &models.TestRegistryCIJob{
			QueuedAt:   &queued,
			StartedAt:  &started,
			FinishedAt: &finished,
		}
		calculateDurations(ciJob)
		assert.NotNil(t, ciJob.DurationSec)
		assert.InDelta(t, 600.0, *ciJob.DurationSec, 0.001)
		assert.NotNil(t, ciJob.QueuedDurationSec)
		assert.InDelta(t, 60.0, *ciJob.QueuedDurationSec, 0.001)
	})

	t.Run("nil timestamps produce nil durations", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		calculateDurations(ciJob)
		assert.Nil(t, ciJob.DurationSec)
		assert.Nil(t, ciJob.QueuedDurationSec)
	})

	t.Run("missing finish leaves duration nil", func(t *testing.T) {
		started := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		ciJob := &models.TestRegistryCIJob{StartedAt: &started}
		calculateDurations(ciJob)
		assert.Nil(t, ciJob.DurationSec)
	})
}

func TestIsTransientStatusCode(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{200, false},
		{404, false},
		{500, false},
		{429, true},
		{502, true},
		{503, true},
		{504, true},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.want, isTransientStatusCode(tt.code))
		})
	}
}

func TestConvertProwJobToCIJob(t *testing.T) {
	t.Run("full conversion", func(t *testing.T) {
		prowJob := &ProwJob{
			Labels: map[string]string{
				"prow.k8s.io/refs.org":  "openshift",
				"prow.k8s.io/refs.repo": "console",
			},
			Spec: ProwJobSpec{
				Job:       "e2e-test",
				Type:      "presubmit",
				Namespace: "ci",
				Refs: &ProwJobRefs{
					Org:  "openshift",
					Repo: "console",
					Pulls: []ProwJobPull{
						{Number: 99, SHA: "abc123", Author: "dev1"},
					},
				},
			},
			Status: ProwJobStatus{
				State:          "success",
				BuildID:        "build-42",
				StartTime:      "2024-06-15T10:00:00Z",
				CompletionTime: "2024-06-15T10:30:00Z",
				URL:            "https://prow.ci/view/123",
			},
		}

		ciJob, err := convertProwJobToCIJob(prowJob, 1, "openshift/console", "openshift", "console")
		assert.Nil(t, err)
		assert.Equal(t, "build-42", ciJob.JobId)
		assert.Equal(t, "e2e-test", ciJob.JobName)
		assert.Equal(t, "prow", ciJob.JobType)
		assert.Equal(t, "openshift", ciJob.Organization)
		assert.Equal(t, "console", ciJob.Repository)
		assert.Equal(t, "pull_request", ciJob.TriggerType)
		assert.Equal(t, "SUCCESS", ciJob.Result)
		assert.Equal(t, "ci", ciJob.Namespace)
		assert.Equal(t, "abc123", ciJob.CommitSHA)
		assert.NotNil(t, ciJob.PullRequestNumber)
		assert.Equal(t, 99, *ciJob.PullRequestNumber)
		assert.Equal(t, "dev1", ciJob.PullRequestAuthor)
		assert.Equal(t, "https://prow.ci/view/123", ciJob.ViewURL)
		assert.Equal(t, uint64(1), ciJob.ConnectionId)
		assert.Equal(t, "openshift/console", ciJob.ScopeId)
	})

	t.Run("postsubmit job", func(t *testing.T) {
		prowJob := &ProwJob{
			Spec: ProwJobSpec{
				Job:  "post-deploy",
				Type: "postsubmit",
				Refs: &ProwJobRefs{
					Org:     "org",
					Repo:    "repo",
					BaseSHA: "sha456",
				},
			},
			Status: ProwJobStatus{
				State:   "failure",
				BuildID: "build-99",
			},
		}

		ciJob, err := convertProwJobToCIJob(prowJob, 2, "scope", "org", "repo")
		assert.Nil(t, err)
		assert.Equal(t, "push", ciJob.TriggerType)
		assert.Equal(t, "FAILURE", ciJob.Result)
		assert.Equal(t, "sha456", ciJob.CommitSHA)
		assert.Nil(t, ciJob.PullRequestNumber)
	})
}

func TestExtractOrgRepoForGCS(t *testing.T) {
	t.Run("refs present", func(t *testing.T) {
		mockLogger := new(mocklog.Logger)
		job := &ProwJob{
			Spec: ProwJobSpec{
				Refs: &ProwJobRefs{Org: "test-org", Repo: "test-repo"},
			},
		}
		org, repo := extractOrgRepoForGCS(job, "fallback-org", "fallback-repo", "job-1", mockLogger)
		assert.Equal(t, "test-org", org)
		assert.Equal(t, "test-repo", repo)
	})

	t.Run("refs nil falls back", func(t *testing.T) {
		mockLogger := new(mocklog.Logger)
		mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()
		job := &ProwJob{}
		org, repo := extractOrgRepoForGCS(job, "fallback-org", "fallback-repo", "job-1", mockLogger)
		assert.Equal(t, "fallback-org", org)
		assert.Equal(t, "fallback-repo", repo)
	})
}

func TestSaveRawJobData(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Create", mock.Anything, mock.Anything).Return(nil)

		job := &ProwJob{
			Spec:   ProwJobSpec{Job: "e2e-test"},
			Status: ProwJobStatus{State: "success", BuildID: "b1"},
		}
		err := saveRawJobData(mockDal, "raw_table", `{"ConnectionId":1}`, "https://api.example.com", job)
		assert.Nil(t, err)
		mockDal.AssertCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Create", mock.Anything, mock.Anything).Return(errors.Default.New("db error"))

		job := &ProwJob{Spec: ProwJobSpec{Job: "test"}}
		err := saveRawJobData(mockDal, "raw_table", `{}`, "url", job)
		assert.NotNil(t, err)
	})
}

func TestParseTimestamps(t *testing.T) {
	t.Run("all timestamps set", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{
			Status: ProwJobStatus{
				PendingTime:    "2024-06-15T09:00:00Z",
				StartTime:      "2024-06-15T09:01:00Z",
				CompletionTime: "2024-06-15T09:30:00Z",
			},
		}
		parseTimestamps(ciJob, prowJob)
		assert.NotNil(t, ciJob.QueuedAt)
		assert.NotNil(t, ciJob.StartedAt)
		assert.NotNil(t, ciJob.FinishedAt)
		assert.Equal(t, 2024, ciJob.QueuedAt.Year())
	})

	t.Run("all timestamps empty", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{
			Status: ProwJobStatus{
				PendingTime:    "",
				StartTime:      "",
				CompletionTime: "",
			},
		}
		parseTimestamps(ciJob, prowJob)
		assert.Nil(t, ciJob.QueuedAt)
		assert.Nil(t, ciJob.StartedAt)
		assert.Nil(t, ciJob.FinishedAt)
	})

	t.Run("only PendingTime set", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{
			Status: ProwJobStatus{
				PendingTime:    "2024-06-15T09:00:00Z",
				StartTime:      "",
				CompletionTime: "",
			},
		}
		parseTimestamps(ciJob, prowJob)
		assert.NotNil(t, ciJob.QueuedAt)
		assert.Nil(t, ciJob.StartedAt)
		assert.Nil(t, ciJob.FinishedAt)
	})

	t.Run("only StartTime and CompletionTime set", func(t *testing.T) {
		ciJob := &models.TestRegistryCIJob{}
		prowJob := &ProwJob{
			Status: ProwJobStatus{
				PendingTime:    "",
				StartTime:      "2024-06-15T09:01:00Z",
				CompletionTime: "2024-06-15T09:30:00Z",
			},
		}
		parseTimestamps(ciJob, prowJob)
		assert.Nil(t, ciJob.QueuedAt)
		assert.NotNil(t, ciJob.StartedAt)
		assert.NotNil(t, ciJob.FinishedAt)
	})
}
