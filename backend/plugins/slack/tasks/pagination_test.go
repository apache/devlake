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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	mockapi "github.com/apache/devlake/mocks/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type slackTestPage struct {
	items      int
	nextCursor string
}

// runSlackCollector runs the collector built by newArgs against a fake Slack
// API that serves pages in order, and returns the query of each request and
// the number of raw rows saved.
func runSlackCollector(
	t *testing.T,
	field string,
	pages []slackTestPage,
	newArgs func(rawArgs api.RawDataSubTaskArgs, apiClient api.RateLimitedApiClient) api.ApiCollectorArgs,
) ([]url.Values, int) {
	savedRows := 0
	mockDal := new(mockdal.Dal)
	mockDal.On("AutoMigrate", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockDal.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		savedRows += len(args.Get(0).([]*api.RawData))
	}).Return(nil)

	var queries []url.Values
	mockApi := new(mockapi.RateLimitedApiClient)
	mockApi.On("DoGetAsync", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		query := args.Get(1).(url.Values)
		queries = append(queries, query)
		if len(queries) > len(pages) {
			t.Fatalf("unexpected request #%d: %v", len(queries), query)
		}
		page := pages[len(queries)-1]
		items := make([]string, page.items)
		for i := range items {
			items[i] = fmt.Sprintf(`{"ts":"%d.%d"}`, len(queries), i)
		}
		body := fmt.Sprintf(`{"ok":true,"%s":[%s],"response_metadata":{"next_cursor":"%s"}}`,
			field, strings.Join(items, ","), page.nextCursor)
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    &http.Request{Method: http.MethodGet, URL: &url.URL{Scheme: "https", Host: "slack.com"}},
		}
		handler := args.Get(3).(plugin.ApiAsyncCallback)
		assert.Nil(t, handler(res))
	})
	mockApi.On("NextTick", mock.Anything).Run(func(args mock.Arguments) {
		assert.Nil(t, args.Get(0).(func() errors.Error)())
	})
	mockApi.On("HasError").Return(false)
	mockApi.On("WaitAsync").Return(nil)
	mockApi.On("SetAfterFunction", mock.Anything).Return()

	collector, err := api.NewApiCollector(newArgs(api.RawDataSubTaskArgs{
		Ctx:    unithelper.DummySubTaskContext(mockDal),
		Params: SlackApiParams{ConnectionId: 1},
		Table:  "slack_test",
	}, mockApi))
	assert.Nil(t, err)
	assert.Nil(t, collector.Execute())
	return queries, savedRows
}

func TestSlackCollectorsFollowCursorPastShortPages(t *testing.T) {
	collectors := []struct {
		name    string
		field   string
		limit   string
		newArgs func(rawArgs api.RawDataSubTaskArgs, apiClient api.RateLimitedApiClient) api.ApiCollectorArgs
	}{
		{"conversations.list", "channels", "100", newChannelCollectorArgs},
		{"conversations.history", "messages", "100",
			func(rawArgs api.RawDataSubTaskArgs, apiClient api.RateLimitedApiClient) api.ApiCollectorArgs {
				input := api.NewQueueIterator()
				input.Push(&ChannelInput{ChannelId: "C1"})
				return newChannelMessageCollectorArgs(rawArgs, apiClient, input)
			}},
		{"conversations.replies", "messages", "50",
			func(rawArgs api.RawDataSubTaskArgs, apiClient api.RateLimitedApiClient) api.ApiCollectorArgs {
				input := api.NewQueueIterator()
				input.Push(&ThreadInput{ChannelId: "C1", ThreadTs: "1.0"})
				return newThreadCollectorArgs(rawArgs, apiClient, input)
			}},
	}
	for _, c := range collectors {
		t.Run(c.name, func(t *testing.T) {
			// Every page is shorter than the requested limit. Only the last
			// one has an empty next_cursor.
			queries, savedRows := runSlackCollector(t, c.field, []slackTestPage{
				{items: 2, nextCursor: "cursor-2"},
				{items: 1, nextCursor: "cursor-3"},
				{items: 3, nextCursor: ""},
			}, c.newArgs)

			assert.Equal(t, 6, savedRows)
			if assert.Len(t, queries, 3) {
				assert.False(t, queries[0].Has("cursor"))
				assert.Equal(t, "cursor-2", queries[1].Get("cursor"))
				assert.Equal(t, "cursor-3", queries[2].Get("cursor"))
				for _, q := range queries {
					assert.Equal(t, c.limit, q.Get("limit"))
				}
			}
		})
	}
}
