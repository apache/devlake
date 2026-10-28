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
	"regexp"
	"testing"

	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
)

func TestDetectAiTool_CodeRabbit(t *testing.T) {
	tests := []struct {
		name       string
		accountId  string
		body       string
		wantTool   string
		wantIsAi   bool
	}{
		{
			name:      "CodeRabbit by username",
			accountId: "coderabbitai",
			body:      "Some review comment",
			wantTool:  models.AiToolCodeRabbit,
			wantIsAi:  true,
		},
		{
			name:      "CodeRabbit by pattern in body",
			accountId: "someuser",
			body:      "## Summary by CodeRabbit\n\nThis PR adds...",
			wantTool:  models.AiToolCodeRabbit,
			wantIsAi:  true,
		},
		{
			name:      "CodeRabbit walkthrough pattern",
			accountId: "bot",
			body:      "## Walkthrough\nThe changes introduce...",
			wantTool:  models.AiToolCodeRabbit,
			wantIsAi:  true,
		},
		{
			name:      "Not an AI review",
			accountId: "developer",
			body:      "LGTM, looks good to me!",
			wantTool:  "",
			wantIsAi:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskData := &AiReviewTaskData{
				Options: &AiReviewOptions{
					ScopeConfig: models.GetDefaultScopeConfig(),
				},
			}
			// Compile patterns
			err := CompilePatterns(taskData)
			assert.NoError(t, err)

			gotTool, gotIsAi := detectAiTool(taskData, tt.accountId, tt.body)
			assert.Equal(t, tt.wantTool, gotTool)
			assert.Equal(t, tt.wantIsAi, gotIsAi)
		})
	}
}

func TestParseReviewMetrics(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		wantComplexity   string
		wantEffort       int
		wantIssuesMin    int
		wantSuggestions  int
	}{
		{
			name:            "Simple review with time estimate",
			body:            "This is a simple change that takes ~12 minutes to review.",
			wantComplexity:  "simple",
			wantEffort:      12,
			wantIssuesMin:   0,
			wantSuggestions: 0,
		},
		{
			name:            "Complex review with issues",
			body:            "This is a complex change. Found a bug in the auth logic. Also there's an error in validation.",
			wantComplexity:  "complex",
			wantEffort:      30,
			wantIssuesMin:   2,
			wantSuggestions: 0,
		},
		{
			name:            "Review with suggestions",
			body:            "I suggest refactoring this. You should consider using a map. I would recommend adding tests.",
			wantComplexity:  "",
			wantEffort:      0,
			wantIssuesMin:   0,
			wantSuggestions: 3,
		},
		{
			name:            "Review with line changes",
			body:            "Changes: +50 −36 lines modified",
			wantComplexity:  "",
			wantEffort:      0,
			wantIssuesMin:   0,
			wantSuggestions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := parseReviewMetrics(tt.body)

			if tt.wantComplexity != "" {
				assert.Equal(t, tt.wantComplexity, metrics.Complexity)
			}
			if tt.wantEffort > 0 {
				assert.Equal(t, tt.wantEffort, metrics.EffortMinutes)
			}
			assert.GreaterOrEqual(t, metrics.IssuesFound, tt.wantIssuesMin)
			assert.GreaterOrEqual(t, metrics.SuggestionsCount, tt.wantSuggestions)
		})
	}
}

func TestDetectRiskLevel(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantLevel string
		wantScore int
	}{
		{
			name:      "High risk - security",
			body:      "CRITICAL: This introduces a security vulnerability",
			wantLevel: models.RiskLevelHigh,
			wantScore: 80,
		},
		{
			name:      "Medium risk - warning",
			body:      "Warning: This change may have moderate impact",
			wantLevel: models.RiskLevelMedium,
			wantScore: 50,
		},
		{
			name:      "Low risk - minor",
			body:      "Minor suggestion: consider renaming this variable",
			wantLevel: models.RiskLevelLow,
			wantScore: 20,
		},
		{
			name:      "Default to low",
			body:      "Looks good, no issues found",
			wantLevel: models.RiskLevelLow,
			wantScore: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskData := &AiReviewTaskData{
				Options: &AiReviewOptions{
					ScopeConfig: models.GetDefaultScopeConfig(),
				},
			}
			err := CompilePatterns(taskData)
			assert.NoError(t, err)

			gotLevel, gotScore := detectRiskLevel(taskData, tt.body)
			assert.Equal(t, tt.wantLevel, gotLevel)
			assert.Equal(t, tt.wantScore, gotScore)
		})
	}
}

