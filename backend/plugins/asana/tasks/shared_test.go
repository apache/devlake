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

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

func asanaListPage(t *testing.T, body string) *http.Response {
	req, err := http.NewRequest(http.MethodGet, "https://app.asana.com/api/1.0/tasks/1/stories", nil)
	assert.Nil(t, err)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func TestSetAsanaPageQuery(t *testing.T) {
	first := url.Values{}
	setAsanaPageQuery(first, &api.RequestData{Pager: &api.Pager{Page: 1, Size: asanaPageSize}})
	assert.Equal(t, "100", first.Get("limit"))
	assert.False(t, first.Has("offset"))

	next := url.Values{}
	setAsanaPageQuery(next, &api.RequestData{Pager: &api.Pager{Page: 2, Size: asanaPageSize}, CustomData: "eyJ0eXAiOJiKV1iQLCJhbGciOiJIUzI1NiJ9"})
	assert.Equal(t, "100", next.Get("limit"))
	assert.Equal(t, "eyJ0eXAiOJiKV1iQLCJhbGciOiJIUzI1NiJ9", next.Get("offset"))
}

func TestGetNextAsanaPageOffset(t *testing.T) {
	offset, err := getNextAsanaPageOffset(nil, asanaListPage(t, `{
		"data": [{"gid": "1"}],
		"next_page": {"offset": "eyJ0eXAiOJiKV1iQLCJhbGciOiJIUzI1NiJ9", "path": "/tasks/1/stories?limit=100&offset=eyJ0eXAiOJiKV1iQLCJhbGciOiJIUzI1NiJ9", "uri": "https://app.asana.com/api/1.0/tasks/1/stories?limit=100&offset=eyJ0eXAiOJiKV1iQLCJhbGciOiJIUzI1NiJ9"}
	}`))
	assert.Nil(t, err)
	assert.Equal(t, "eyJ0eXAiOJiKV1iQLCJhbGciOiJIUzI1NiJ9", offset)

	// A full last page has "next_page": null. Going on without an offset
	// would request page 1 again, so the collection must end here.
	_, err = getNextAsanaPageOffset(nil, asanaListPage(t, `{"data": [{"gid": "1"}], "next_page": null}`))
	assert.ErrorIs(t, err, api.ErrFinishCollect)
}
