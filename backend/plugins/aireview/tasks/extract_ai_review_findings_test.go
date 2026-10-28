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
	"strings"
	"testing"

	"github.com/apache/devlake/core/errors"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	mocklog "github.com/apache/devlake/mocks/core/log"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNormalizeBody(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"strips JSON quotes", `"hello world"`, "hello world"},
		{"converts escaped newlines", `line1\nline2`, "line1\nline2"},
		{"removes escaped carriage returns", `line1\r\nline2`, "line1\nline2"},
		{"converts escaped quotes", `say \"hello\"`, `say "hello"`},
		{"empty string", "", ""},
		{"single quote char", `"`, `"`},
		{"combined escaping", `"first\\nsecond\nthird"`, "first\\\nsecond\nthird"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeBody(tt.input))
		})
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"short text returned as-is", "fix bug", "fix bug"},
		{"first sentence extracted", "Fix the bug. Then refactor.", "Fix the bug"},
		{"exclamation as sentence end", "Security issue found! Check now.", "Security issue found"},
		{"question mark as sentence end", "Is this correct? Let me check.", "Is this correct"},
		{"long text truncated with ellipsis",
			"This is a very long description that goes well beyond the eighty character limit and should be truncated properly",
			"This is a very long description that goes well beyond the eighty character li..."},
		{"exactly 80 chars", strings.Repeat("a", 80), strings.Repeat("a", 80)},
		{"79 chars no truncation", strings.Repeat("a", 79), strings.Repeat("a", 79)},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, truncateTitle(tt.input))
		})
	}
}

func TestGenerateFindingId(t *testing.T) {
	id1 := generateFindingId("review-1", "file.go", 0)
	id2 := generateFindingId("review-1", "file.go", 0)
	id3 := generateFindingId("review-1", "file.go", 1)
	id4 := generateFindingId("review-2", "file.go", 0)

	assert.Equal(t, id1, id2, "same inputs must produce same ID")
	assert.NotEqual(t, id1, id3, "different index must produce different ID")
	assert.NotEqual(t, id1, id4, "different review must produce different ID")
	assert.True(t, strings.HasPrefix(id1, "aifinding:"), "must have aifinding: prefix")
	assert.NotEmpty(t, id1)
}

func TestDetectFindingCategory(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"security keyword", "This has a SQL injection vulnerability", models.FindingCategorySecurity},
		{"password keyword", "Hardcoded password detected", models.FindingCategorySecurity},
		{"performance keyword", "This could cause a memory leak", models.FindingCategoryPerformance},
		{"optimization keyword", "Consider optimizing this query", models.FindingCategoryPerformance},
		{"bug keyword", "This will crash on nil input", models.FindingCategoryBug},
		{"error keyword", "Error handling is missing", models.FindingCategoryBug},
		{"style keyword", "Naming convention violation", models.FindingCategoryStyle},
		{"lint keyword", "Lint formatting issue", models.FindingCategoryStyle},
		{"documentation keyword", "Missing doc comment", models.FindingCategoryDocumentation},
		{"maintainability keyword", "Consider refactoring this method", models.FindingCategoryMaintainability},
		{"duplicate keyword", "Duplicate code detected", models.FindingCategoryMaintainability},
		{"default to best_practice", "Consider using a constant here", models.FindingCategoryBestPractice},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectFindingCategory(tt.input))
		})
	}
}

func TestDetectFindingSeverity(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"critical keyword", "Critical security vulnerability found", models.FindingSeverityCritical},
		{"data loss keyword", "This could cause data loss", models.FindingSeverityCritical},
		{"error keyword", "This is a bug that will fail", models.FindingSeverityError},
		{"must keyword", "Must handle this case", models.FindingSeverityError},
		{"warning keyword", "You should validate input", models.FindingSeverityWarning},
		{"consider keyword", "Consider adding a check", models.FindingSeverityWarning},
		{"default to info", "Nice formatting here", models.FindingSeverityInfo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectFindingSeverity(tt.input))
		})
	}
}

