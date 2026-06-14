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

// TestIssuesToCollectChildrenClauses pins the incremental behaviour of the
// per-issue child collectors (comments, history): a full sync sweeps every
// issue, while an incremental run adds an updated_at filter so unchanged issues
// are skipped instead of triggering a request each run.
func TestIssuesToCollectChildrenClauses(t *testing.T) {
	// full sync: no `since` -> select/from/where(connection,team) only
	full := issuesToCollectChildrenClauses(1, "team-1", nil)
	assert.Len(t, full, 3)

	// incremental: a `since` adds the updated_at filter clause
	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	incremental := issuesToCollectChildrenClauses(1, "team-1", &since)
	assert.Len(t, incremental, 4)
}
