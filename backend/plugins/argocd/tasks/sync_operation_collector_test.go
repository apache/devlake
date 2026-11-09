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

func TestSanitizeOperationEntries_SkipsNulls(t *testing.T) {
	opState := json.RawMessage("null")
	history := []json.RawMessage{
		json.RawMessage("null"),
		json.RawMessage(`{"id": 1}`),
		json.RawMessage("  "),
	}

	result := sanitizeOperationEntries(opState, history)

	if assert.Len(t, result, 1) {
		assert.JSONEq(t, `{"id": 1}`, string(result[0]))
	}
}

func TestSanitizeOperationEntries_PreservesOperationState(t *testing.T) {
	opState := json.RawMessage(`{"phase": "Running"}`)
	history := []json.RawMessage{
		json.RawMessage(`{"id": 2}`),
	}

	result := sanitizeOperationEntries(opState, history)

	if assert.Len(t, result, 2) {
		assert.JSONEq(t, `{"phase": "Running"}`, string(result[0]))
		assert.JSONEq(t, `{"id": 2}`, string(result[1]))
	}
}

func TestSanitizeOperationEntries_TrimsWhitespace(t *testing.T) {
	opState := json.RawMessage("   ")
	history := []json.RawMessage{
		json.RawMessage("  "),
		json.RawMessage(` { "id":3 } `),
	}

	result := sanitizeOperationEntries(opState, history)

	if assert.Len(t, result, 1) {
		assert.JSONEq(t, `{ "id":3 }`, string(result[0]))
	}
}

func TestRecordHealthOnLatestSync_TagsOnlyNewestHistoryEntry(t *testing.T) {
	history := []json.RawMessage{
		json.RawMessage(`{"id": 1, "revision": "aaa"}`),
		json.RawMessage(`{"id": 2, "revision": "bbb"}`),
		json.RawMessage("null"),
	}

	recordHealthOnLatestSync(json.RawMessage(`{"phase": "Succeeded"}`), history, "Degraded")

	assert.JSONEq(t, `{"id": 1, "revision": "aaa"}`, string(history[0]))
	assert.JSONEq(t, `{"id": 2, "revision": "bbb", "applicationHealth": "Degraded"}`, string(history[1]))
	assert.Equal(t, "null", string(history[2]))
}

func TestRecordHealthOnLatestSync_NoOperationState(t *testing.T) {
	history := []json.RawMessage{json.RawMessage(`{"id": 7}`)}

	recordHealthOnLatestSync(nil, history, "Healthy")

	assert.JSONEq(t, `{"id": 7, "applicationHealth": "Healthy"}`, string(history[0]))
}

func TestRecordHealthOnLatestSync_NewerOperationOwnsTheHealth(t *testing.T) {
	for _, phase := range []string{"Failed", "Error", "Running", "Terminating"} {
		history := []json.RawMessage{json.RawMessage(`{"id": 1}`)}

		recordHealthOnLatestSync(json.RawMessage(`{"phase": "`+phase+`"}`), history, "Degraded")

		assert.JSONEq(t, `{"id": 1}`, string(history[0]), phase)
	}
}

func TestRecordHealthOnLatestSync_NoHealthReported(t *testing.T) {
	history := []json.RawMessage{json.RawMessage(`{"id": 1}`)}

	recordHealthOnLatestSync(nil, history, "")

	assert.JSONEq(t, `{"id": 1}`, string(history[0]))
}
