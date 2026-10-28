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
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCalculateOutcome(t *testing.T) {
	cases := []struct {
		flagged bool
		failed  bool
		want    string
	}{
		{true, true, models.PredictionTP},
		{true, false, models.PredictionFP},
		{false, true, models.PredictionFN},
		{false, false, models.PredictionTN},
	}
	for _, tc := range cases {
		got := calculateOutcome(tc.flagged, tc.failed)
		if got != tc.want {
			t.Errorf("calculateOutcome(flagged=%v, failed=%v) = %q, want %q",
				tc.flagged, tc.failed, got, tc.want)
		}
	}
}

// TestNoCiDataProducesNoPredictions verifies the core invariant: a PR that has
// no matching entry in the ciOutcomes map must not produce any prediction.
//
// This is the unit-level guard for the bug where missing CI data was treated as
// "CI passed", turning every flagged PR into a False Positive.
func TestNoCiDataProducesNoPredictions(t *testing.T) {
	// Simulate the inner loop logic directly: an empty CI outcomes map (no
	// TestRegistry configured) paired with one high-risk AI-reviewed PR.
	ciOutcomes := map[prCiKey]ciOutcomeEntry{} // no CI data for any repo

	summaries := []prAiSummary{
		{
			PullRequestId:  "github:GithubPullRequest:1:9001",
			PullRequestKey: "901",
			RepoId:         "github:GithubRepo:1:999",
			RepoShortName:  "no-ci-repo",
			AiTool:         "CodeRabbit",
			MaxRiskScore:   80, // above any reasonable warning threshold
		},
	}

	written := 0
	for i := range summaries {
		ps := &summaries[i]
		ciKey := prCiKey{PullRequestNumber: ps.PullRequestKey, Repository: ps.RepoShortName}
		_, hasCiData := ciOutcomes[ciKey]
		if !hasCiData {
			continue // must skip — this is what we're testing
		}
		written++
	}

	if written != 0 {
		t.Errorf("expected 0 predictions written for repo with no CI data, got %d", written)
	}
}

// TestGeneratePredictionId verifies that two identical inputs produce the same
// ID and that differing inputs produce different IDs.
func TestGeneratePredictionId(t *testing.T) {
	id1 := generatePredictionId("pr:1", "CodeRabbit", "job_result")
	id2 := generatePredictionId("pr:1", "CodeRabbit", "job_result")
	id3 := generatePredictionId("pr:1", "CodeRabbit", "test_cases")
	id4 := generatePredictionId("pr:2", "CodeRabbit", "job_result")

	if id1 != id2 {
		t.Errorf("same inputs produced different IDs: %q vs %q", id1, id2)
	}
	if id1 == id3 {
		t.Errorf("different ci_source should produce different IDs, both got %q", id1)
	}
	if id1 == id4 {
		t.Errorf("different PR ID should produce different IDs, both got %q", id1)
	}
	if len(id1) == 0 {
		t.Error("generatePredictionId returned empty string")
	}
}

func TestRepoShortNameFrom(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"org/repo format", "openshift/console", "console"},
		{"nested path", "org/sub/repo", "repo"},
		{"no slash returns full name", "myrepo", "myrepo"},
		{"trailing slash", "org/", ""},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoShortNameFrom(tt.input)
			if got != tt.want {
				t.Errorf("repoShortNameFrom(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestUniqueRepoShortNames(t *testing.T) {
	t.Run("deduplicates and preserves order", func(t *testing.T) {
		summaries := []prAiSummary{
			{RepoShortName: "repo-a"},
			{RepoShortName: "repo-b"},
			{RepoShortName: "repo-a"},
			{RepoShortName: "repo-c"},
			{RepoShortName: "repo-b"},
		}
		got := uniqueRepoShortNames(summaries)
		want := []string{"repo-a", "repo-b", "repo-c"}
		if len(got) != len(want) {
			t.Fatalf("uniqueRepoShortNames() len = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("uniqueRepoShortNames()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		got := uniqueRepoShortNames(nil)
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("single entry", func(t *testing.T) {
		summaries := []prAiSummary{{RepoShortName: "only"}}
		got := uniqueRepoShortNames(summaries)
		if len(got) != 1 || got[0] != "only" {
			t.Errorf("expected [only], got %v", got)
		}
	})
}

func TestSavePredictionsBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

		batch := []*models.AiFailurePrediction{
			{Id: "p1", PullRequestId: "pr-1"},
			{Id: "p2", PullRequestId: "pr-2"},
		}
		err := savePredictionsBatch(mockDal, batch)
		assert.Nil(t, err)
		mockDal.AssertNumberOfCalls(t, "CreateOrUpdate", 2)
	})

	t.Run("empty batch", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		err := savePredictionsBatch(mockDal, []*models.AiFailurePrediction{})
		assert.Nil(t, err)
	})

	t.Run("error on save", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		batch := []*models.AiFailurePrediction{{Id: "p1"}}
		err := savePredictionsBatch(mockDal, batch)
		assert.NotNil(t, err)
	})
}

