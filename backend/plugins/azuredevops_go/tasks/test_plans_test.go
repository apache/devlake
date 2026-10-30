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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTestResultReferenceIDsAreStrings(t *testing.T) {
	var result adoTestResult
	err := json.Unmarshal([]byte(`{
		"id": 501,
		"testCase": {"id": "100", "name": "Verify checkout"},
		"testPlan": {"id": "7"},
		"testCaseReferenceId": 100,
		"durationInMs": 5076.0
	}`), &result)
	assert.NoError(t, err)
	assert.Equal(t, "100", result.TestCase.ID)
	assert.Equal(t, "7", result.TestPlan.ID)
	assert.Equal(t, 100, result.TestCaseReferenceId)
	assert.Equal(t, 100, asInt(result.TestCase.ID))
	assert.Equal(t, 7, asInt(result.TestPlan.ID))
	if assert.NotNil(t, result.DurationInMs) {
		assert.Equal(t, 5076, int(*result.DurationInMs))
	}
}