func TestDetectReviewState(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		status    string
		wantState string
	}{
		{
			name:      "Approved in body",
			body:      "LGTM, approved!",
			status:    "",
			wantState: models.ReviewStateApproved,
		},
		{
			name:      "Changes requested",
			body:      "Please make changes requested above",
			status:    "",
			wantState: models.ReviewStateChangesRequested,
		},
		{
			name:      "Approved via status",
			body:      "Some comment",
			status:    "APPROVED",
			wantState: models.ReviewStateApproved,
		},
		{
			name:      "Default to commented",
			body:      "Here are some thoughts",
			status:    "",
			wantState: models.ReviewStateCommented,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState := detectReviewState(tt.body, tt.status)
			assert.Equal(t, tt.wantState, gotState)
		})
	}
}

func TestExtractSummary(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantContain string
		wantNotContain string
	}{
		{
			name:        "Extract summary section",
			body:        "## Summary\nThis PR adds a new feature.\n\nDetails follow...",
			wantContain: "This PR adds a new feature",
		},
		{
			name:        "Extract walkthrough section",
			body:        "## Walkthrough\nThe changes introduce new handlers.\n\n## Details",
			wantContain: "changes introduce new handlers",
		},
		{
			name:        "Truncate long body without summary",
			body:        "This is a very long comment that goes on and on with more content.",
			wantContain: "This is a very long comment",
		},
		{
			name:        "Clean HTML tags",
			body:        "<table><tr><td>Some content here that is long enough to be extracted as a summary paragraph</td></tr></table>",
			wantContain: "Some content here",
			wantNotContain: "<table>",
		},
		{
			name:        "Preserve markdown quote blocks",
			body:        "> This is a quote\n>Another line\n\nActual content here is important",
			wantContain: "> This is a quote",
		},
		{
			name:        "Handle escaped newlines",
			body:        "First line\\nSecond line\\nThird line with content",
			wantContain: "Second line",
		},
		{
			name:        "Extract Qodo focus areas",
			body:        "## PR Reviewer Guide\n\n**Estimated effort to review**: 3\n\n**Recommended focus areas for review**:\n\n**Over-broad replace** - The sed replacements may break production.",
			wantContain: "Over-broad replace",
		},
		{
			name:        "Convert details/summary to markdown",
			body:        "Main content<details><summary>Hidden</summary>Secret stuff</details>More content",
			wantContain: "**Hidden**",
			wantNotContain: "<details>",
		},
		{
			name:        "Preserve markdown links",
			body:        "Check [this link](https://example.com) for details.",
			wantContain: "[this link]",
		},
		{
			name:        "Handle real Qodo format",
			body:        "## PR Reviewer Guide\n\n<table>\n<tr><td>\n\n**Ticket compliance**\n\n</td></tr>\n<tr><td><strong>Estimated effort to review</strong>: 2</td></tr>\n</table>",
			wantContain: "Effort: 2/5",
			wantNotContain: "<table>",
		},
		{
			name:        "Gemini PR-level summary extracts content after greeting",
			body:        "## Summary of Changes\n\nHello @user, I'm Gemini Code Assist[^1]! Here is my review.\n\nThis pull request refactors the authentication module to use JWT tokens instead of session cookies, improving scalability.\n\n### Highlights\n\n* **Token rotation**: Added automatic token refresh logic\n* **Session cleanup**: Removed legacy session store",
			wantContain: "This pull request refactors the authentication module",
			wantNotContain: "of Changes",
		},
		{
			name:           "Gemini inline review strips priority badge",
			body:           "![medium](https://www.gstatic.com/codereviewagent/medium-priority.svg)\n\nThe error returned by `db.Save()` is not checked here, which could silently drop write failures.",
			wantContain:    "error returned by",
			wantNotContain: "gstatic.com",
		},
		{
			name:           "Gemini inline review with JSON-quoted body",
			body:           "\"![medium](https://www.gstatic.com/codereviewagent/medium-priority.svg)\\n\\nThis test uses the `eth4` interface, which is not created in the test setup.\"",
			wantContain:    "This test uses the",
			wantNotContain: "gstatic.com",
		},
		{
			name:           "Gemini inline review with multiple badges",
			body:           "\"![security-high](https://www.gstatic.com/codereviewagent/security-high-priority.svg) ![high](https://www.gstatic.com/codereviewagent/high-priority.svg)\\n\\nA command injection vulnerability exists here.\"",
			wantContain:    "command injection vulnerability",
			wantNotContain: "gstatic.com",
		},
		{
			name:        "Gemini PR summary extracts highlights",
			body:        "## Summary of Changes\n\nHello @dev, I'm Gemini Code Assist[^1]!\n\nThis PR adds retry logic for failed API calls to improve resilience.\n\n### Highlights\n\n* **Exponential backoff**: Implements configurable retry delays\n* **Circuit breaker**: Prevents cascading failures",
			wantContain: "Exponential backoff",
		},
		{
			name:           "Generic summary regex does not match Gemini heading",
			body:           "## Summary of Changes\n\nSome unrelated content that is long enough to be a paragraph for extraction purposes.",
			wantContain:    "Some unrelated content",
			wantNotContain: "of Changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSummary(tt.body)
			assert.Contains(t, got, tt.wantContain)
			if tt.wantNotContain != "" {
				assert.NotContains(t, got, tt.wantNotContain)
			}
		})
	}
}

