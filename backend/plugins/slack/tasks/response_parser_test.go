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
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/apache/devlake/core/errors"
	"github.com/stretchr/testify/assert"
)

func newSlackResponse(t *testing.T, method string, body string) *http.Response {
	u, err := url.Parse("https://slack.com/api/" + method)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    &http.Request{Method: http.MethodGet, URL: u},
	}
}

func TestSlackResponseParsers(t *testing.T) {
	parsers := []struct {
		method string
		field  string
		parse  func(*http.Response) ([]json.RawMessage, errors.Error)
	}{
		{"conversations.list", "channels", parseChannelResponse},
		{"conversations.history", "messages", parseChannelMessageResponse},
		{"conversations.replies", "messages", parseThreadResponse},
	}
	for _, p := range parsers {
		t.Run(p.method+" ok", func(t *testing.T) {
			res := newSlackResponse(t, p.method, `{"ok":true,"`+p.field+`":[{"id":"1"},{"id":"2"}]}`)
			items, err := p.parse(res)
			assert.Nil(t, err)
			assert.Len(t, items, 2)
		})
		t.Run(p.method+" not ok", func(t *testing.T) {
			res := newSlackResponse(t, p.method, `{"ok":false,"error":"not_in_channel"}`)
			items, err := p.parse(res)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "slack "+p.method+" error: not_in_channel")
			assert.Empty(t, items)
		})
		t.Run(p.method+" not ok without error code", func(t *testing.T) {
			res := newSlackResponse(t, p.method, `{"ok":false}`)
			_, err := p.parse(res)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "slack "+p.method+" error: unknown error")
		})
	}
}

func TestParseThreadResponseSkipsThreadNotFound(t *testing.T) {
	res := newSlackResponse(t, "conversations.replies", `{"ok":false,"error":"thread_not_found"}`)
	items, err := parseThreadResponse(res)
	assert.Nil(t, err)
	assert.Empty(t, items)
}
