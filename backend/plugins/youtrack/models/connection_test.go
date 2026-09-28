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

package models

import (
	"testing"

	"github.com/apache/devlake/core/utils"
	"github.com/stretchr/testify/assert"
)

func TestYoutrackConnectionSanitize(t *testing.T) {
	connection := YoutrackConnection{}
	connection.Token = "perm:am9obi5kb2Uuc3VwZXJzZWNyZXQ"
	sanitized := connection.Sanitize()
	// the token is masked, never echoed back verbatim
	assert.NotEqual(t, connection.Token, sanitized.Token)
	assert.NotContains(t, sanitized.Token, "c3VwZXJzZWNyZXQ")
	// the original is untouched (Sanitize works on a copy)
	assert.Equal(t, "perm:am9obi5kb2Uuc3VwZXJzZWNyZXQ", connection.Token)
}

func TestYoutrackConnectionMergeFromRequest(t *testing.T) {
	const storedToken = "perm:am9obi5kb2Uuc3VwZXJzZWNyZXQ"
	stored := func() *YoutrackConnection {
		c := &YoutrackConnection{}
		c.Name = "my youtrack"
		c.Endpoint = "https://example.myjetbrains.com/youtrack/api"
		c.Token = storedToken
		return c
	}

	t.Run("PATCHing back the sanitized token preserves the stored one", func(t *testing.T) {
		connection := stored()
		err := connection.MergeFromRequest(connection, map[string]interface{}{
			"name":  "renamed",
			"token": utils.SanitizeString(storedToken),
		})
		assert.NoError(t, err)
		assert.Equal(t, "renamed", connection.Name)
		assert.Equal(t, storedToken, connection.Token)
	})

	t.Run("PATCHing an empty token preserves the stored one", func(t *testing.T) {
		connection := stored()
		err := connection.MergeFromRequest(connection, map[string]interface{}{
			"token": "",
		})
		assert.NoError(t, err)
		assert.Equal(t, storedToken, connection.Token)
	})

	t.Run("PATCH without a token key leaves the stored one alone", func(t *testing.T) {
		connection := stored()
		err := connection.MergeFromRequest(connection, map[string]interface{}{
			"name": "renamed again",
		})
		assert.NoError(t, err)
		assert.Equal(t, storedToken, connection.Token)
		assert.Equal(t, "https://example.myjetbrains.com/youtrack/api", connection.Endpoint)
	})

	t.Run("PATCHing a genuinely new token replaces the stored one", func(t *testing.T) {
		connection := stored()
		err := connection.MergeFromRequest(connection, map[string]interface{}{
			"token": "perm:bmV3LnRva2Vu",
		})
		assert.NoError(t, err)
		assert.Equal(t, "perm:bmV3LnRva2Vu", connection.Token)
	})
}
