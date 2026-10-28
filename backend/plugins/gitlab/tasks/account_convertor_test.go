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

	"github.com/stretchr/testify/assert"

	"github.com/apache/devlake/plugins/gitlab/models"
)

func TestIsGitlabBotAccount(t *testing.T) {
	tests := []struct {
		name     string
		username string
		fullName string
		want     bool
	}{
		// Project/group access tokens: ^(project|group)_[0-9]+_bot...
		{"project access token", "project_78877_bot_ec63972b1234567890abcdef", "", true},
		{"group access token", "group_80889_bot_73ed0037abcdef123456", "", true},

		// Hyphenated -bot/-robot suffixes.
		{"hyphenated bot suffix", "pnc-stage-bot", "", true},
		{"hyphenated bot suffix variant", "snyk-broker-bot", "", true},
		{"hyphenated robot suffix", "openshift-cherrypick-robot", "", true},

		// Name contains "Service Account" (case-insensitive), independent of username shape.
		{"service account name marks bot", "renovate-app", "Renovate Service Account", true},
		{"service account name case-insensitive", "ci-runner", "SERVICE ACCOUNT for CI", true},
		{"known prod example: username and name both signal bot", "rh-renovate-bot", "renovate service account", true},

		// Negative cases: must not false-positive on substrings or near-miss shapes.
		{"normal human user", "jsmith", "John Smith", false},
		{"username containing bot mid-string", "robotics-team-lead", "", false},
		{"name containing bot substring without service account phrase", "abbott", "Abbott Contractor", false},
		{"project id present but no bot marker", "project_78877_admin", "", false},
		{"bot as a separate segment, not a suffix or token prefix", "team_bot_helper", "", false},
		{"underscore before bot without project/group token prefix", "user_78877_bot", "", false},
		{"empty name and no bot marker", "alice", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGitlabBotAccount(tt.username, tt.fullName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildGitlabDomainAccount_SetsIsBot(t *testing.T) {
	tests := []struct {
		name      string
		account   *models.GitlabAccount
		wantIsBot bool
	}{
		{"human user", &models.GitlabAccount{Username: "jsmith", Name: "John Smith"}, false},
		{"project token bot", &models.GitlabAccount{Username: "project_123_bot_abc"}, true},
		{"service account name", &models.GitlabAccount{Username: "renovate-app", Name: "Renovate Service Account"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGitlabDomainAccount("gitlab:GitlabAccount:1:1", tt.account)
			assert.Equal(t, tt.wantIsBot, got.IsBot)
			assert.Equal(t, tt.account.Username, got.UserName)
		})
	}
}
