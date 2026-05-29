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
	"time"

	"github.com/stretchr/testify/assert"
)

// TestBuildIssueFilter pins the server-side incremental filter that replaces
// the previous reliance on result ordering plus a client-side early-stop. A
// full sync must produce an empty filter (match all); an incremental run must
// produce Linear's IssueFilter shape `{ updatedAt: { gt: <RFC3339> } }`.
func TestBuildIssueFilter(t *testing.T) {
	full, err := json.Marshal(buildIssueFilter(nil))
	assert.NoError(t, err)
	assert.Equal(t, "{}", string(full))

	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	incremental, err := json.Marshal(buildIssueFilter(&since))
	assert.NoError(t, err)
	assert.JSONEq(t, `{"updatedAt":{"gt":"2026-05-01T00:00:00Z"}}`, string(incremental))
}
