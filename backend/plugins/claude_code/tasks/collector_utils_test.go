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

	"github.com/stretchr/testify/assert"
)

func TestComputeUsageDateRangeSkipsRecentDays(t *testing.T) {
	now := time.Date(2026, 6, 19, 14, 30, 0, 0, time.UTC)

	start, until := computeUsageDateRange(now, nil)

	assert.Equal(t, time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC), until)
}

func TestComputeUsageDateRangeHonorsApiFloor(t *testing.T) {
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)

	start, until := computeUsageDateRange(now, nil)

	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), until)
}

func TestComputeUsageDateRangeResumesFromLastCollectedDay(t *testing.T) {
	now := time.Date(2026, 6, 19, 14, 30, 0, 0, time.UTC)
	since := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	start, until := computeUsageDateRange(now, &since)

	assert.Equal(t, time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC), until)
}

func TestComputeUsageDateRangeCollectsWhenSinceIsNow(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 31, 0, 0, time.UTC)

	start, until := computeUsageDateRange(now, &now)

	assert.Equal(t, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), until)
	assert.True(t, newClaudeCodeDayIterator(start, until).HasNext())
	assert.True(t, newClaudeCodeDateRangeIterator(start, until, claudeCodeSummaryMaxDays).HasNext())
}

func TestComputeUsageDateRangeCapsBackfillHorizon(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 31, 0, 0, time.UTC)
	since := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	start, until := computeUsageDateRange(now, &since)

	assert.Equal(t, time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), until)
}

func TestComputeUsageDateRangeCollectsEveryDayAcrossRuns(t *testing.T) {
	first := time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC)
	for _, interval := range []int{1, 2, 3, 7} {
		collected := map[time.Time]bool{}
		var since *time.Time
		var now time.Time
		for run := 0; run < 6; run++ {
			now = first.AddDate(0, 0, run*interval)
			start, until := computeUsageDateRange(now, since)
			for d := start; !d.After(until); d = d.AddDate(0, 0, 1) {
				collected[d] = true
			}
			runStart := now
			since = &runStart
		}

		start, _ := computeUsageDateRange(first, nil)
		_, until := computeUsageDateRange(now, nil)
		for d := start; !d.After(until); d = d.AddDate(0, 0, 1) {
			assert.Truef(t, collected[d], "interval %d: %s not collected", interval, d.Format(claudeCodeDateLayout))
		}
	}
}