func TestDetectFindingType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"suggest keyword", "I suggest using a map here", models.FindingTypeSuggestion},
		{"consider keyword", "Consider refactoring this", models.FindingTypeSuggestion},
		{"issue keyword", "There is an issue with this logic", models.FindingTypeIssue},
		{"bug keyword", "This is a bug in the error handling", models.FindingTypeIssue},
		{"default to comment", "The code looks clean here", models.FindingTypeComment},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectFindingType(tt.input))
		})
	}
}

func TestDetectSuggestionApplied(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		want  bool
	}{
		{"applied suggestion marker", "The applied suggestion looks good", true},
		{"resolved marker", "✅ Resolved this issue", true},
		{"applied in commit", "suggestion applied in commit abc123", true},
		{"no marker", "This is a regular comment", false},
		{"empty body", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectSuggestionApplied(tt.body, 0))
		})
	}
}

func TestParseSuggestionBlocks(t *testing.T) {
	t.Run("single suggestion block", func(t *testing.T) {
		review := &models.AiReview{
			Id:            "r1",
			PullRequestId: "pr1",
			RepoId:        "repo1",
			AiTool:        models.AiToolCodeRabbit,
		}
		body := "Some text\n```suggestion\nfunc foo() {}\n```\nMore text"

		findings := parseSuggestionBlocks(review, body)

		assert.Len(t, findings, 1)
		assert.Equal(t, "func foo() {}", findings[0].SuggestedCode)
		assert.Equal(t, models.FindingTypeSuggestion, findings[0].Type)
		assert.Equal(t, "pr1", findings[0].PullRequestId)
		assert.Equal(t, "repo1", findings[0].RepoId)
	})

	t.Run("multiple suggestion blocks", func(t *testing.T) {
		review := &models.AiReview{Id: "r2", AiTool: models.AiToolQodo}
		body := "```suggestion\nline1\n```\ntext\n```suggestion\nline2\n```"

		findings := parseSuggestionBlocks(review, body)
		assert.Len(t, findings, 2)
		assert.Equal(t, "line1", findings[0].SuggestedCode)
		assert.Equal(t, "line2", findings[1].SuggestedCode)
	})

	t.Run("no suggestion blocks", func(t *testing.T) {
		review := &models.AiReview{Id: "r3", AiTool: models.AiToolGemini}
		findings := parseSuggestionBlocks(review, "Just regular text with ```code``` blocks")
		assert.Empty(t, findings)
	})
}

func TestParseCodeRabbitFindings(t *testing.T) {
	t.Run("file block with issues", func(t *testing.T) {
		review := &models.AiReview{
			Id:            "r1",
			PullRequestId: "pr1",
			RepoId:        "repo1",
			AiTool:        models.AiToolCodeRabbit,
		}
		body := "📁 src/main.go\n- Missing error handling in this function\n- Consider adding a nil check for safety\n"

		findings := parseCodeRabbitFindings(review, body)

		assert.Len(t, findings, 2)
		assert.Equal(t, "src/main.go", findings[0].FilePath)
		assert.Contains(t, findings[0].Description, "Missing error handling")
		assert.Contains(t, findings[1].Description, "nil check")
	})

	t.Run("File: prefix format", func(t *testing.T) {
		review := &models.AiReview{Id: "r2", AiTool: models.AiToolCodeRabbit}
		body := "File: pkg/utils.go\n* Unused import detected\n"

		findings := parseCodeRabbitFindings(review, body)
		assert.Len(t, findings, 1)
		assert.Equal(t, "pkg/utils.go", findings[0].FilePath)
	})

	t.Run("no file blocks", func(t *testing.T) {
		review := &models.AiReview{Id: "r3", AiTool: models.AiToolCodeRabbit}
		findings := parseCodeRabbitFindings(review, "Just a summary comment")
		assert.Empty(t, findings)
	})
}

