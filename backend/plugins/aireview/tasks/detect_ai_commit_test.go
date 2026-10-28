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
	"strings"
	"testing"

	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
)

func TestDetectAiCommit(t *testing.T) {
	extra := regexp.MustCompile(`(?i)(?:co-authored-by|assisted-by|made-with):.*(?:\bai\b|chatgpt|openai)`)

	tests := []struct {
		name       string
		message    string
		authorName string
		extra      *regexp.Regexp
		wantTool   string
		wantAI     bool
	}{
		{
			name:     "Co-Authored-By Cursor",
			message:  "Fix nil check\n\nCo-Authored-By: Cursor <cursoragent@cursor.com>",
			wantTool: models.AiToolCursor,
			wantAI:   true,
		},
		{
			name:     "co-authored-by cursor.com case variant",
			message:  "refactor\n\nco-authored-by: Cursor Agent <bot@cursor.com>",
			wantTool: models.AiToolCursor,
			wantAI:   true,
		},
		{
			name:     "Assisted-by Cursor",
			message:  "update config\n\nAssisted-by: Cursor",
			wantTool: models.AiToolCursor,
			wantAI:   true,
		},
		{
			name:     "Made-with Cursor",
			message:  "docs\n\nMade-with: Cursor",
			wantTool: models.AiToolCursor,
			wantAI:   true,
		},
		{
			name:     "Co-Authored-By Claude",
			message:  "Add tests\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
			wantTool: models.AiToolClaude,
			wantAI:   true,
		},
		{
			name:     "anthropic.com trailer",
			message:  "chore\n\nCo-authored-by: Claude <bot@anthropic.com>",
			wantTool: models.AiToolClaude,
			wantAI:   true,
		},
		{
			name:     "Assisted-by Claude",
			message:  "feat\n\nAssisted-by: Claude",
			wantTool: models.AiToolClaude,
			wantAI:   true,
		},
		{
			name:     "Co-Authored-By Copilot",
			message:  "lint\n\nCo-Authored-By: Copilot <copilot@github.com>",
			wantTool: models.AiToolCopilot,
			wantAI:   true,
		},
		{
			name:     "Co-Authored-By CodeRabbit",
			message:  "Apply suggestion\n\nCo-authored-by: coderabbitai[bot] <noreply@github.com>",
			wantTool: models.AiToolCodeRabbit,
			wantAI:   true,
		},
		{
			name:       "CodeRabbit author without trailer",
			message:    "Apply suggestions from code review",
			authorName: "coderabbitai[bot]",
			wantTool:   models.AiToolCodeRabbit,
			wantAI:     true,
		},
		{
			name:       "CodeRabbit author mixed case",
			message:    "Apply suggestions from code review",
			authorName: "CodeRabbitAI[bot]",
			wantTool:   models.AiToolCodeRabbit,
			wantAI:     true,
		},
		{
			name:     "Cursor takes precedence over Claude in the same message",
			message:  "both\n\nCo-Authored-By: Cursor <cursoragent@cursor.com>\nCo-Authored-By: Claude <noreply@anthropic.com>",
			wantTool: models.AiToolCursor,
			wantAI:   true,
		},
		{
			name:     "unknown Assisted-by",
			message:  "tweak\n\nAssisted-by: SomeOtherTool",
			wantTool: models.AiToolAssistedByUnknown,
			wantAI:   true,
		},
		{
			name:     "unknown Made-with",
			message:  "tweak\n\nMade-with: SomeOtherTool",
			wantTool: models.AiToolMadeWithUnknown,
			wantAI:   true,
		},
		{
			name:     "extra pattern chatgpt trailer",
			message:  "wip\n\nCo-Authored-By: ChatGPT <noreply@openai.com>",
			extra:    extra,
			wantTool: models.AiToolOther,
			wantAI:   true,
		},
		{
			name:     "human commit mentioning Claude in the body is not AI",
			message:  "Document how we evaluated Claude vs Copilot for reviews",
			wantTool: "",
			wantAI:   false,
		},
		{
			name:       "empty message human author",
			message:    "",
			authorName: "Jane Doe",
			wantTool:   "",
			wantAI:     false,
		},
		{
			name:     "trailer after a long body is still detected",
			message:  strings.Repeat("x", 3000) + "\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
			wantTool: models.AiToolClaude,
			wantAI:   true,
		},
		{
			name:     "nil extra pattern does not match generic AI",
			message:  "wip\n\nCo-Authored-By: ChatGPT <noreply@openai.com>",
			extra:    nil,
			wantTool: "",
			wantAI:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTool, gotAI := detectAiCommit(tt.message, tt.authorName, tt.extra)
			assert.Equal(t, tt.wantTool, gotTool)
			assert.Equal(t, tt.wantAI, gotAI)
		})
	}
}

func TestCommitTrailer(t *testing.T) {
	assert.Equal(t, "short", commitTrailer("short"))
	long := strings.Repeat("a", trailerSuffixLen+10)
	assert.Equal(t, trailerSuffixLen, len(commitTrailer(long)))
}
