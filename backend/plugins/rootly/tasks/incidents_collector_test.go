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

func TestBuildIncidentsQuery_FirstPageNoSince(t *testing.T) {
	q := buildIncidentsQuery("svc_42", 100, 1, nil)
	assert.Equal(t, "svc_42", q.Get("filter[service_ids]"))
	assert.Equal(t, "100", q.Get("page[size]"))
	assert.Equal(t, "1", q.Get("page[number]"))
	assert.Equal(t, "-updated_at", q.Get("sort"))
	assert.Equal(t, "", q.Get("filter[updated_at][gt]"))
	assert.Equal(t, "", q.Get("filter[services]"), "regression guard: must be filter[service_ids], not filter[services]")
}

func TestBuildIncidentsQuery_SubsequentPage(t *testing.T) {
	q := buildIncidentsQuery("svc_42", 100, 3, nil)
	assert.Equal(t, "3", q.Get("page[number]"))
}

func TestBuildIncidentsQuery_WithSince(t *testing.T) {
	since := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	q := buildIncidentsQuery("svc_42", 100, 1, &since)
	assert.Equal(t, "2026-05-01T12:00:00Z", q.Get("filter[updated_at][gt]"))
}
