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
)

func TestIsBotAccount(t *testing.T) {
	tests := []struct {
		name        string
		login       string
		accountType string
		want        bool
	}{
		// API-reported type takes precedence over any login pattern.
		{"api type marks bot even without suffix", "some-github-app", "Bot", true},

		// [bot] suffix - GitHub Apps.
		{"github app bot suffix", "renovate[bot]", "", true},
		{"github-actions bot suffix", "github-actions[bot]", "", true},
		{"codecov bot suffix", "codecov-commenter[bot]", "", true},
		{"coderabbit bot suffix", "coderabbit[bot]", "", true},
		{"cursor bot suffix", "cursor[bot]", "", true},
		{"claude bot suffix", "claude[bot]", "", true},
		{"copilot reviewer bot suffix", "copilot-pull-request-reviewer[bot]", "", true},

		// Known bot logins referenced by DPROD-1262.
		{"triage bot login", "konflux-ci-triage[bot]", "", true},
		{"review bot login", "konflux-ci-review[bot]", "", true},
		{"coder bot login", "konflux-ci-coder[bot]", "", true},
		{"fullsend bot login", "konflux-ci-fullsend[bot]", "", true},

		// Hyphenated -bot/-robot suffixes (no brackets).
		{"hyphenated bot suffix", "openshift-bot", "", true},
		{"hyphenated robot suffix", "openshift-cherrypick-robot", "", true},
		{"redhat renovate bot suffix", "redhat-renovate-bot", "", true},
		{"snyk bot suffix", "snyk-bot", "", true},

		// Standalone names without a suffix marker.
		{"standalone copilot login", "copilot", "", true},
		{"dependabot without brackets", "dependabot", "", true},
		{"github-actions without brackets", "github-actions", "", true},
		{"codecov commenter bot, real GitHub API type is User", "codecov-commenter", "User", true},

		// Negative cases: must not false-positive on substrings, only
		// on the known suffixes/markers above.
		{"normal human user", "octocat", "User", false},
		{"organization account is not a bot", "my-org", "Organization", false},
		{"surname containing bot as a substring", "abbott", "User", false},
		{"login containing bot mid-string", "robotics-fan", "User", false},
		{"copilot substring does not mark bot", "copilot-trainer", "User", false},
		{"dependabot substring does not mark bot", "dependabot-helper", "User", false},
		{"github-actions substring does not mark bot", "github-actions-user", "User", false},
		{"codecov-commenter substring does not mark bot", "codecov-commenter-fan", "User", false},
		{"empty type and no bot marker", "some-human-login", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBotAccount(tt.login, tt.accountType)
			assert.Equal(t, tt.want, got)
		})
	}
}
