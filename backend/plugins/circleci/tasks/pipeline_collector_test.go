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
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

const pipelinesPage = `{
	"items": [
		{"id": "p3", "created_at": "2026-03-01T00:00:00Z"},
		{"id": "p2", "created_at": "2026-02-01T00:00:00Z"},
		{"id": "p1", "created_at": "2026-01-01T00:00:00Z"}
	],
	"next_page_token": "token"
}`

func makePipelinesResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(pipelinesPage)),
		Request:    &http.Request{},
	}
}

func TestParsePipelinesWithoutTimeAfter(t *testing.T) {
	// An advanced mode blueprint has no timeAfter: keep every pipeline
	// instead of dereferencing a nil time.
	items, err := parsePipelines(makePipelinesResponse(), nil)
	assert.Nil(t, err)
	assert.Len(t, items, 3)
}

func TestParsePipelinesStopsBeforeTimeAfter(t *testing.T) {
	timeAfter := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	items, err := parsePipelines(makePipelinesResponse(), &timeAfter)
	assert.ErrorIs(t, err, api.ErrFinishCollect)
	assert.Len(t, items, 2)
	assert.Contains(t, string(items[0]), `"p3"`)
	assert.Contains(t, string(items[1]), `"p2"`)
}