func TestParseGenericFindings(t *testing.T) {
	t.Run("bullet points with file paths", func(t *testing.T) {
		review := &models.AiReview{
			Id:     "r1",
			AiTool: models.AiToolQodo,
		}
		body := "- The function in src/handler.go should validate input before processing\n- Consider adding tests for the edge case scenario here\n"

		findings := parseGenericFindings(review, body)

		assert.GreaterOrEqual(t, len(findings), 1)
		found := false
		for _, f := range findings {
			if f.FilePath == "src/handler.go" {
				found = true
			}
		}
		assert.True(t, found, "should extract file path from bullet")
	})

	t.Run("short bullets are skipped", func(t *testing.T) {
		review := &models.AiReview{Id: "r2", AiTool: models.AiToolGemini}
		body := "- Short\n- Also short\n"
		findings := parseGenericFindings(review, body)
		assert.Empty(t, findings)
	})

	t.Run("asterisk bullets", func(t *testing.T) {
		review := &models.AiReview{Id: "r3", AiTool: models.AiToolGemini}
		body := "* This is a long enough bullet point to be considered a real finding\n"
		findings := parseGenericFindings(review, body)
		assert.Len(t, findings, 1)
	})
}

func TestParseFindings(t *testing.T) {
	t.Run("CodeRabbit review dispatches to CodeRabbit parser", func(t *testing.T) {
		review := &models.AiReview{
			Id:     "r1",
			AiTool: models.AiToolCodeRabbit,
			Body:   "📁 main.go\n- Security issue with input validation in the handler function\n",
		}
		findings := parseFindings(review)
		assert.NotEmpty(t, findings)
	})

	t.Run("non-CodeRabbit review uses generic parser", func(t *testing.T) {
		review := &models.AiReview{
			Id:     "r2",
			AiTool: models.AiToolQodo,
			Body:   "- Consider refactoring this function for better maintainability\n",
		}
		findings := parseFindings(review)
		assert.NotEmpty(t, findings)
	})

	t.Run("suggestion blocks extracted from any tool", func(t *testing.T) {
		review := &models.AiReview{
			Id:     "r3",
			AiTool: models.AiToolGemini,
			Body:   "```suggestion\nreturn nil, fmt.Errorf(\"invalid: %w\", err)\n```",
		}
		findings := parseFindings(review)
		hasSuggestion := false
		for _, f := range findings {
			if f.Type == models.FindingTypeSuggestion {
				hasSuggestion = true
			}
		}
		assert.True(t, hasSuggestion, "should find suggestion block")
	})

	t.Run("empty body returns no findings", func(t *testing.T) {
		review := &models.AiReview{Id: "r4", AiTool: models.AiToolCodeRabbit, Body: ""}
		findings := parseFindings(review)
		assert.Empty(t, findings)
	})
}

func TestSaveFindingsBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

		batch := []*models.AiReviewFinding{
			{Id: "f1", Description: "finding 1"},
			{Id: "f2", Description: "finding 2"},
		}
		err := saveFindingsBatch(mockDal, batch)
		assert.Nil(t, err)
		mockDal.AssertNumberOfCalls(t, "CreateOrUpdate", 2)
	})

	t.Run("empty batch", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		err := saveFindingsBatch(mockDal, []*models.AiReviewFinding{})
		assert.Nil(t, err)
	})

	t.Run("error on save", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).
			Return(nil).Once()
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		batch := []*models.AiReviewFinding{
			{Id: "f1"},
			{Id: "f2"},
		}
		err := saveFindingsBatch(mockDal, batch)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

func TestExtractAiReviewFindings(t *testing.T) {
	t.Run("processes one review with findings", func(t *testing.T) {
		mockCtx := new(mockplugin.SubTaskContext)
		mockDl := new(mockdal.Dal)
		mockLogger := new(mocklog.Logger)
		mockRows := new(mockdal.Rows)

		data := &AiReviewTaskData{
			Options: &AiReviewOptions{RepoId: "repo-1"},
		}

		mockCtx.On("GetData").Return(data)
		mockCtx.On("GetDal").Return(mockDl)
		mockCtx.On("GetLogger").Return(mockLogger)
		mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()

		mockDl.On("Cursor", mock.Anything).Return(mockRows, nil)
		mockRows.On("Next").Return(true).Once()
		mockRows.On("Next").Return(false)
		mockRows.On("Close").Return(nil)

		mockDl.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(1).(*models.AiReview)
			*dst = models.AiReview{
				Id:     "review-1",
				RepoId: "repo-1",
				AiTool: "CodeRabbit",
				Body:   "- This has a security vulnerability in the auth handler that needs fixing\n",
			}
		}).Return(nil)

		mockDl.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

		err := ExtractAiReviewFindings(mockCtx)
		assert.Nil(t, err)
	})
}
