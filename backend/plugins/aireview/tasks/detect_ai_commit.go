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

	"github.com/apache/devlake/plugins/aireview/models"
)

// trailerSuffixLen is how many bytes of the commit message to inspect.
// Co-Authored-By / Assisted-by / Made-with trailers live at the end.
const trailerSuffixLen = 2048

var (
	cursorTrailerRe     = regexp.MustCompile(`(?i)(?:co-authored-by:|assisted-by:|made-with:).*(?:cursor(?:\.com)?)`)
	claudeTrailerRe     = regexp.MustCompile(`(?i)(?:co-authored-by:|assisted-by:|made-with:).*(?:claude|anthropic\.com)`)
	copilotTrailerRe    = regexp.MustCompile(`(?i)(?:co-authored-by:|assisted-by:|made-with:).*copilot`)
	codeRabbitTrailerRe = regexp.MustCompile(`(?i)(?:co-authored-by:|assisted-by:|made-with:).*coderabbit`)
	codeRabbitAuthorRe  = regexp.MustCompile(`(?i)coderabbit`)
	assistedByRe        = regexp.MustCompile(`(?i)assisted-by:`)
	madeWithRe          = regexp.MustCompile(`(?i)made-with:`)
)

// commitTrailer returns the suffix of message that typically contains git trailers.
func commitTrailer(message string) string {
	if len(message) <= trailerSuffixLen {
		return message
	}
	return message[len(message)-trailerSuffixLen:]
}

// detectAiCommit reports whether a commit is AI-assisted and which tool authored it.
func detectAiCommit(message, authorName string, extraPattern *regexp.Regexp) (string, bool) {
	trailer := commitTrailer(message)

	if cursorTrailerRe.MatchString(trailer) {
		return models.AiToolCursor, true
	}
	if claudeTrailerRe.MatchString(trailer) {
		return models.AiToolClaude, true
	}
	if copilotTrailerRe.MatchString(trailer) {
		return models.AiToolCopilot, true
	}
	if codeRabbitTrailerRe.MatchString(trailer) || codeRabbitAuthorRe.MatchString(authorName) {
		return models.AiToolCodeRabbit, true
	}
	if assistedByRe.MatchString(trailer) {
		return models.AiToolAssistedByUnknown, true
	}
	if madeWithRe.MatchString(trailer) {
		return models.AiToolMadeWithUnknown, true
	}
	if extraPattern != nil && extraPattern.MatchString(trailer) {
		return models.AiToolOther, true
	}
	return "", false
}
