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
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/unithelper"
	mockdal "github.com/apache/incubator-devlake/mocks/core/dal"
	mockapi "github.com/apache/incubator-devlake/mocks/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCicdRunLoadLatestRunUpdatedAt_ReturnsLatestTimestamp(t *testing.T) {
	connectionId := uint64(1)
	repoId := 101
	latestTs := time.Date(2025, 4, 1, 10, 11, 12, 0, time.UTC)

	mockDal := new(mockdal.Dal)
	mockDal.On("First", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(0).(*models.GithubRun)
		dst.ConnectionId = connectionId
		dst.RepoId = repoId
		dst.GithubUpdatedAt = &latestTs
	}).Return(nil).Once()

	ctx := unithelper.DummySubTaskContext(mockDal)
	since, err := loadLatestRunUpdatedAt(ctx, connectionId, repoId)

	require.Nil(t, err)
	require.NotNil(t, since)
	assert.True(t, since.Equal(latestTs))
	mockDal.AssertExpectations(t)
}

func TestCicdRunLoadLatestRunUpdatedAt_NotFoundReturnsNil(t *testing.T) {
	mockDal := new(mockdal.Dal)
	notFoundErr := errors.Default.New("record not found")
	mockDal.On("First", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(notFoundErr).Once()
	mockDal.On("IsErrorNotFound", notFoundErr).Return(true).Once()

	ctx := unithelper.DummySubTaskContext(mockDal)
	since, err := loadLatestRunUpdatedAt(ctx, 1, 101)

	require.Nil(t, err)
	assert.Nil(t, since)
	mockDal.AssertExpectations(t)
}

func TestCicdRunLoadLatestRunUpdatedAt_PropagatesError(t *testing.T) {
	mockDal := new(mockdal.Dal)
	dbErr := errors.Default.New("db unavailable")
	mockDal.On("First", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(dbErr).Once()
	mockDal.On("IsErrorNotFound", dbErr).Return(false).Once()

	ctx := unithelper.DummySubTaskContext(mockDal)
	since, err := loadLatestRunUpdatedAt(ctx, 1, 101)

	assert.Nil(t, since)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "db unavailable")
	mockDal.AssertExpectations(t)
}

// newTestBuilder constructs a leafWindowBuilder with a stubbed probe for unit testing.
func newTestBuilder(probe probeFunc) *leafWindowBuilder {
	mockDal := new(mockdal.Dal)
	return &leafWindowBuilder{
		taskCtx: unithelper.DummySubTaskContext(mockDal),
		data:    &GithubTaskData{Options: &GithubOptions{Name: "o/r"}},
		probe:   probe,
	}
}

// ------------------------------------------------------------------------------------------
// leafWindowBuilder.build unit tests
// ------------------------------------------------------------------------------------------

func TestCicdRunBuildLeafWindows_SingleWindowUnderCap(t *testing.T) {
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, _, _ time.Time) (int, bool, errors.Error) {
		return 500, false, nil
	})

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	require.Len(t, leaves, 1)
	assert.True(t, leaves[0].From.Equal(from))
	assert.True(t, leaves[0].To.Equal(to))
}

func TestCicdRunBuildLeafWindows_EmptyWindow(t *testing.T) {
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, _, _ time.Time) (int, bool, errors.Error) {
		return 0, false, nil
	})

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	assert.Empty(t, leaves)
}

// TestCicdRunBuildLeafWindows_InvertedWindowReturnsNoLeaves pins the entry-guard contract:
// when `from > to` (rapid re-sync edge case), build must short-circuit to (nil, nil)
// without probing so no GitHub round-trip is paid on a no-op sync.
func TestCicdRunBuildLeafWindows_InvertedWindowReturnsNoLeaves(t *testing.T) {
	var probeCalls int32
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, _, _ time.Time) (int, bool, errors.Error) {
		atomic.AddInt32(&probeCalls, 1)
		return 0, false, nil
	})

	to := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	from := to.Add(2 * time.Second)

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	assert.Nil(t, leaves)
	assert.Equal(t, int32(0), atomic.LoadInt32(&probeCalls))
}

