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

	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/apache/devlake/plugins/github/models"
	"github.com/stretchr/testify/assert"
)

func TestIssueLeadTimeMinutes(t *testing.T) {
	createdAt := time.Date(2026, 11, 1, 9, 49, 53, 0, time.UTC)

	leadTime := IssueLeadTimeMinutes(createdAt, createdAt.Add(90*time.Minute))
	if assert.NotNil(t, leadTime) {
		assert.Equal(t, uint(90), *leadTime)
	}

	leadTime = IssueLeadTimeMinutes(createdAt, createdAt)
	if assert.NotNil(t, leadTime) {
		assert.Equal(t, uint(0), *leadTime)
	}

	// Closed before created, as GitHub reports for some transferred issues.
	assert.Nil(t, IssueLeadTimeMinutes(createdAt, createdAt.Add(-44*time.Minute)))
}

func TestExtractApiIssuesReturnsIssueRegexError(t *testing.T) {
	mockDal := new(mockdal.Dal)
	taskCtx := unithelper.DummySubTaskContext(mockDal)
	taskCtx.On("GetData").Return(&GithubTaskData{
		Options: &GithubOptions{
			ConnectionId: 1,
			Name:         "apache/devlake",
			ScopeConfig:  &models.GithubScopeConfig{IssueSeverity: "severity/(.*"},
		},
	})

	err := ExtractApiIssues(taskCtx)

	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "issueSeverity")
	}
}
