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
	"strings"
	"testing"

	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

func newTestmoListResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetTotalPagesFromResponse(t *testing.T) {
	args := &helper.ApiCollectorArgs{PageSize: 100}

	t.Run("first of several pages", func(t *testing.T) {
		// Testmo list endpoints (RunPage, MilestonePage, AutomationRunPage)
		// return the pagination fields at the top level.
		res := newTestmoListResponse(`{"page": 1, "prev_page": null, "next_page": 2, "last_page": 3,
			"per_page": 100, "total": 250, "result": [{"id": 1}]}`)
		totalPages, err := GetTotalPagesFromResponse(res, args)
		assert.Nil(t, err)
		assert.Equal(t, 3, totalPages)
	})

	t.Run("single page", func(t *testing.T) {
		res := newTestmoListResponse(`{"page": 1, "prev_page": null, "next_page": null, "last_page": 1,
			"per_page": 100, "total": 2, "result": [{"id": 1}, {"id": 2}]}`)
		totalPages, err := GetTotalPagesFromResponse(res, args)
		assert.Nil(t, err)
		assert.Equal(t, 1, totalPages)
	})

	t.Run("no results", func(t *testing.T) {
		res := newTestmoListResponse(`{"page": null, "prev_page": null, "next_page": null, "last_page": null,
			"per_page": 100, "total": 0, "result": []}`)
		totalPages, err := GetTotalPagesFromResponse(res, args)
		assert.Nil(t, err)
		assert.Equal(t, 1, totalPages)
	})
}
