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

package api

import (
	"errors"
	"testing"

	"github.com/merico-ai/graphql"
	"github.com/stretchr/testify/assert"
)

func TestIsIgnorableGraphqlQueryError(t *testing.T) {
	assert.True(t, isIgnorableGraphqlQueryError(errors.New("Could not resolve to an Issue with the number of 17.")))
	assert.False(t, isIgnorableGraphqlQueryError(errors.New("some other graphql error")))
	assert.False(t, isIgnorableGraphqlQueryError(nil))
}

func TestFormatGraphqlDataError(t *testing.T) {
	err := graphql.DataError{
		Message: "Could not resolve to a PullRequest with the number of 42.",
		Locations: []struct {
			Line   int
			Column int
		}{{Line: 3, Column: 5}},
	}
	got := formatGraphqlDataError(err, map[string]interface{}{
		"owner": "acme",
		"name":  "repo",
	})
	assert.Contains(t, got, "Could not resolve to a PullRequest with the number of 42.")
	assert.Contains(t, got, "line 3 col 5")
	assert.Contains(t, got, `"owner":"acme"`)
	assert.Contains(t, got, `"name":"repo"`)
}

func TestFormatGraphqlQueryFailure(t *testing.T) {
	got := formatGraphqlQueryFailure(errors.New("non-200 OK status code: 502"), map[string]interface{}{
		"pageSize": 10,
	})
	assert.Contains(t, got, "graphql query failed: non-200 OK status code: 502")
	assert.Contains(t, got, `"pageSize":10`)
}
