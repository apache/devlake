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
	"github.com/stretchr/testify/require"
)

// The tool layer's scalar/JSON encoding of added/removed payloads:
// scalar when single-value, a JSON array string when multi-value; value ids
// and display strings encoded side by side.
func TestEncodeActivityValues(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantValues   string
		wantDisplays string
	}{
		{name: "missing payload", raw: `null`, wantValues: "", wantDisplays: ""},
		{name: "empty array", raw: `[]`, wantValues: "", wantDisplays: ""},
		{
			name:         "single state change is scalar",
			raw:          `[{"isResolved":true,"name":"Closed","id":"94-2","$type":"StateBundleElement"}]`,
			wantValues:   "94-2",
			wantDisplays: "Closed",
		},
		{
			name:         "multi-value tags round-trip as JSON",
			raw:          `[{"name":"Долго в работе","id":"6-1","$type":"Tag"},{"name":"backend","id":"6-2","$type":"Tag"}]`,
			wantValues:   `["6-1","6-2"]`,
			wantDisplays: `["Долго в работе","backend"]`,
		},
		{
			name:         "assignee change prefers fullName for display",
			raw:          `[{"login":"user-1","name":"User 1","fullName":"User 1","id":"1-34","$type":"User"}]`,
			wantValues:   "1-34",
			wantDisplays: "User 1",
		},
		{
			name:         "multi-user assignee change round-trips as JSON",
			raw:          `[{"login":"user-1","fullName":"User 1","id":"1-1","$type":"User"},{"login":"user-2","fullName":"User 2","id":"1-2","$type":"User"}]`,
			wantValues:   `["1-1","1-2"]`,
			wantDisplays: `["User 1","User 2"]`,
		},
		{
			name:         "summary change is a plain string",
			raw:          `"the old summary"`,
			wantValues:   "the old summary",
			wantDisplays: "the old summary",
		},
		{
			name:         "resolved change is an epoch-ms number",
			raw:          `1701853328660`,
			wantValues:   "1701853328660",
			wantDisplays: "1701853328660",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, displays, err := encodeActivityValues(json.RawMessage(tt.raw))
			require.NoError(t, err)
			assert.Equal(t, tt.wantValues, values)
			assert.Equal(t, tt.wantDisplays, displays)
		})
	}

	t.Run("absent payload", func(t *testing.T) {
		values, displays, err := encodeActivityValues(nil)
		require.NoError(t, err)
		assert.Equal(t, "", values)
		assert.Equal(t, "", displays)
	})
}

// FieldName must be stable: the custom field's own name for custom-field
// changes (never the localized PredefinedFilterField name).
func TestActivityFieldName(t *testing.T) {
	newItem := func(raw string) *apiActivityItem {
		item := &apiActivityItem{}
		require.NoError(t, json.Unmarshal([]byte(raw), item))
		return item
	}

	t.Run("custom field changes take the custom field's name", func(t *testing.T) {
		item := newItem(`{"field":{"name":"State","customField":{"name":"State"}},"targetMember":"__CUSTOM_FIELD__State_2"}`)
		assert.Equal(t, "State", activityFieldName(item))
	})

	t.Run("a renamed custom field keeps its real name", func(t *testing.T) {
		item := newItem(`{"field":{"name":"Client type","customField":{"name":"Client type"}},"targetMember":"__CUSTOM_FIELD__Client type_1"}`)
		assert.Equal(t, "Client type", activityFieldName(item))
	})

	t.Run("tags fall back to the target member, never the localized name", func(t *testing.T) {
		item := newItem(`{"field":{"name":"тег"},"targetMember":"tags"}`)
		assert.Equal(t, "tags", activityFieldName(item))
	})

	t.Run("resolved falls back to the target member", func(t *testing.T) {
		item := newItem(`{"field":{"name":"дата завершения"},"targetMember":"resolved"}`)
		assert.Equal(t, "resolved", activityFieldName(item))
	})

	t.Run("issue-created has no field name", func(t *testing.T) {
		item := newItem(`{"field":{"name":"создана"},"targetMember":null}`)
		assert.Equal(t, "", activityFieldName(item))
	})
}

// Only $type=User elements of an added/removed array derive into accounts.
func TestActivityItemUsers(t *testing.T) {
	t.Run("users are extracted, bundle elements and tags are not", func(t *testing.T) {
		raw := json.RawMessage(`[
			{"login":"user-1","fullName":"User 1","id":"1-1","$type":"User"},
			{"name":"Closed","id":"94-2","$type":"StateBundleElement"},
			{"name":"backend","id":"6-2","$type":"Tag"}
		]`)
		users := activityItemUsers(raw)
		require.Len(t, users, 1)
		assert.Equal(t, "1-1", users[0].Id)
		assert.Equal(t, "user-1", users[0].Login)
	})

	t.Run("scalar and null payloads yield nothing", func(t *testing.T) {
		assert.Empty(t, activityItemUsers(json.RawMessage(`"the old summary"`)))
		assert.Empty(t, activityItemUsers(json.RawMessage(`1701853328660`)))
		assert.Empty(t, activityItemUsers(json.RawMessage(`null`)))
		assert.Empty(t, activityItemUsers(nil))
	})
}