func TestHtmlToMarkdown(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContain    string
		wantNotContain string
	}{
		{
			name:           "Convert strong to bold markdown",
			input:          "<strong>Bold</strong> text",
			wantContain:    "**Bold**",
			wantNotContain: "<strong>",
		},
		{
			name:        "Handle escaped newlines",
			input:       "Line1\\nLine2\\nLine3",
			wantContain: "Line1\nLine2\nLine3",
		},
		{
			name:           "Convert details/summary to markdown",
			input:          "Before<details><summary>Title</summary>Hidden content</details>After",
			wantContain:    "**Title**",
			wantNotContain: "<details>",
		},
		{
			name:        "Preserve markdown quote blocks",
			input:       "> Quoted text\n> More quoted",
			wantContain: "> Quoted text",
		},
		{
			name:        "Preserve bold markers",
			input:       "This is **bold** and __also bold__",
			wantContain: "**bold**",
		},
		{
			name:        "Preserve markdown headers",
			input:       "## This is a header\n\nParagraph text",
			wantContain: "## This is a header",
		},
		{
			name:        "Preserve markdown links",
			input:       "See [documentation](https://docs.example.com) here",
			wantContain: "[documentation](https://docs.example.com)",
		},
		{
			name:        "Convert HTML links to markdown",
			input:       "See <a href='https://docs.example.com'>documentation</a> here",
			wantContain: "[documentation](https://docs.example.com)",
		},
		{
			name:           "Remove HTML tables but keep content",
			input:          "<table><tr><td>Cell content</td></tr></table>",
			wantContain:    "Cell content",
			wantNotContain: "<table>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := htmlToMarkdown(tt.input)
			assert.Contains(t, got, tt.wantContain)
			if tt.wantNotContain != "" {
				assert.NotContains(t, got, tt.wantNotContain)
			}
		})
	}
}

func TestGenerateReviewId(t *testing.T) {
	// Test deterministic ID generation
	id1 := generateReviewId("pr-123", "comment-456", "coderabbit")
	id2 := generateReviewId("pr-123", "comment-456", "coderabbit")
	id3 := generateReviewId("pr-123", "comment-789", "coderabbit")

	assert.Equal(t, id1, id2, "Same inputs should produce same ID")
	assert.NotEqual(t, id1, id3, "Different inputs should produce different IDs")
	assert.True(t, len(id1) > 0, "ID should not be empty")
	assert.Contains(t, id1, "aireview:", "ID should have correct prefix")
}

func TestDetectSourcePlatform(t *testing.T) {
	tests := []struct {
		name     string
		prId     string
		wantPlat string
	}{
		{
			name:     "GitHub PR",
			prId:     "github:GithubPullRequest:1:12345",
			wantPlat: "github",
		},
		{
			name:     "GitLab MR",
			prId:     "gitlab:GitlabMergeRequest:1:67890",
			wantPlat: "gitlab",
		},
		{
			name:     "Unknown platform",
			prId:     "bitbucket:PullRequest:1:11111",
			wantPlat: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectSourcePlatform(tt.prId)
			assert.Equal(t, tt.wantPlat, got)
		})
	}
}

