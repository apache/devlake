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
	"fmt"
	"testing"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/pkg/gcshelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// fakeHistoryStore implements gcshelper.HistoryStore for unit tests.
type fakeHistoryStore struct {
	// subdirs maps a prefix to its immediate child prefixes.
	subdirs map[string][]string
	// files maps a GCS object path to its content.
	files map[string][]byte
}

func (f *fakeHistoryStore) ListSubdirectories(_ context.Context, prefix string) ([]string, error) {
	return f.subdirs[prefix], nil
}

func (f *fakeHistoryStore) ReadFile(_ context.Context, path string) ([]byte, error) {
	data, ok := f.files[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return data, nil
}

// Compile-time check that fakeHistoryStore implements the interface.
var _ gcshelper.HistoryStore = (*fakeHistoryStore)(nil)

// TestFetchPRBuilds_HappyPath verifies that fetchPRBuilds correctly parses
// finished.json / started.json and returns one row per build.
func TestFetchPRBuilds_HappyPath(t *testing.T) {
	now := time.Now()
	startedTS := now.Add(-10 * time.Minute).Unix()
	finishedTS := now.Add(-5 * time.Minute).Unix()

	org, repo, prNum := "myorg", "myrepo", int64(42)
	jobName := "pull-ci-myorg-myrepo-main-unit-tests"
	buildID := "9999999"

	prPrefix := fmt.Sprintf("pr-logs/pull/%s_%s/%d/", org, repo, prNum)
	jobPrefix := prPrefix + jobName + "/"
	buildPrefix := jobPrefix + buildID + "/"

	store := &fakeHistoryStore{
		subdirs: map[string][]string{
			prPrefix:  {jobPrefix},
			jobPrefix: {buildPrefix},
		},
		files: map[string][]byte{
			buildPrefix + "finished.json": fmt.Appendf(nil,
				`{"timestamp":%d,"passed":true,"result":"SUCCESS","revision":"abc123"}`, finishedTS,
			),
			buildPrefix + "started.json": fmt.Appendf(nil,
				`{"timestamp":%d,"commit":"abc123"}`, startedTS,
			),
		},
	}

	pr := missingPR{
		PullRequestKey: "42",
		OrgName:        org,
		RepoShortName:  repo,
		RepoFullName:   org + "/" + repo,
	}
	cutoff := now.Add(-7 * 24 * time.Hour) // 7 days ago

	rows, err := fetchPRBuilds(context.Background(), store, pr, prNum, cutoff, &nopLogger{})
	if err != nil {
		t.Fatalf("fetchPRBuilds returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row.Result != "success" {
		t.Errorf("expected result=success, got %q", row.Result)
	}
	if row.JobName != jobName {
		t.Errorf("expected jobName=%q, got %q", jobName, row.JobName)
	}
	if *row.PullRequestNumber != prNum {
		t.Errorf("expected pullRequestNumber=%d, got %d", prNum, *row.PullRequestNumber)
	}
	if row.TriggerType != "pull_request" {
		t.Errorf("expected trigger_type=pull_request, got %q", row.TriggerType)
	}
	if row.ConnectionId != 0 {
		t.Errorf("expected connection_id=0 sentinel, got %d", row.ConnectionId)
	}
	if row.DurationSec == nil {
		t.Error("expected DurationSec to be populated")
	} else {
		wantDuration := float64(finishedTS-startedTS) - 1 // allow 1s tolerance
		if *row.DurationSec < wantDuration {
			t.Errorf("DurationSec=%f seems too small", *row.DurationSec)
		}
	}
}

// TestFetchPRBuilds_CutoffRespected verifies that builds older than the cutoff
// are not returned.
func TestFetchPRBuilds_CutoffRespected(t *testing.T) {
	now := time.Now()
	oldFinishedTS := now.Add(-200 * 24 * time.Hour).Unix() // 200 days ago

	org, repo, prNum := "myorg", "myrepo", int64(7)
	jobName := "pull-ci-myorg-myrepo-main-e2e"
	buildID := "1000000"

	prPrefix := fmt.Sprintf("pr-logs/pull/%s_%s/%d/", org, repo, prNum)
	jobPrefix := prPrefix + jobName + "/"
	buildPrefix := jobPrefix + buildID + "/"

	store := &fakeHistoryStore{
		subdirs: map[string][]string{
			prPrefix:  {jobPrefix},
			jobPrefix: {buildPrefix},
		},
		files: map[string][]byte{
			buildPrefix + "finished.json": fmt.Appendf(nil,
				`{"timestamp":%d,"passed":false,"result":"FAILURE"}`, oldFinishedTS,
			),
		},
	}

	pr := missingPR{
		PullRequestKey: "7",
		OrgName:        org,
		RepoShortName:  repo,
		RepoFullName:   org + "/" + repo,
	}
	cutoff := now.Add(-90 * 24 * time.Hour) // 90 days

	rows, err := fetchPRBuilds(context.Background(), store, pr, prNum, cutoff, &nopLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows (build is older than cutoff), got %d", len(rows))
	}
}

// TestFetchPRBuilds_FailedBuild verifies that failed builds are persisted with
// result=failure.
func TestFetchPRBuilds_FailedBuild(t *testing.T) {
	now := time.Now()
	finishedTS := now.Add(-2 * time.Hour).Unix()

	org, repo, prNum := "myorg", "myrepo", int64(99)
	jobName := "pull-ci-myorg-myrepo-main-integration"
	buildID := "8888888"

	prPrefix := fmt.Sprintf("pr-logs/pull/%s_%s/%d/", org, repo, prNum)
	jobPrefix := prPrefix + jobName + "/"
	buildPrefix := jobPrefix + buildID + "/"

	store := &fakeHistoryStore{
		subdirs: map[string][]string{
			prPrefix:  {jobPrefix},
			jobPrefix: {buildPrefix},
		},
		files: map[string][]byte{
			buildPrefix + "finished.json": fmt.Appendf(nil,
				`{"timestamp":%d,"passed":false,"result":"FAILURE"}`, finishedTS,
			),
		},
	}

	pr := missingPR{
		PullRequestKey: "99",
		OrgName:        org,
		RepoShortName:  repo,
		RepoFullName:   org + "/" + repo,
	}
	cutoff := now.Add(-30 * 24 * time.Hour)

	rows, err := fetchPRBuilds(context.Background(), store, pr, prNum, cutoff, &nopLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Result != "failure" {
		t.Errorf("expected result=failure, got %q", rows[0].Result)
	}
}

// TestFetchPRBuilds_NoPRDirectoryInGCS verifies graceful handling when a PR
// has no GCS directory (no builds were run).
func TestFetchPRBuilds_NoPRDirectoryInGCS(t *testing.T) {
	store := &fakeHistoryStore{
		subdirs: map[string][]string{},
		files:   map[string][]byte{},
	}

	pr := missingPR{
		PullRequestKey: "1",
		OrgName:        "myorg",
		RepoShortName:  "myrepo",
		RepoFullName:   "myorg/myrepo",
	}

	rows, err := fetchPRBuilds(context.Background(), store, pr, 1, time.Now().Add(-30*24*time.Hour), &nopLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

// TestFetchPRBuilds_MultipleJobs verifies that multiple job names under one PR
// each produce their own rows.
func TestFetchPRBuilds_MultipleJobs(t *testing.T) {
	now := time.Now()
	finishedTS := now.Add(-1 * time.Hour).Unix()

	org, repo, prNum := "myorg", "myrepo", int64(5)
	job1 := "pull-ci-myorg-myrepo-main-unit"
	job2 := "pull-ci-myorg-myrepo-main-e2e"
	buildID := "5555555"

	prPrefix := fmt.Sprintf("pr-logs/pull/%s_%s/%d/", org, repo, prNum)
	job1Prefix := prPrefix + job1 + "/"
	job2Prefix := prPrefix + job2 + "/"
	build1Prefix := job1Prefix + buildID + "/"
	build2Prefix := job2Prefix + buildID + "/"

	store := &fakeHistoryStore{
		subdirs: map[string][]string{
			prPrefix:    {job1Prefix, job2Prefix},
			job1Prefix:  {build1Prefix},
			job2Prefix:  {build2Prefix},
		},
		files: map[string][]byte{
			build1Prefix + "finished.json": fmt.Appendf(nil, `{"timestamp":%d,"passed":true,"result":"SUCCESS"}`, finishedTS),
			build2Prefix + "finished.json": fmt.Appendf(nil, `{"timestamp":%d,"passed":false,"result":"FAILURE"}`, finishedTS),
		},
	}

	pr := missingPR{
		PullRequestKey: "5",
		OrgName:        org,
		RepoShortName:  repo,
		RepoFullName:   org + "/" + repo,
	}

	rows, err := fetchPRBuilds(context.Background(), store, pr, prNum, now.Add(-30*24*time.Hour), &nopLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (one per job), got %d", len(rows))
	}

	results := map[string]string{}
	for _, r := range rows {
		results[r.JobName] = r.Result
	}
	if results[job1] != "success" {
		t.Errorf("job1 result: got %q, want success", results[job1])
	}
	if results[job2] != "failure" {
		t.Errorf("job2 result: got %q, want failure", results[job2])
	}
}

// nopLogger is a no-op log.Logger implementation for unit tests.
type nopLogger struct{}

func (*nopLogger) IsLevelEnabled(_ log.LogLevel) bool        { return false }
func (*nopLogger) Printf(_ string, _ ...any)                 {}
func (*nopLogger) Log(_ log.LogLevel, _ string, _ ...any)    {}
func (*nopLogger) Debug(_ string, _ ...any)                  {}
func (*nopLogger) Info(_ string, _ ...any)                   {}
func (*nopLogger) Warn(_ error, _ string, _ ...any)          {}
func (*nopLogger) Error(_ error, _ string, _ ...any)         {}
func (*nopLogger) Nested(_ string) log.Logger                { return &nopLogger{} }
func (*nopLogger) GetConfig() *log.LoggerConfig              { return &log.LoggerConfig{} }
func (*nopLogger) SetStream(_ *log.LoggerStreamConfig)       {}

func TestFindMissingPRs(t *testing.T) {
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("success with org/repo name", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		callCount := 0
		mockDal.On("All", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callCount++
			if callCount == 1 {
				// First call: repo name query — anonymous struct
				dst := args.Get(0).(*[]struct {
					Name string `gorm:"column:name"`
				})
				*dst = []struct {
					Name string `gorm:"column:name"`
				}{{Name: "org/my-repo"}}
			} else {
				// Second call: missing PRs query
				dst := args.Get(0).(*[]missingPRRow)
				*dst = []missingPRRow{
					{PullRequestKey: "42", RepoName: "org/my-repo"},
				}
			}
		})

		result, err := findMissingPRs(mockDal, "repo-1", cutoff)
		assert.Nil(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "42", result[0].PullRequestKey)
		assert.Equal(t, "org", result[0].OrgName)
		assert.Equal(t, "my-repo", result[0].RepoShortName)
		assert.Equal(t, "org/my-repo", result[0].RepoFullName)
	})

	t.Run("repo name without slash", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		callCount := 0
		mockDal.On("All", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callCount++
			if callCount == 1 {
				dst := args.Get(0).(*[]struct {
					Name string `gorm:"column:name"`
				})
				*dst = []struct {
					Name string `gorm:"column:name"`
				}{{Name: "simple-repo"}}
			} else {
				dst := args.Get(0).(*[]missingPRRow)
				*dst = []missingPRRow{
					{PullRequestKey: "10", RepoName: "simple-repo"},
				}
			}
		})

		result, err := findMissingPRs(mockDal, "repo-2", cutoff)
		assert.Nil(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "", result[0].OrgName)
		assert.Equal(t, "simple-repo", result[0].RepoShortName)
	})

	t.Run("repo name query error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Return(errors.Default.New("db error"))

		result, err := findMissingPRs(mockDal, "repo-1", cutoff)
		assert.Nil(t, result)
		assert.NotNil(t, err)
		assert.Contains(t, fmt.Sprint(err), "db error")
	})

	t.Run("repo not found", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			// Leave the destination slice empty (zero rows)
		})

		result, err := findMissingPRs(mockDal, "nonexistent-repo", cutoff)
		assert.Nil(t, result)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("missing PRs query error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		callCount := 0
		mockDal.On("All", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callCount++
			if callCount == 1 {
				dst := args.Get(0).(*[]struct {
					Name string `gorm:"column:name"`
				})
				*dst = []struct {
					Name string `gorm:"column:name"`
				}{{Name: "org/my-repo"}}
			}
		}).Once()
		mockDal.On("All", mock.Anything, mock.Anything).Return(errors.Default.New("query error")).Once()

		result, err := findMissingPRs(mockDal, "repo-1", cutoff)
		assert.Nil(t, result)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "query error")
	})
}
