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

	"github.com/apache/devlake/plugins/tempo/models"
	"github.com/stretchr/testify/assert"
)

func TestConvertWorklog(t *testing.T) {
	tempoWorklog := &models.TempoWorklog{
		ConnectionId:     1,
		TempoWorklogId:   12345,
		IssueId:          10001,
		AuthorAccountId:  "abc123",
		TimeSpentSeconds: 3600,
		StartDate:        "2024-01-15",
		StartTime:        "09:00:00",
		Description:      "Working on task",
		CreatedAt:        "2024-01-15T10:00:00Z",
	}

	worklog := convertWorklog(1, tempoWorklog)
	assert.Equal(t, "tempo:TempoWorklog:1:12345", worklog.Id)
	assert.Equal(t, "jira:JiraIssue:1:10001", worklog.IssueId)
	assert.Equal(t, "jira:JiraAccount:1:abc123", worklog.AuthorId)
	assert.Equal(t, 60, worklog.TimeSpentMinutes)
	assert.Equal(t, "Working on task", worklog.Comment)
	if assert.NotNil(t, worklog.StartedDate) {
		assert.Equal(t, time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC), *worklog.StartedDate)
	}
	if assert.NotNil(t, worklog.LoggedDate) {
		assert.Equal(t, time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), *worklog.LoggedDate)
	}
}

func TestConvertWorklogIdsAreUniquePerConnection(t *testing.T) {
	// Tempo worklog ids are only unique within one Tempo instance: two
	// connections can both have worklog 12345, and their domain rows must
	// not share a primary key.
	first := convertWorklog(1, &models.TempoWorklog{ConnectionId: 1, TempoWorklogId: 12345, IssueId: 10001})
	second := convertWorklog(2, &models.TempoWorklog{ConnectionId: 2, TempoWorklogId: 12345, IssueId: 20001})
	assert.NotEqual(t, first.Id, second.Id)
	assert.Equal(t, "tempo:TempoWorklog:2:12345", second.Id)
}