func TestCompilePatterns(t *testing.T) {
	t.Run("Default config compiles successfully", func(t *testing.T) {
		taskData := &AiReviewTaskData{
			Options: &AiReviewOptions{
				ScopeConfig: models.GetDefaultScopeConfig(),
			},
		}
		err := CompilePatterns(taskData)
		assert.NoError(t, err)
		assert.NotNil(t, taskData.CodeRabbitUsernameRegex)
		assert.NotNil(t, taskData.CodeRabbitPatternRegex)
		assert.NotNil(t, taskData.RiskHighPatternRegex)
	})

	t.Run("Invalid regex returns error", func(t *testing.T) {
		taskData := &AiReviewTaskData{
			Options: &AiReviewOptions{
				ScopeConfig: &models.AiReviewScopeConfig{
					CodeRabbitEnabled: true,
					CodeRabbitPattern: "[invalid(regex",
				},
			},
		}
		err := CompilePatterns(taskData)
		assert.Error(t, err)
	})

	t.Run("Nil config uses defaults", func(t *testing.T) {
		taskData := &AiReviewTaskData{
			Options: &AiReviewOptions{
				ScopeConfig: nil,
			},
		}
		err := CompilePatterns(taskData)
		assert.NoError(t, err)
		assert.NotNil(t, taskData.Options.ScopeConfig)
	})
}

func TestCodeRabbitPatternMatching(t *testing.T) {
	pattern := regexp.MustCompile(`(?i)(coderabbit|walkthrough|summary by coderabbit)`)

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"CodeRabbit mention", "This is a CodeRabbit review", true},
		{"Walkthrough section", "## Walkthrough", true},
		{"Summary by CodeRabbit", "Summary by CodeRabbit:", true},
		{"Case insensitive", "CODERABBIT", true},
		{"No match", "Regular comment", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pattern.MatchString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectAiTool_Qodo(t *testing.T) {
	tests := []struct {
		name      string
		accountId string
		body      string
		wantTool  string
		wantIsAi  bool
	}{
		{
			name:      "Qodo by username",
			accountId: "qodo-merge",
			body:      "Some review comment",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Qodo by username with prefix",
			accountId: "rhdh-qodo-merge",
			body:      "Some review comment",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Qodo by PR Reviewer Guide pattern",
			accountId: "somebot",
			body:      "## PR Reviewer Guide 🔍\n\n⏱️ Estimated effort to review: 3 🔵🔵🔵⚪⚪",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Qodo by effort estimate pattern",
			accountId: "bot",
			body:      "⏱️ Estimated effort to review: 2 🔵🔵⚪⚪⚪",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Qodo mentioned in body",
			accountId: "someuser",
			body:      "This is a Qodo review with security findings",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Not Qodo - regular comment",
			accountId: "developer",
			body:      "LGTM, looks good!",
			wantTool:  "",
			wantIsAi:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskData := &AiReviewTaskData{
				Options: &AiReviewOptions{
					ScopeConfig: models.GetDefaultScopeConfig(),
				},
			}
			// Compile patterns
			err := CompilePatterns(taskData)
			assert.NoError(t, err)

			gotTool, gotIsAi := detectAiTool(taskData, tt.accountId, tt.body)
			assert.Equal(t, tt.wantTool, gotTool)
			assert.Equal(t, tt.wantIsAi, gotIsAi)
		})
	}
}

func TestDetectAiTool_Gemini(t *testing.T) {
	tests := []struct {
		name      string
		accountId string
		body      string
		wantTool  string
		wantIsAi  bool
	}{
		{
			name:      "Gemini by GitHub username",
			accountId: "gemini-code-assist",
			body:      "Some review comment",
			wantTool:  models.AiToolGemini,
			wantIsAi:  true,
		},
		{
			name:      "Gemini by GitLab username",
			accountId: "gemini-code-assist-bot",
			body:      "Some review comment with code assist feedback",
			wantTool:  models.AiToolGemini,
			wantIsAi:  true,
		},
		{
			name:      "Gemini by body pattern - codereviewagent",
			accountId: "somebot",
			body:      "Review by codereviewagent: this looks good",
			wantTool:  models.AiToolGemini,
			wantIsAi:  true,
		},
		{
			name:      "Gemini by body pattern - gstatic reference",
			accountId: "somebot",
			body:      "![icon](https://www.gstatic.com/codereviewagent/icon.png) Review summary",
			wantTool:  models.AiToolGemini,
			wantIsAi:  true,
		},
		{
			name:      "Gemini by body pattern - greeting phrase",
			accountId: "bot",
			body:      "Hello @user, I'm Gemini Code Assist! Here is my review of this pull request.",
			wantTool:  models.AiToolGemini,
			wantIsAi:  true,
		},
		{
			name:      "Not Gemini - regular comment",
			accountId: "developer",
			body:      "LGTM, looks good!",
			wantTool:  "",
			wantIsAi:  false,
		},
		{
			name:      "Not Gemini - human mentioning Gemini Code Assist",
			accountId: "developer",
			body:      "Gemini Code Assist has reviewed this merge request and I agree with its findings.",
			wantTool:  "",
			wantIsAi:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskData := &AiReviewTaskData{
				Options: &AiReviewOptions{
					ScopeConfig: models.GetDefaultScopeConfig(),
				},
			}
			err := CompilePatterns(taskData)
			assert.NoError(t, err)

			gotTool, gotIsAi := detectAiTool(taskData, tt.accountId, tt.body)
			assert.Equal(t, tt.wantTool, gotTool)
			assert.Equal(t, tt.wantIsAi, gotIsAi)
		})
	}
}