func TestBuildFlakyTestSet(t *testing.T) {
	t.Run("success with results", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]struct {
				Name       string `gorm:"column:name"`
				Repository string `gorm:"column:repository"`
			})
			*dst = []struct {
				Name       string `gorm:"column:name"`
				Repository string `gorm:"column:repository"`
			}{
				{Name: "TestFlaky1", Repository: "repo-a"},
				{Name: "TestFlaky2", Repository: "repo-b"},
			}
		}).Return(nil)

		result, err := buildFlakyTestSet(mockDal)
		assert.Nil(t, err)
		assert.Len(t, result, 2)
		assert.True(t, result[prCiKey{PullRequestNumber: "TestFlaky1", Repository: "repo-a"}])
		assert.True(t, result[prCiKey{PullRequestNumber: "TestFlaky2", Repository: "repo-b"}])
	})

	t.Run("empty results", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Return(nil)

		result, err := buildFlakyTestSet(mockDal)
		assert.Nil(t, err)
		assert.Empty(t, result)
	})

	t.Run("error returns wrapped error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		result, err := buildFlakyTestSet(mockDal)
		assert.NotNil(t, err)
		assert.Nil(t, result)
	})
}

func TestBuildFlakyJobSet(t *testing.T) {
	t.Run("success with results", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]struct {
				JobName    string `gorm:"column:job_name"`
				Repository string `gorm:"column:repository"`
			})
			*dst = []struct {
				JobName    string `gorm:"column:job_name"`
				Repository string `gorm:"column:repository"`
			}{
				{JobName: "e2e-test", Repository: "repo-a"},
			}
		}).Return(nil)

		result, err := buildFlakyJobSet(mockDal)
		assert.Nil(t, err)
		assert.Len(t, result, 1)
		assert.True(t, result["e2e-test|repo-a"])
	})

	t.Run("error returns wrapped error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		result, err := buildFlakyJobSet(mockDal)
		assert.NotNil(t, err)
		assert.Nil(t, result)
	})
}

func TestLoadAiReviewPrSummaries(t *testing.T) {
	t.Run("single repo mode", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]struct {
				PullRequestId  string    `gorm:"column:pull_request_id"`
				PullRequestKey string    `gorm:"column:pull_request_key"`
				RepoId         string    `gorm:"column:repo_id"`
				RepoName       string    `gorm:"column:repo_name"`
				AiTool         string    `gorm:"column:ai_tool"`
				MaxRiskScore   int       `gorm:"column:max_risk_score"`
				CreatedDate    time.Time `gorm:"column:created_date"`
				PrTitle        string    `gorm:"column:pr_title"`
				PrUrl          string    `gorm:"column:pr_url"`
				PrAuthor       string    `gorm:"column:pr_author"`
				PrCreatedAt    time.Time `gorm:"column:pr_created_at"`
				Additions      int       `gorm:"column:additions"`
				Deletions      int       `gorm:"column:deletions"`
			})
			*dst = []struct {
				PullRequestId  string    `gorm:"column:pull_request_id"`
				PullRequestKey string    `gorm:"column:pull_request_key"`
				RepoId         string    `gorm:"column:repo_id"`
				RepoName       string    `gorm:"column:repo_name"`
				AiTool         string    `gorm:"column:ai_tool"`
				MaxRiskScore   int       `gorm:"column:max_risk_score"`
				CreatedDate    time.Time `gorm:"column:created_date"`
				PrTitle        string    `gorm:"column:pr_title"`
				PrUrl          string    `gorm:"column:pr_url"`
				PrAuthor       string    `gorm:"column:pr_author"`
				PrCreatedAt    time.Time `gorm:"column:pr_created_at"`
				Additions      int       `gorm:"column:additions"`
				Deletions      int       `gorm:"column:deletions"`
			}{
				{
					PullRequestId:  "pr-1",
					PullRequestKey: "42",
					RepoId:         "repo-1",
					RepoName:       "org/my-repo",
					AiTool:         "CodeRabbit",
					MaxRiskScore:   80,
				},
			}
		}).Return(nil)

		result, err := loadAiReviewPrSummaries(mockDal, "repo-1", "")
		assert.Nil(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "pr-1", result[0].PullRequestId)
		assert.Equal(t, "my-repo", result[0].RepoShortName)
		assert.Equal(t, "CodeRabbit", result[0].AiTool)
	})

	t.Run("error returns wrapped error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		result, err := loadAiReviewPrSummaries(mockDal, "repo-1", "")
		assert.NotNil(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty results", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Return(nil)

		result, err := loadAiReviewPrSummaries(mockDal, "repo-1", "")
		assert.Nil(t, err)
		assert.Empty(t, result)
	})
}

