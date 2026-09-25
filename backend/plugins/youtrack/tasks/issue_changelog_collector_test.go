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
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/apache/devlake/core/errors"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func activityPageResponse(body string) *http.Response {
	u, _ := url.Parse("https://youtrack.example.com/youtrack/api/issues/2-1/activitiesPage")
	return &http.Response{
		StatusCode: 200,
		Request:    &http.Request{URL: u},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// The cursor discipline: categories is mandatory and repeats with
// fields/$top on every cursor request; the opaque cursor is URL-encoded by
// url.Values encoding (it contains +, ^ and : — a raw + would decode as a
// space on the server).
func TestChangelogQuery(t *testing.T) {
	t.Run("first request carries filters and no cursor", func(t *testing.T) {
		query, err := changelogQuery(&helper.RequestData{}, 100)
		require.NoError(t, err)
		assert.Equal(t, changelogCategories, query.Get("categories"))
		assert.Equal(t, changelogFields, query.Get("fields"))
		assert.Equal(t, "100", query.Get("$top"))
		assert.Empty(t, query.Get("cursor"))
	})

	t.Run("exactly the five configured categories are requested", func(t *testing.T) {
		query, err := changelogQuery(&helper.RequestData{}, 100)
		require.NoError(t, err)
		assert.Equal(t,
			"CustomFieldCategory,IssueCreatedCategory,IssueResolvedCategory,SummaryCategory,TagsCategory",
			query.Get("categories"))
	})

	t.Run("cursor request repeats the filters and adds the cursor", func(t *testing.T) {
		reqData := &helper.RequestData{CustomData: "AI.2-99664+:RH.9-1787322+:1789731977844"}
		query, err := changelogQuery(reqData, 100)
		require.NoError(t, err)
		assert.Equal(t, changelogCategories, query.Get("categories"))
		assert.Equal(t, changelogFields, query.Get("fields"))
		assert.Equal(t, "AI.2-99664+:RH.9-1787322+:1789731977844", query.Get("cursor"))
		// url.Values.Encode is what the api client applies to the query
		// string — the cursor's + and : must be percent-encoded on the wire
		assert.Contains(t, query.Encode(), "cursor=AI.2-99664%2B%3ARH.9-1787322%2B%3A1789731977844")
	})

	t.Run("empty cursor is dropped", func(t *testing.T) {
		query, err := changelogQuery(&helper.RequestData{CustomData: ""}, 100)
		require.NoError(t, err)
		assert.Empty(t, query.Get("cursor"))
	})
}

// The walk terminates cleanly on hasAfter=false; otherwise the page's
// afterCursor drives the next request.
func TestNextActivityCursor(t *testing.T) {
	t.Run("hasAfter=false finishes the walk", func(t *testing.T) {
		res := activityPageResponse(`{"activities":[{"id":"0-0.9-1"}],"hasAfter":false,"afterCursor":"A^B"}`)
		next, err := nextActivityCursor(&helper.RequestData{}, res)
		assert.Nil(t, next)
		assert.True(t, errors.Is(err, helper.ErrFinishCollect))
	})

	t.Run("hasAfter=true hands the opaque cursor to the next request", func(t *testing.T) {
		res := activityPageResponse(`{"activities":[{"id":"0-0.9-1"}],"hasAfter":true,"afterCursor":"AI.2-99664+:RH.9-1787322+:1789731977844"}`)
		next, err := nextActivityCursor(&helper.RequestData{}, res)
		require.NoError(t, err)
		assert.Equal(t, "AI.2-99664+:RH.9-1787322+:1789731977844", next)
	})

	t.Run("an empty last page finishes the walk", func(t *testing.T) {
		res := activityPageResponse(`{"activities":[],"hasAfter":false}`)
		next, err := nextActivityCursor(&helper.RequestData{}, res)
		assert.Nil(t, next)
		assert.True(t, errors.Is(err, helper.ErrFinishCollect))
	})
}

// The raw-table grain is one row per activity item.
func TestParseActivityPageResponse(t *testing.T) {
	res := activityPageResponse(`{"activities":[{"id":"0-0.9-1"},{"id":"0-0.9-2"},{"id":"0-0.9-3"}],"hasAfter":false}`)
	items, err := parseActivityPageResponse(res)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.JSONEq(t, `{"id":"0-0.9-1"}`, string(items[0]))
}