func TestCicdRunBuildLeafWindows_BisectsOnCap(t *testing.T) {
	var callCount int32
	fullWindow := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, f, to time.Time) (int, bool, errors.Error) {
		atomic.AddInt32(&callCount, 1)
		if to.Sub(f) >= fullWindow {
			return 1000, false, nil
		}
		return 500, false, nil
	})

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	require.Len(t, leaves, 2)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&callCount), int32(3))
	assert.True(t, leaves[0].From.Equal(from))
	assert.True(t, leaves[1].To.Equal(to))
}

func TestCicdRunBuildLeafWindows_BisectsOn422(t *testing.T) {
	fullWindow := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, f, to time.Time) (int, bool, errors.Error) {
		if to.Sub(f) >= fullWindow {
			return 0, true, nil
		}
		return 100, false, nil
	})

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	require.Len(t, leaves, 2)
}

func TestCicdRunBuildLeafWindows_MinWindowReturnsError(t *testing.T) {
	// Only a single-second bucket (from.Unix() == to.Unix()) is truly unbisectable; a 1s-wide
	// window bisects into [T, T] and [T+1, T+1] and the recursive call on the inner bucket
	// surfaces the error.
	t.Run("zero-width saturated window -> error", func(t *testing.T) {
		b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, _, _ time.Time) (int, bool, errors.Error) {
			return 1000, false, nil
		})

		from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		to := from

		leaves, err := b.build(from, to)
		assert.Nil(t, leaves)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "single 1-second bucket")
		assert.Contains(t, err.Error(), "Refusing to advance collector state")
	})

	t.Run("1s-wide saturated window bisects then fails on inner single-second bucket", func(t *testing.T) {
		b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, _, _ time.Time) (int, bool, errors.Error) {
			return 1000, false, nil
		})

		from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		to := from.Add(time.Second)

		leaves, err := b.build(from, to)
		assert.Nil(t, leaves)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "single 1-second bucket")
	})
}

func TestCicdRunBuildLeafWindows_MinWindowOn422ReturnsError(t *testing.T) {
	t.Run("zero-width 422 window -> error", func(t *testing.T) {
		b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, _, _ time.Time) (int, bool, errors.Error) {
			return 0, true, nil
		})

		from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		to := from

		leaves, err := b.build(from, to)
		assert.Nil(t, leaves)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "single 1-second bucket")
	})

	t.Run("1s-wide 422 window bisects then fails on inner single-second bucket", func(t *testing.T) {
		b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, _, _ time.Time) (int, bool, errors.Error) {
			return 0, true, nil
		})

		from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		to := from.Add(time.Second)

		leaves, err := b.build(from, to)
		assert.Nil(t, leaves)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "single 1-second bucket")
	})
}

func TestCicdRunBuildLeafWindows_NoRightHalfEmptyAtSubSecondWidths(t *testing.T) {
	// Regression guard: integer-second bisection keeps both halves inside [from, to] at
	// sub-second-tail widths; the old duration-based bisector failed with "right half empty".
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(1500 * time.Millisecond)
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, f, tt time.Time) (int, bool, errors.Error) {
		if f.Equal(from) && tt.Equal(to) {
			return 1000, false, nil
		}
		return 500, false, nil
	})

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	require.Len(t, leaves, 2)
	assert.Equal(t, from.Unix(), leaves[0].From.Unix())
	assert.Equal(t, from.Unix(), leaves[0].To.Unix())
	assert.Equal(t, from.Unix()+1, leaves[1].From.Unix())
	assert.True(t, leaves[1].To.Equal(to))
}

// TestCicdRunBuildLeafWindows_ProbeErrIgnoreAndContinueRecovery pins the contract that
// `build` treats (total=0, is422=true, err=nil) as a valid saturated signal, which is what
// defaultProbeTotalCount emits when a shared ApiAsyncClient's AfterResponse hook converts
// 422 into helper.ErrIgnoreAndContinue.
func TestCicdRunBuildLeafWindows_ProbeErrIgnoreAndContinueRecovery(t *testing.T) {
	fullWindow := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, f, to time.Time) (int, bool, errors.Error) {
		if to.Sub(f) >= fullWindow {
			return 0, true, nil
		}
		return 200, false, nil
	})

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	require.Len(t, leaves, 2)
	assert.True(t, leaves[0].From.Equal(from))
	assert.True(t, leaves[1].To.Equal(to))
	assert.True(t, leaves[1].From.Equal(leaves[0].To.Add(time.Second)))
}