func TestBuildCommentUrl(t *testing.T) {
	tests := []struct {
		name      string
		prUrl     string
		commentId string
		want      string
	}{
		{
			name:      "GitHub comment",
			prUrl:     "https://github.com/owner/repo/pull/123",
			commentId: "github:GithubPrComment:1:456789",
			want:      "https://github.com/owner/repo/pull/123#issuecomment-456789",
		},
		{
			name:      "GitLab comment",
			prUrl:     "https://gitlab.com/owner/repo/-/merge_requests/123",
			commentId: "gitlab:GitlabMrComment:1:456789",
			want:      "https://gitlab.com/owner/repo/-/merge_requests/123#note_456789",
		},
		{
			name:      "Empty PR URL",
			prUrl:     "",
			commentId: "github:GithubPrComment:1:456789",
			want:      "",
		},
		{
			name:      "Malformed comment ID",
			prUrl:     "https://github.com/owner/repo/pull/123",
			commentId: "invalid",
			want:      "https://github.com/owner/repo/pull/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCommentUrl(tt.prUrl, tt.commentId)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestQodoPatternMatching(t *testing.T) {
	pattern := regexp.MustCompile(`(?i)(qodo|pr reviewer guide|estimated effort to review)`)

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Qodo mention", "This is a Qodo review", true},
		{"PR Reviewer Guide header", "## PR Reviewer Guide 🔍", true},
		{"Effort estimate", "⏱️ Estimated effort to review: 3", true},
		{"Case insensitive", "QODO", true},
		{"No match", "Regular comment", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pattern.MatchString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseReviewMetrics_EffortRating(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantRating     int
		wantComplexity string
	}{
		{
			name:           "CodeRabbit effort rating with complexity",
			body:           "🎯 3 (Moderate) | ⏱️ ~20 minutes",
			wantRating:     3,
			wantComplexity: "moderate",
		},
		{
			name:           "CodeRabbit effort rating only",
			body:           "Review effort: 🎯 4",
			wantRating:     4,
			wantComplexity: "complex", // inferred from rating >= 4
		},
		{
			name:           "CodeRabbit simple rating",
			body:           "🎯 1 (Simple)",
			wantRating:     1,
			wantComplexity: "simple",
		},
		{
			name:           "Qodo effort estimate format",
			body:           "⏱️ Estimated effort to review: 3 🔵🔵🔵⚪⚪",
			wantRating:     3,
			wantComplexity: "moderate", // inferred from rating 3
		},
		{
			name:           "Qodo table format",
			body:           "<tr><td><strong>Estimated effort to review</strong>: 2</td></tr>",
			wantRating:     2,
			wantComplexity: "simple", // inferred from rating <= 2
		},
		{
			name:           "No effort rating",
			body:           "This is a regular review comment",
			wantRating:     0,
			wantComplexity: "",
		},
		{
			name:           "Effort rating 5 (complex)",
			body:           "🎯 5 (Very Complex)",
			wantRating:     5,
			wantComplexity: "complex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := parseReviewMetrics(tt.body)
			assert.Equal(t, tt.wantRating, metrics.EffortRating)
			if tt.wantComplexity != "" {
				assert.Equal(t, tt.wantComplexity, metrics.Complexity)
			}
		})
	}
}

func TestParseReviewMetrics_PreMergeChecks(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		wantPassed       int
		wantFailed       int
		wantInconclusive int
	}{
		{
			name:             "CodeRabbit pre-merge checks all present",
			body:             "Pre-merge checks: 2 passed, 1 failed, 1 inconclusive",
			wantPassed:       2,
			wantFailed:       1,
			wantInconclusive: 1,
		},
		{
			name:             "CodeRabbit with emojis",
			body:             "✅ 3 checks passed | ❌ 1 check failed",
			wantPassed:       3,
			wantFailed:       1,
			wantInconclusive: 0,
		},
		{
			name:             "Only passed checks",
			body:             "All checks passed: 5 passed",
			wantPassed:       5,
			wantFailed:       0,
			wantInconclusive: 0,
		},
		{
			name:             "Inconclusive only",
			body:             "Status: 2 inconclusive",
			wantPassed:       0,
			wantFailed:       0,
			wantInconclusive: 2,
		},
		{
			name:             "No check information",
			body:             "This is a regular review",
			wantPassed:       0,
			wantFailed:       0,
			wantInconclusive: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := parseReviewMetrics(tt.body)
			assert.Equal(t, tt.wantPassed, metrics.PreMergeChecksPassed)
			assert.Equal(t, tt.wantFailed, metrics.PreMergeChecksFailed)
			assert.Equal(t, tt.wantInconclusive, metrics.PreMergeChecksInconclusive)
		})
	}
}

