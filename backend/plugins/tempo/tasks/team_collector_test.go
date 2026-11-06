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

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

func TestBuildTeamQuery(t *testing.T) {
	pager := &api.Pager{Page: 2, Skip: 50, Size: 50}

	t.Run("a team scope only fetches its own team", func(t *testing.T) {
		q := buildTeamQuery(&TempoOptions{ConnectionId: 1, TeamId: 71}, pager)
		assert.Equal(t, "71", q.Get("teamIds"))
		assert.Equal(t, "50", q.Get("offset"))
		assert.Equal(t, "50", q.Get("limit"))
		// not a GET /4/teams parameter
		assert.Empty(t, q.Get("updatedFrom"))
	})

	t.Run("without a team scope, fetches every team", func(t *testing.T) {
		q := buildTeamQuery(&TempoOptions{ConnectionId: 1}, nil)
		assert.Empty(t, q.Get("teamIds"))
		assert.Equal(t, "0", q.Get("offset"))
		assert.Equal(t, "50", q.Get("limit"))
	})
}
