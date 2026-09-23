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

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

func TestBuildWorklogQuery(t *testing.T) {
	pager := &api.Pager{Page: 3, Skip: 2000, Size: 1000}
	timeAfter := time.Date(2026, 1, 28, 0, 0, 0, 0, time.UTC)
	lastSuccess := time.Date(2026, 9, 22, 0, 0, 13, 0, time.UTC)

	t.Run("pages with offset and limit", func(t *testing.T) {
		q := buildWorklogQuery(&TempoOptions{TeamId: 4}, pager, false, nil)
		assert.Equal(t, "2000", q.Get("offset"))
		assert.Equal(t, "1000", q.Get("limit"))
	})

	t.Run("full sync starts at timeAfter, without a rolling window", func(t *testing.T) {
		q := buildWorklogQuery(&TempoOptions{TeamId: 4}, pager, false, &timeAfter)
		assert.Equal(t, "2026-01-28", q.Get("from"))
		assert.Empty(t, q.Get("to"))
		assert.Empty(t, q.Get("updatedFrom"))
	})

	t.Run("incremental sync fetches what changed since the last success", func(t *testing.T) {
		q := buildWorklogQuery(&TempoOptions{TeamId: 4}, pager, true, &lastSuccess)
		assert.Equal(t, "2026-09-22T00:00:13Z", q.Get("updatedFrom"))
		assert.Empty(t, q.Get("from"))
	})

	t.Run("global endpoint gets the same bounds", func(t *testing.T) {
		q := buildWorklogQuery(&TempoOptions{}, pager, false, &timeAfter)
		assert.Equal(t, "2026-01-28", q.Get("from"))
	})

	t.Run("explicit fromDate/toDate options win", func(t *testing.T) {
		opts := &TempoOptions{TeamId: 4, FromDate: "2026-03-01", ToDate: "2026-03-31"}
		q := buildWorklogQuery(opts, pager, true, &lastSuccess)
		assert.Equal(t, "2026-03-01", q.Get("from"))
		assert.Equal(t, "2026-03-31", q.Get("to"))
		assert.Empty(t, q.Get("updatedFrom"))
	})

	t.Run("no bounds when nothing is known", func(t *testing.T) {
		q := buildWorklogQuery(&TempoOptions{TeamId: 4}, pager, false, nil)
		assert.Empty(t, q.Get("from"))
		assert.Empty(t, q.Get("updatedFrom"))
	})
}
