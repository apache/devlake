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

func TestBuildIssueQuery(t *testing.T) {
	t.Run("full sync without timeAfter has no updated clause", func(t *testing.T) {
		assert.Equal(t,
			"project: {PROJ1} sort by: created asc",
			buildIssueQuery("PROJ1", nil))
	})

	t.Run("window opens 26h before since (the profile-timezone overlap) and has no upper bound", func(t *testing.T) {
		since := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
		assert.Equal(t,
			"project: {PROJ1} updated: {2026-09-19T08:00} .. * sort by: created asc",
			buildIssueQuery("PROJ1", &since))
	})

	t.Run("dates are rendered in UTC minute precision", func(t *testing.T) {
		// +05:00 input must render as UTC — the query must never depend on
		// the collector host's timezone either
		loc := time.FixedZone("UTC+5", 5*60*60)
		since := time.Date(2026, 9, 20, 15, 30, 0, 0, loc)
		assert.Equal(t,
			"project: {PROJ1} updated: {2026-09-19T08:30} .. * sort by: created asc",
			buildIssueQuery("PROJ1", &since))
	})
}
