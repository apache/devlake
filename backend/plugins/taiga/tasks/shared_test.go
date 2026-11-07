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
	"net/http"
	"testing"

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

func TestTaigaProjectListQueryRequestsThePage(t *testing.T) {
	query := taigaProjectListQuery(42)

	got, err := query(&api.RequestData{Pager: &api.Pager{Page: 3, Size: 1000}})

	assert.Nil(t, err)
	assert.Equal(t, "42", got.Get("project"))
	// Taiga paginates list endpoints (30 items by default) and only returns
	// the page asked for, so the collector must send page and page_size.
	assert.Equal(t, "3", got.Get("page"))
	assert.Equal(t, "1000", got.Get("page_size"))
}

func TestGetNextTaigaPage(t *testing.T) {
	more := &http.Response{Header: http.Header{}}
	more.Header.Set("X-Pagination-Next", "https://taiga.example/api/v1/issues?page=2&page_size=1000&project=42")
	_, err := getNextTaigaPage(nil, more)
	assert.Nil(t, err)

	last := &http.Response{Header: http.Header{}}
	_, err = getNextTaigaPage(nil, last)
	assert.ErrorIs(t, err, api.ErrFinishCollect)
}