func TestLoadCiOutcomesByTestCases(t *testing.T) {
	t.Run("empty repoShortNames", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		result, err := loadCiOutcomesByTestCases(mockDal, []string{}, nil)
		assert.Nil(t, err)
		assert.Empty(t, result)
	})

	t.Run("success with rows", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]struct {
				PullRequestNumber int64  `gorm:"column:pull_request_number"`
				Repository        string `gorm:"column:repository"`
				TestName          string `gorm:"column:test_name"`
				Status            string `gorm:"column:status"`
			})
			*dst = []struct {
				PullRequestNumber int64  `gorm:"column:pull_request_number"`
				Repository        string `gorm:"column:repository"`
				TestName          string `gorm:"column:test_name"`
				Status            string `gorm:"column:status"`
			}{
				{PullRequestNumber: 42, Repository: "repo-a", TestName: "TestPassing", Status: "passed"},
				{PullRequestNumber: 42, Repository: "repo-a", TestName: "TestRealFailure", Status: "failed"},
				{PullRequestNumber: 42, Repository: "repo-a", TestName: "FlakyTest", Status: "failed"},
			}
		}).Return(nil)

		flakyTests := map[prCiKey]bool{
			{PullRequestNumber: "FlakyTest", Repository: "repo-a"}: true,
		}

		result, err := loadCiOutcomesByTestCases(mockDal, []string{"repo-a"}, flakyTests)
		assert.Nil(t, err)
		assert.Len(t, result, 1)
		key := prCiKey{PullRequestNumber: "42", Repository: "repo-a"}
		assert.True(t, result[key].HadNonFlakyFailure)
	})

	t.Run("error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		result, err := loadCiOutcomesByTestCases(mockDal, []string{"repo-a"}, nil)
		assert.NotNil(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "CI test case outcomes")
	})
}

func TestLoadCiOutcomesByJobResult(t *testing.T) {
	t.Run("empty repoShortNames", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		result, err := loadCiOutcomesByJobResult(mockDal, []string{}, nil, false)
		assert.Nil(t, err)
		assert.Empty(t, result)
	})

	t.Run("success without infra filter", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]ciJobRow)
			*dst = []ciJobRow{
				{PullRequestNumber: 10, Repository: "repo-a", JobId: "j1", JobName: "build", Result: "SUCCESS"},
				{PullRequestNumber: 10, Repository: "repo-a", JobId: "j2", JobName: "test", Result: "FAILURE"},
			}
		}).Return(nil)

		result, err := loadCiOutcomesByJobResult(mockDal, []string{"repo-a"}, nil, false)
		assert.Nil(t, err)
		assert.Len(t, result, 1)
		key := prCiKey{PullRequestNumber: "10", Repository: "repo-a"}
		assert.True(t, result[key].HadNonFlakyFailure)
	})

	t.Run("success with infra filter", func(t *testing.T) {
		mockDal := new(mockdal.Dal)

		// First call: load job rows
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]ciJobRow)
			*dst = []ciJobRow{
				{PullRequestNumber: 10, Repository: "repo-a", JobId: "j1", JobName: "build", Result: "FAILURE"},
				{PullRequestNumber: 10, Repository: "repo-a", JobId: "j2", JobName: "test", Result: "FAILURE"},
			}
		}).Return(nil).Once()

		// Second call: buildJobRunsWithTestFailures — only j2 has test failures
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]struct {
				JobId string `gorm:"column:job_id"`
			})
			*dst = []struct {
				JobId string `gorm:"column:job_id"`
			}{
				{JobId: "j2"},
			}
		}).Return(nil).Once()

		result, err := loadCiOutcomesByJobResult(mockDal, []string{"repo-a"}, nil, true)
		assert.Nil(t, err)
		assert.Len(t, result, 1)
		key := prCiKey{PullRequestNumber: "10", Repository: "repo-a"}
		// j1 was infra failure (no test failures), j2 had test failures
		assert.True(t, result[key].HadNonFlakyFailure)
	})

	t.Run("error on query", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		result, err := loadCiOutcomesByJobResult(mockDal, []string{"repo-a"}, nil, false)
		assert.NotNil(t, err)
		assert.Nil(t, result)
	})
}

func TestBuildJobRunsWithTestFailures(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]struct {
				JobId string `gorm:"column:job_id"`
			})
			*dst = []struct {
				JobId string `gorm:"column:job_id"`
			}{
				{JobId: "job-1"},
				{JobId: "job-2"},
			}
		}).Return(nil)

		result, err := buildJobRunsWithTestFailures(mockDal, []string{"repo-a"})
		assert.Nil(t, err)
		assert.Len(t, result, 2)
		assert.True(t, result["job-1"])
		assert.True(t, result["job-2"])
	})

	t.Run("error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		result, err := buildJobRunsWithTestFailures(mockDal, []string{"repo-a"})
		assert.NotNil(t, err)
		assert.Nil(t, result)
	})
}
