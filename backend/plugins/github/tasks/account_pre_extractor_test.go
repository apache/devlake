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

func TestConvertAccountBotType(t *testing.T) {
	for _, tc := range []struct{ name, raw, wantLogin string }{
		{"bot alias", `{"login":"Copilot","id":175728472,"type":"Bot"}`, "Copilot[bot]"},
		{"suffixed bot", `{"login":"copilot-pull-request-reviewer[bot]","id":175728472,"type":"Bot"}`, "copilot-pull-request-reviewer[bot]"},
		{"human", `{"login":"octocat","id":583231,"type":"User"}`, "octocat"},
		{"no type", `{"login":"legacy","id":1}`, "legacy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var response GithubAccountResponse
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &response))
			original := response
			account, err := convertAccount(&response, 42, 7)
			require.NoError(t, err)
			assert.Equal(t, tc.wantLogin, account.Login)
			assert.Equal(t, response.Id, account.AccountId)
			assert.Equal(t, 42, account.RepoGithubId)
			assert.Equal(t, uint64(7), account.ConnectionId)
			assert.Equal(t, original, response, "normalization must not mutate the API response")
		})
	}
}