func TestCicdRunBuildLeafWindows_ProbeErrorPropagates(t *testing.T) {
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, _, _ time.Time) (int, bool, errors.Error) {
		return 0, false, errors.Default.New("boom")
	})

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	leaves, err := b.build(from, to)
	assert.Nil(t, leaves)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestCicdRunBuildLeafWindows_BoundaryCases(t *testing.T) {
	// Boundary total_count values at zero-width (unbisectable single-second bucket) and 1s
	// (bisectable) widths.
	cases := []struct {
		total       int
		is422       bool
		expectLeaf  bool
		expectError bool
		description string
	}{
		{0, false, false, false, "zero total -> no leaf"},
		{1, false, true, false, "1 total -> one leaf"},
		{999, false, true, false, "999 total -> one leaf"},
		{1000, false, false, true, "1000 -> error (saturated at bucket)"},
		{1001, false, false, true, "1001 -> error (saturated at bucket)"},
		{0, true, false, true, "422 -> error (saturated at bucket)"},
	}
	widths := []struct {
		name     string
		duration time.Duration
	}{
		{"zero-width (single-second bucket)", 0},
		{"1s-wide (bisectable at second precision)", time.Second},
	}
	for _, w := range widths {
		for _, c := range cases {
			t.Run(w.name+"/"+c.description, func(t *testing.T) {
				total := c.total
				is422 := c.is422
				b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, _, _ time.Time) (int, bool, errors.Error) {
					return total, is422, nil
				})

				from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				to := from.Add(w.duration)

				leaves, err := b.build(from, to)
				if c.expectError {
					require.NotNil(t, err)
					assert.Nil(t, leaves)
					assert.Contains(t, err.Error(), "single 1-second bucket")
					return
				}
				require.Nil(t, err)
				if c.expectLeaf {
					assert.Len(t, leaves, 1)
				} else {
					assert.Empty(t, leaves)
				}
			})
		}
	}
}

func TestCicdRunBuildLeafWindows_IntegerSecondSplit(t *testing.T) {
	// Parent [T, T+1s] with total=1000 must split into two single-second-bucket leaves
	// [T, T] and [T+1, T+1], each with total < FILTERED_SEARCH_CAP.
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Second)
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, f, tt time.Time) (int, bool, errors.Error) {
		if f.Equal(from) && tt.Equal(to) {
			return 1000, false, nil
		}
		return 500, false, nil
	})

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	require.Len(t, leaves, 2)

	assert.Equal(t, from.Unix(), leaves[0].From.Unix())
	assert.Equal(t, from.Unix(), leaves[0].To.Unix())
	assert.Equal(t, from.Unix()+1, leaves[1].From.Unix())
	assert.Equal(t, from.Unix()+1, leaves[1].To.Unix())
	assert.True(t, leaves[1].From.Equal(leaves[0].To.Add(time.Second)))
}

func TestCicdRunBuildLeafWindows_BootstrapFromEpoch(t *testing.T) {
	// Simulate createdAfter == nil path: caller passes 2018-01-01 as start.
	full := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC))
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, f, to time.Time) (int, bool, errors.Error) {
		if to.Sub(f) >= full/2 {
			return 1000, false, nil
		}
		return 500, false, nil
	})

	from := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	assert.GreaterOrEqual(t, len(leaves), 2)
}

// ------------------------------------------------------------------------------------------
// Boundary (non-overlap) test
// ------------------------------------------------------------------------------------------

func TestCicdRunBuildLeafWindows_BoundaryNonOverlapping(t *testing.T) {
	fullWindow := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	b := newTestBuilder(func(_ plugin.SubTaskContext, _ *GithubTaskData, f, to time.Time) (int, bool, errors.Error) {
		if to.Sub(f) >= fullWindow {
			return 1000, false, nil
		}
		return 500, false, nil
	})

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	leaves, err := b.build(from, to)
	require.Nil(t, err)
	require.Len(t, leaves, 2)

	assert.True(t, leaves[1].From.Equal(leaves[0].To.Add(time.Second)),
		"right.From (%s) must equal left.To+1s (%s)",
		leaves[1].From.UTC().Format(time.RFC3339),
		leaves[0].To.Add(time.Second).UTC().Format(time.RFC3339))

	leftStr := leaves[0].To.UTC().Format(githubTimeLayout)
	rightStr := leaves[1].From.UTC().Format(githubTimeLayout)
	assert.NotEqual(t, leftStr, rightStr, "boundary timestamps must differ by at least 1s")
}

