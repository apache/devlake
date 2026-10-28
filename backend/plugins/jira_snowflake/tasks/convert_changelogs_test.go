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
	"strings"
	"testing"

	"github.com/apache/devlake/core/models/domainlayer/didgen"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// convertSprintIds
// ---------------------------------------------------------------------------

func newSprintIdGen() *didgen.DomainIdGenerator {
	return didgen.NewDomainIdGenerator(&jiramodels.JiraSprint{})
}

func TestConvertSprintIds_Empty(t *testing.T) {
	gen := newSprintIdGen()
	result, err := convertSprintIds("", 1, gen)
	require.Nil(t, err)
	assert.Equal(t, "", result)
}

func TestConvertSprintIds_Single(t *testing.T) {
	gen := newSprintIdGen()
	result, err := convertSprintIds("42", 1, gen)
	require.Nil(t, err)
	assert.NotEmpty(t, result)
	// The generated domain ID should contain the sprint number
	assert.Contains(t, result, "42")
}

func TestConvertSprintIds_Multiple(t *testing.T) {
	gen := newSprintIdGen()
	result, err := convertSprintIds("10, 20, 30", 1, gen)
	require.Nil(t, err)
	// Comma-separated output for multiple sprint IDs
	parts := splitAndTrim(result, ",")
	assert.Len(t, parts, 3)
}

func TestConvertSprintIds_JiraFormatWithLabel(t *testing.T) {
	// Jira often stores sprint values like "com.atlassian.greenhopper.service.sprint.Sprint@abc[id=42,...]"
	gen := newSprintIdGen()
	result, err := convertSprintIds("com.atlassian.greenhopper.service.sprint.Sprint@abc[id=42,rapidViewId=100]", 1, gen)
	require.Nil(t, err)
	// Should still extract numeric ID 42
	assert.NotEmpty(t, result)
}

func TestConvertSprintIds_InvalidNumber(t *testing.T) {
	// A string that contains no digits at all
	gen := newSprintIdGen()
	result, err := convertSprintIds("no-digits-here", 1, gen)
	require.Nil(t, err)
	assert.Equal(t, "", result)
}

func TestConvertSprintIds_DeterministicForSameInput(t *testing.T) {
	gen := newSprintIdGen()
	r1, _ := convertSprintIds("99", 5, gen)
	r2, _ := convertSprintIds("99", 5, gen)
	assert.Equal(t, r1, r2, "same input should always produce same domain ID")
}

func TestConvertSprintIds_DifferentConnectionIds(t *testing.T) {
	gen := newSprintIdGen()
	r1, _ := convertSprintIds("99", 1, gen)
	r2, _ := convertSprintIds("99", 2, gen)
	assert.NotEqual(t, r1, r2, "different connection IDs should produce different domain IDs")
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		if p := strings.TrimSpace(part); p != "" {
			result = append(result, p)
		}
	}
	return result
}