func TestParseSuggestionAcceptance(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantAccepted int
	}{
		{
			name:         "No acceptance markers",
			body:         "Summary by CodeRabbit. Simple review with no suggestions applied.",
			wantAccepted: 0,
		},
		{
			name: "CodeRabbit resolved markers in tracking table",
			body: `| Thread | File | Action | Status |
|--------|------|--------|--------|
| 1 | api/handler.go:15 | Committable | ✅ Resolved |
| 2 | api/handler.go:42 | Committable | ✅ Resolved |
| 3 | api/middleware.go:8 | Suggestion | Pending |`,
			wantAccepted: 2,
		},
		{
			name:         "Explicit suggestions applied summary",
			body:         "3 suggestions applied out of 5 total.",
			wantAccepted: 3,
		},
		{
			name: "CodeRabbit resolved markers plus summary",
			body: `Summary by CodeRabbit.
| 1 | handler.go | ✅ Resolved |
| 2 | middleware.go | ✅ Resolved |
2 suggestions applied.`,
			wantAccepted: 4,
		},
		{
			name: "Qodo checked checkboxes mixed with unchecked",
			body: `**Suggestions**
- [x] **Use constants for config keys** - Replace magic strings
- [ ] **Add validation** - Validate port range
- [x] **Extract defaults** - Move defaults to a separate block
- [ ] **Add docs** - Document config options`,
			wantAccepted: 2,
		},
		{
			name: "All checkboxes checked - likely self-review, not suggestions",
			body: `**Self-review checklist**
- [x] I reviewed the code changes
- [x] I ran the tests
- [x] I updated the docs`,
			wantAccepted: 0,
		},
		{
			name:         "Suggestion committed in specific commit",
			body:         "Suggestion was applied in commit abc1234.",
			wantAccepted: 1,
		},
		{
			name:         "Applied suggestion marker",
			body:         "Applied suggestion: use constants instead of magic strings.",
			wantAccepted: 1,
		},
		{
			name:         "Suggestion accepted (alternate wording)",
			body:         "1 suggestion accepted by the developer.",
			wantAccepted: 1,
		},
		{
			name:         "Multiple suggestions implemented",
			body:         "5 suggestions implemented in the latest push.",
			wantAccepted: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := ReviewMetrics{}
			parseSuggestionAcceptance(tt.body, &metrics)
			assert.Equal(t, tt.wantAccepted, metrics.SuggestionsAccepted)
		})
	}
}

func TestParseReviewMetrics_CodeRabbitRealWorld(t *testing.T) {
	// Test with realistic CodeRabbit output format
	realCodeRabbitBody := `## Summary by CodeRabbit

🎯 3 (Moderate) | ⏱️ ~20 minutes

**Pre-merge checks**: 2 passed, 1 inconclusive

### Walkthrough
The changes introduce new handlers for authentication.

### Changes
- Added JWT validation
- Updated middleware configuration
`

	metrics := parseReviewMetrics(realCodeRabbitBody)

	assert.Equal(t, 3, metrics.EffortRating)
	assert.Equal(t, 20, metrics.EffortMinutes)
	assert.Equal(t, "moderate", metrics.Complexity)
	assert.Equal(t, 2, metrics.PreMergeChecksPassed)
	assert.Equal(t, 1, metrics.PreMergeChecksInconclusive)
}