// ------------------------------------------------------------------------------------------
// Query hook window application
// ------------------------------------------------------------------------------------------

func TestCicdRunQueryHookAppliesWindow(t *testing.T) {
	from := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	to := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)
	reqData := &helper.RequestData{
		Pager: &helper.Pager{Page: 2, Size: 30},
		Input: &TimeWindow{From: from, To: to},
	}

	q, err := buildRunsQuery(reqData)
	require.Nil(t, err)
	assert.Equal(t, "2024-01-02T03:04:05Z..2024-06-07T08:09:10Z", q.Get("created"))
	assert.Equal(t, "2", q.Get("page"))
	assert.Equal(t, "30", q.Get("per_page"))
	assert.True(t, strings.Contains(q.Get("created"), ".."))
}

// ------------------------------------------------------------------------------------------
// Thin integration test
// Verifies:
//   - Delete (raw table truncation) is called at most once for a full sync
//   - DoGetAsync is called at least once per leaf window (concurrency=1, PageSize=3)
// ------------------------------------------------------------------------------------------

func TestCicdRunRegisterCollectorForLeafWindows_SingleDelete(t *testing.T) {
	mockDal := new(mockdal.Dal)
	notFoundErr := errors.Default.New("record not found")
	mockDal.On("First", mock.Anything, mock.Anything).Return(notFoundErr).Once()
	mockDal.On("IsErrorNotFound", mock.Anything).Return(true).Maybe()
	mockDal.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Return(nil).Once()
	mockDal.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	mockDal.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()

	mockCtx := unithelper.DummySubTaskContext(mockDal)

	var getAsyncCount int32
	mockApi := new(mockapi.RateLimitedApiClient)
	mockApi.On("DoGetAsync", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		atomic.AddInt32(&getAsyncCount, 1)
		res := &http.Response{
			Request: &http.Request{URL: &url.URL{}},
			Body:    io.NopCloser(bytes.NewBufferString(`{"total_count":0,"workflow_runs":[]}`)),
		}
		handler := args.Get(3).(plugin.ApiAsyncCallback)
		_ = handler(res)
	})
	mockApi.On("NextTick", mock.Anything).Run(func(args mock.Arguments) {
		fn := args.Get(0).(func() errors.Error)
		assert.Nil(t, fn())
	})
	mockApi.On("HasError").Return(false)
	mockApi.On("WaitAsync").Return(nil)
	mockApi.On("GetAfterFunction").Return(nil)
	mockApi.On("SetAfterFunction", mock.Anything).Return()

	manager, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx: mockCtx,
		Params: GithubApiParams{
			ConnectionId: 1,
			Name:         "apache/incubator-devlake",
		},
		Table: RAW_RUN_TABLE,
	})
	require.Nil(t, err)

	leaves := []TimeWindow{
		{From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{From: time.Date(2024, 1, 15, 0, 0, 1, 0, time.UTC), To: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)},
		{From: time.Date(2024, 2, 1, 0, 0, 1, 0, time.UTC), To: time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC)},
	}

	// Force concurrency=1 and PageSize=3 for deterministic DoGetAsync counting; share the
	// production Query hook via buildRunsQuery so changes to it stay covered.
	iterator := helper.NewQueueIterator()
	for i := range leaves {
		w := leaves[i]
		iterator.Push(&w)
	}
	err = manager.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   mockApi,
		Input:       iterator,
		UrlTemplate: "repos/{{ .Params.Name }}/actions/runs",
		Query:       buildRunsQuery,
		PageSize:    3,
		Concurrency: 1,
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			return nil, nil
		},
	})
	require.Nil(t, err)

	require.Nil(t, manager.Execute())

	mockDal.AssertExpectations(t)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&getAsyncCount), int32(len(leaves)))
}
