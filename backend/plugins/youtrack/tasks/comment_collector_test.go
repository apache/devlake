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

// The per-issue collectors' bounding rule: an incremental run
// walks only tool-layer issues with updated >= since; a full sync walks
// them all.
func TestChangedIssuesSince(t *testing.T) {
	since := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	t.Run("incremental run bounds the input by the bookmark", func(t *testing.T) {
		assert.Equal(t, &since, changedIssuesSince(true, &since))
	})

	t.Run("full sync walks all issues", func(t *testing.T) {
		assert.Nil(t, changedIssuesSince(false, &since))
	})

	t.Run("full sync without a bookmark walks all issues", func(t *testing.T) {
		assert.Nil(t, changedIssuesSince(false, nil))
	})
}
