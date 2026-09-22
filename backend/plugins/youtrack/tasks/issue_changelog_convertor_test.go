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

	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// didgen resolves the plugin name through the plugin registry, which a bare
// unit test doesn't have — register a stub first.
func registerYoutrackPlugin(t *testing.T) {
	mockMeta := mockplugin.NewPluginMeta(t)
	mockMeta.On("RootPkgPath").Return("github.com/apache/devlake/plugins/youtrack")
	mockMeta.On("Name").Return("youtrack").Maybe()
	_ = plugin.RegisterPlugin("youtrack", mockMeta)
}

func TestDecodeChangelogValues(t *testing.T) {
	assert.Nil(t, decodeChangelogValues(""))
	assert.Equal(t, []string{"94-2"}, decodeChangelogValues("94-2"))
	assert.Equal(t, []string{"6-1", "6-2"}, decodeChangelogValues(`["6-1","6-2"]`))
	// a scalar that happens to start with [ falls back to scalar
	assert.Equal(t, []string{"[not json"}, decodeChangelogValues("[not json"))
}

// State changes convert through the same shared helper as issues: the
// configured mapping first, else the workflow_states isResolved lookup.
func TestStdChangelogValues(t *testing.T) {
	isResolvedByName := map[string]bool{"Closed": true, "Backlog": false}

	t.Run("configured mapping wins", func(t *testing.T) {
		value, err := stdChangelogValues(map[string]string{"Backlog": ticket.OTHER}, isResolvedByName, "Backlog")
		require.NoError(t, err)
		assert.Equal(t, ticket.OTHER, value)
	})

	t.Run("isResolved fallback: resolved state maps to DONE", func(t *testing.T) {
		value, err := stdChangelogValues(nil, isResolvedByName, "Closed")
		require.NoError(t, err)
		assert.Equal(t, ticket.DONE, value)
	})

	t.Run("isResolved fallback: unresolved state maps to TODO", func(t *testing.T) {
		value, err := stdChangelogValues(nil, isResolvedByName, "Backlog")
		require.NoError(t, err)
		assert.Equal(t, ticket.TODO, value)
	})

	t.Run("a state name the bundle no longer knows degrades to TODO", func(t *testing.T) {
		value, err := stdChangelogValues(nil, isResolvedByName, "Renamed Away")
		require.NoError(t, err)
		assert.Equal(t, ticket.TODO, value)
	})

	t.Run("multi-value changes keep the JSON shape", func(t *testing.T) {
		value, err := stdChangelogValues(nil, isResolvedByName, `["Backlog","Closed"]`)
		require.NoError(t, err)
		assert.Equal(t, `["TODO","DONE"]`, value)
	})

	t.Run("empty stays empty", func(t *testing.T) {
		value, err := stdChangelogValues(nil, isResolvedByName, "")
		require.NoError(t, err)
		assert.Equal(t, "", value)
	})
}

// Assignee changes convert user ids to didgen account ids, shape-preserving.
func TestAccountChangelogValues(t *testing.T) {
	registerYoutrackPlugin(t)
	accountIdGen := didgen.NewDomainIdGenerator(&models.YoutrackAccount{})

	t.Run("scalar user id", func(t *testing.T) {
		value, err := accountChangelogValues(accountIdGen, 1, "1-34")
		require.NoError(t, err)
		assert.Equal(t, "youtrack:YoutrackAccount:1:1-34", value)
	})

	t.Run("multi-user change keeps the JSON shape", func(t *testing.T) {
		value, err := accountChangelogValues(accountIdGen, 1, `["1-1","1-2"]`)
		require.NoError(t, err)
		assert.Equal(t, `["youtrack:YoutrackAccount:1:1-1","youtrack:YoutrackAccount:1:1-2"]`, value)
	})

	t.Run("empty stays empty", func(t *testing.T) {
		value, err := accountChangelogValues(accountIdGen, 1, "")
		require.NoError(t, err)
		assert.Equal(t, "", value)
	})
}

// The scalar/JSON encoding round-trips through extract and convert.
func TestChangelogValuesRoundTrip(t *testing.T) {
	raw := `[{"name":"Долго в работе","id":"6-1","$type":"Tag"},{"name":"backend","id":"6-2","$type":"Tag"}]`
	values, displays, err := encodeActivityValues([]byte(raw))
	require.NoError(t, err)
	assert.Equal(t, []string{"6-1", "6-2"}, decodeChangelogValues(values))
	assert.Equal(t, []string{"Долго в работе", "backend"}, decodeChangelogValues(displays))
}
