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

	"github.com/apache/devlake/core/models/domainlayer/ticket"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// getStdStatus
// ---------------------------------------------------------------------------

func TestGetStdStatus(t *testing.T) {
	tests := []struct {
		statusKey string
		want      string
	}{
		{"done", ticket.DONE},
		{"new", ticket.TODO},
		{"indeterminate", ticket.IN_PROGRESS},
		// any unknown key should also map to IN_PROGRESS
		{"", ticket.IN_PROGRESS},
		{"unknown", ticket.IN_PROGRESS},
		{"in_progress", ticket.IN_PROGRESS},
	}
	for _, tt := range tests {
		t.Run(tt.statusKey, func(t *testing.T) {
			assert.Equal(t, tt.want, getStdStatus(tt.statusKey))
		})
	}
}

// ---------------------------------------------------------------------------
// getTypeMappings
// ---------------------------------------------------------------------------

func TestGetTypeMappings_NilScopeConfig(t *testing.T) {
	data := &JiraSnowflakeTaskData{
		Options: &JiraSnowflakeOptions{ScopeConfig: nil},
	}
	m, err := getTypeMappings(data, nil)
	require.Nil(t, err)
	assert.Empty(t, m.StdTypeMappings)
	assert.Empty(t, m.StandardStatusMappings)
}

func TestGetTypeMappings_EmptyScopeConfig(t *testing.T) {
	data := &JiraSnowflakeTaskData{
		Options: &JiraSnowflakeOptions{
			ScopeConfig: &jiramodels.JiraScopeConfig{},
		},
	}
	m, err := getTypeMappings(data, nil)
	require.Nil(t, err)
	assert.Empty(t, m.StdTypeMappings)
}

func TestGetTypeMappings_WithTypeMappings(t *testing.T) {
	data := &JiraSnowflakeTaskData{
		Options: &JiraSnowflakeOptions{
			ScopeConfig: &jiramodels.JiraScopeConfig{
				TypeMappings: map[string]jiramodels.TypeMapping{
					"Story": {
						StandardType: "requirement",
						StatusMappings: jiramodels.StatusMappings{
							"done": {StandardStatus: "DONE"},
						},
					},
					"Bug": {
						StandardType: "bug",
					},
				},
			},
		},
	}
	m, err := getTypeMappings(data, nil)
	require.Nil(t, err)

	assert.Equal(t, "REQUIREMENT", m.StdTypeMappings["Story"], "type name mapping should be uppercased")
	assert.Equal(t, "BUG", m.StdTypeMappings["Bug"])
	assert.Equal(t, "", m.StdTypeMappings["Task"], "unmapped type should return empty string")

	// status override for Story
	assert.Equal(t, jiramodels.StatusMappings{"done": {StandardStatus: "DONE"}}, m.StandardStatusMappings["Story"])
}
