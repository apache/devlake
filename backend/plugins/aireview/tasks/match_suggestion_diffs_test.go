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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/apache/devlake/core/errors"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIsApplySuggestionCommit(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		aiToolUser string
		want       bool
	}{
		{
			name:       "GitHub Apply suggestion button",
			message:    "Apply suggestions from code review\n\nCo-authored-by: coderabbitai[bot] <136622811+coderabbitai[bot]@users.noreply.github.com>",
			aiToolUser: "coderabbitai[bot]",
			want:       true,
		},
		{
			name:       "Short Apply suggestion message",
			message:    "Apply suggestion",
			aiToolUser: "",
			want:       true,
		},
		{
			name:       "Co-authored-by CodeRabbit",
			message:    "Fix nil check\n\nCo-authored-by: coderabbitai[bot] <noreply@github.com>",
			aiToolUser: "",
			want:       true,
		},
		{
			name:       "Co-authored-by Qodo",
			message:    "Add validation\n\nCo-authored-by: qodo-merge-pro[bot] <noreply@github.com>",
			aiToolUser: "",
			want:       true,
		},
		{
			name:       "Regular commit, no suggestion",
			message:    "Fix bug in handler\n\nAdded nil check to prevent panic.",
			aiToolUser: "coderabbitai[bot]",
			want:       false,
		},
		{
			name:       "Empty message",
			message:    "",
			aiToolUser: "coderabbitai[bot]",
			want:       false,
		},
		{
			name:       "Commit mentioning AI tool user",
			message:    "Applied fix suggested by coderabbitai[bot]",
			aiToolUser: "coderabbitai[bot]",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isApplySuggestionCommit(tt.message, tt.aiToolUser)
			if got != tt.want {
				t.Errorf("isApplySuggestionCommit(%q, %q) = %v, want %v", tt.message, tt.aiToolUser, got, tt.want)
			}
		})
	}
}

func TestFilePathsMatch(t *testing.T) {
	tests := []struct {
		name  string
		path1 string
		path2 string
		want  bool
	}{
		{"exact match", "pkg/server/server.go", "pkg/server/server.go", true},
		{"with a/ prefix", "a/pkg/server/server.go", "pkg/server/server.go", true},
		{"with b/ prefix", "b/pkg/server/server.go", "pkg/server/server.go", true},
		{"both prefixed", "a/pkg/server/server.go", "b/pkg/server/server.go", true},
		{"suffix match", "src/pkg/server/server.go", "pkg/server/server.go", true},
		{"different files", "pkg/server/server.go", "pkg/server/config.go", false},
		{"different dirs", "pkg/server/server.go", "cmd/server/server.go", false},
		{"empty paths", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filePathsMatch(tt.path1, tt.path2)
			if got != tt.want {
				t.Errorf("filePathsMatch(%q, %q) = %v, want %v", tt.path1, tt.path2, got, tt.want)
			}
		})
	}
}

func TestCalculateTemporalScore(t *testing.T) {
	tests := []struct {
		name  string
		delta time.Duration
		want  float64
	}{
		{"10 minutes", 10 * time.Minute, 75.0},
		{"1 hour", 1 * time.Hour, 60.0},
		{"12 hours", 12 * time.Hour, 45.0},
		{"48 hours", 48 * time.Hour, 30.0},
		{"1 week", 7 * 24 * time.Hour, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateTemporalScore(tt.delta)
			if got != tt.want {
				t.Errorf("calculateTemporalScore(%v) = %f, want %f", tt.delta, got, tt.want)
			}
		})
	}
}

func TestNonTrivialLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"code with empty lines", "foo()\n\nbar()\n", 2},
		{"just braces", "{\n}\n", 0},
		{"mixed", "if x > 0 {\n  return x\n}\n", 2},
		{"whitespace only", "   \n\t\n  \n", 0},
		{"single brace variants", "}\n},\n];\n);\n", 0},
		{"real code", "log.Println(\"hello\")\nreturn nil\n", 2},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nonTrivialLines(tt.input)
			if len(got) != tt.want {
				t.Errorf("nonTrivialLines(%q) = %d lines, want %d (got: %v)", tt.input, len(got), tt.want, got)
			}
		})
	}
}

func TestExtractAddedLines(t *testing.T) {
	patch := `@@ -1,5 +1,7 @@
 func foo() {
-    return nil
+    if x > 0 {
+        return x
+    }
+    return 0
 }
`
	got := extractAddedLines(patch)
	// Should get: "if x > 0 {" (not trivial because has content), "return x", "return 0"
	// "}" is trivial and excluded
	want := 3
	if len(got) != want {
		t.Errorf("extractAddedLines() = %d lines, want %d (got: %v)", len(got), want, got)
	}
}

func TestCountMatchingLines(t *testing.T) {
	tests := []struct {
		name         string
		suggested    []string
		added        []string
		wantMatched  int
		wantTotal    int
	}{
		{
			name:        "full match",
			suggested:   []string{"return x", "return 0"},
			added:       []string{"return x", "return 0", "log.Println()"},
			wantMatched: 2,
			wantTotal:   2,
		},
		{
			name:        "partial match",
			suggested:   []string{"return x", "return y", "return z"},
			added:       []string{"return x", "return w"},
			wantMatched: 1,
			wantTotal:   3,
		},
		{
			name:        "no match",
			suggested:   []string{"foo()"},
			added:       []string{"bar()"},
			wantMatched: 0,
			wantTotal:   1,
		},
		{
			name:        "empty suggestion",
			suggested:   []string{},
			added:       []string{"foo()"},
			wantMatched: 0,
			wantTotal:   0,
		},
		{
			name:        "duplicate lines — no double counting",
			suggested:   []string{"return nil", "return nil"},
			added:       []string{"return nil"},
			wantMatched: 1,
			wantTotal:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, total := countMatchingLines(tt.suggested, tt.added)
			if matched != tt.wantMatched || total != tt.wantTotal {
				t.Errorf("countMatchingLines() = (%d, %d), want (%d, %d)", matched, total, tt.wantMatched, tt.wantTotal)
			}
		})
	}
}

func TestIsTrivialLine(t *testing.T) {
	// isTrivialLine expects already-trimmed input (nonTrivialLines does TrimSpace before calling)
	trivial := []string{"", "{", "}", "},", "];", ");", "};", "*/", "/*", "(", ")", "[", "]", "],"}
	for _, s := range trivial {
		if !isTrivialLine(s) {
			t.Errorf("isTrivialLine(%q) = false, want true", s)
		}
	}

	nonTrivial := []string{"return nil", "if x > 0 {", "// comment", "x := 1"}
	for _, s := range nonTrivial {
		if isTrivialLine(s) {
			t.Errorf("isTrivialLine(%q) = true, want false", s)
		}
	}
}

func TestMatchFinding(t *testing.T) {
	baseTime := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		finding      suggestionFinding
		commits      []prCommit
		fileChanges  []commitFileChange
		patches      []commitFilePatch
		wantMatched  bool
		wantMethod   string
		wantMinScore float64
	}{
		{
			name: "Apply suggestion commit with matching file",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:            "f1",
					FilePath:      "pkg/server/server.go",
					Type:          models.FindingTypeSuggestion,
					SuggestedCode: "return nil",
				},
				ReviewCreatedDate: baseTime,
				AiToolUser:        "coderabbitai[bot]",
			},
			commits: []prCommit{
				{CommitSha: "abc123", AuthoredDate: baseTime.Add(20 * time.Minute), Message: "Apply suggestion from code review\n\nCo-authored-by: coderabbitai[bot] <noreply@github.com>"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "abc123", FilePath: "pkg/server/server.go", Additions: 3, Deletions: 1},
			},
			wantMatched:  true,
			wantMethod:   "diff_commit_msg",
			wantMinScore: 100.0,
		},
		{
			name: "Line-level match with patch data",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:              "f2",
					FilePath:        "pkg/config/config.go",
					MatchedFilePath: "pkg/config/config.go",
					Type:            models.FindingTypeSuggestion,
					SuggestedCode:   "if cfg == nil {\n    return ErrNilConfig\n}\n",
				},
				ReviewCreatedDate: baseTime,
			},
			commits: []prCommit{
				{CommitSha: "def456", AuthoredDate: baseTime.Add(15 * time.Minute), Message: "Fix config validation"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "def456", FilePath: "pkg/config/config.go", Additions: 5, Deletions: 2},
			},
			patches: []commitFilePatch{
				{CommitSha: "def456", FilePath: "pkg/config/config.go", Patch: "@@ -10,3 +10,5 @@\n func Load() {\n+if cfg == nil {\n+    return ErrNilConfig\n+}\n return cfg\n"},
			},
			wantMatched:  true,
			wantMethod:   "diff_line_pct",
			wantMinScore: 100.0,
		},
		{
			name: "Partial line match — 50%",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:              "f2b",
					FilePath:        "pkg/config/config.go",
					MatchedFilePath: "pkg/config/config.go",
					Type:            models.FindingTypeSuggestion,
					SuggestedCode:   "if cfg == nil {\n    return ErrNilConfig\n}\nlog.Warn(\"checked\")\n",
				},
				ReviewCreatedDate: baseTime,
			},
			commits: []prCommit{
				{CommitSha: "def456", AuthoredDate: baseTime.Add(15 * time.Minute), Message: "Fix config validation"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "def456", FilePath: "pkg/config/config.go", Additions: 3, Deletions: 1},
			},
			patches: []commitFilePatch{
				{CommitSha: "def456", FilePath: "pkg/config/config.go", Patch: "@@ -10,3 +10,5 @@\n+if cfg == nil {\n+    return ErrNilConfig\n+}\n"},
			},
			wantMatched:  true,
			wantMethod:   "diff_line_pct",
			wantMinScore: 50.0,
		},
		{
			name: "File modified much later — temporal fallback",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:            "f3",
					FilePath:      "pkg/handler.go",
					Type:          models.FindingTypeSuggestion,
					SuggestedCode: "return err",
				},
				ReviewCreatedDate: baseTime,
			},
			commits: []prCommit{
				{CommitSha: "ghi789", AuthoredDate: baseTime.Add(48 * time.Hour), Message: "Refactor handler"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "ghi789", FilePath: "pkg/handler.go", Additions: 20, Deletions: 10},
			},
			// No patches — falls back to temporal
			wantMatched:  true,
			wantMethod:   "diff_file_temporal",
			wantMinScore: 25.0,
		},
		{
			name: "No matching file in commits",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:            "f4",
					FilePath:      "pkg/server/server.go",
					Type:          models.FindingTypeSuggestion,
					SuggestedCode: "return nil",
				},
				ReviewCreatedDate: baseTime,
			},
			commits: []prCommit{
				{CommitSha: "jkl012", AuthoredDate: baseTime.Add(10 * time.Minute), Message: "Fix tests"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "jkl012", FilePath: "pkg/server/server_test.go", Additions: 10, Deletions: 5},
			},
			wantMatched: false,
		},
		{
			name: "Commit before suggestion — should not match",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:            "f5",
					FilePath:      "pkg/server/server.go",
					Type:          models.FindingTypeSuggestion,
					SuggestedCode: "return nil",
				},
				ReviewCreatedDate: baseTime,
			},
			commits: []prCommit{
				{CommitSha: "mno345", AuthoredDate: baseTime.Add(-1 * time.Hour), Message: "Initial commit"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "mno345", FilePath: "pkg/server/server.go", Additions: 100, Deletions: 0},
			},
			wantMatched: false,
		},
		{
			name: "No file path — only commit message can match",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:            "f6",
					Type:          models.FindingTypeSuggestion,
					SuggestedCode: "x := 1",
				},
				ReviewCreatedDate: baseTime,
				AiToolUser:        "coderabbitai[bot]",
			},
			commits: []prCommit{
				{CommitSha: "pqr678", AuthoredDate: baseTime.Add(5 * time.Minute), Message: "Apply suggestion\n\nCo-authored-by: coderabbitai[bot] <noreply>"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "pqr678", FilePath: "pkg/handler.go", Additions: 2, Deletions: 1},
			},
			wantMatched:  true,
			wantMethod:   "diff_commit_msg",
			wantMinScore: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchFinding(tt.finding, tt.commits, tt.fileChanges, tt.patches)
			if result.Matched != tt.wantMatched {
				t.Errorf("matchFinding() matched = %v, want %v", result.Matched, tt.wantMatched)
			}
			if tt.wantMatched {
				if result.Method != tt.wantMethod {
					t.Errorf("matchFinding() method = %q, want %q", result.Method, tt.wantMethod)
				}
				if result.Score < tt.wantMinScore {
					t.Errorf("matchFinding() score = %f, want >= %f", result.Score, tt.wantMinScore)
				}
			}
		})
	}
}

func TestParseFilesJSON(t *testing.T) {
	input := `[{"sha":"abc","filename":"pkg/foo.go","status":"modified","additions":2,"deletions":1,"changes":3,"patch":"@@ -1,3 +1,4 @@\n func foo() {\n+    return nil\n }"},{"sha":"abc","filename":"pkg/bar.go","status":"modified","additions":1,"deletions":0,"changes":1,"patch":"@@ -5,2 +5,3 @@\n+import \"fmt\""},{"sha":"abc","filename":"pkg/bin","status":"added"}]`

	files := parseFilesJSON(input)
	if len(files) != 2 {
		t.Fatalf("parseFilesJSON() returned %d files, want 2 (binary file should be excluded)", len(files))
	}
	if files[0].Filename != "pkg/foo.go" {
		t.Errorf("files[0].Filename = %q, want %q", files[0].Filename, "pkg/foo.go")
	}
	if files[1].Filename != "pkg/bar.go" {
		t.Errorf("files[1].Filename = %q, want %q", files[1].Filename, "pkg/bar.go")
	}
	if len(files[0].Patch) == 0 {
		t.Error("files[0].Patch is empty")
	}
	// Verify JSON unescaping works — \n in patch should become real newlines
	if !strings.Contains(files[0].Patch, "\n") {
		t.Errorf("files[0].Patch should contain real newlines, got: %q", files[0].Patch)
	}
}

func TestLoadSuggestionFindings(t *testing.T) {
	t.Run("success with one row", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockRows := new(mockdal.Rows)

		mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
		mockRows.On("Next").Return(true).Once()
		mockRows.On("Next").Return(false).Once()
		mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(1).(*suggestionFinding)
			dst.AiReviewFinding = models.AiReviewFinding{
				Id:       "finding-1",
				FilePath: "pkg/server.go",
				RepoId:   "repo-1",
				Type:     models.FindingTypeSuggestion,
			}
			dst.ReviewCreatedDate = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			dst.AiToolUser = "coderabbitai[bot]"
		}).Return(nil)
		mockRows.On("Close").Return(nil)

		findings, err := loadSuggestionFindings(mockDal, "repo-1")
		assert.Nil(t, err)
		assert.Len(t, findings, 1)
		assert.Equal(t, "finding-1", findings[0].Id)
		assert.Equal(t, "coderabbitai[bot]", findings[0].AiToolUser)
		mockDal.AssertExpectations(t)
		mockRows.AssertExpectations(t)
	})

	t.Run("cursor error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Cursor", mock.Anything).Return(nil, errors.Default.New("db error"))

		findings, err := loadSuggestionFindings(mockDal, "repo-1")
		assert.Nil(t, findings)
		assert.NotNil(t, err)
		assert.Contains(t, fmt.Sprint(err), "db error")
	})

	t.Run("fetch error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockRows := new(mockdal.Rows)

		mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
		mockRows.On("Next").Return(true).Once()
		mockDal.On("Fetch", mockRows, mock.Anything).Return(errors.Default.New("scan error"))
		mockRows.On("Close").Return(nil)

		findings, err := loadSuggestionFindings(mockDal, "repo-1")
		assert.Nil(t, findings)
		assert.NotNil(t, err)
		assert.Contains(t, fmt.Sprint(err), "scan error")
	})
}

func TestLoadPRCommits(t *testing.T) {
	t.Run("success with one row", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockRows := new(mockdal.Rows)

		mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
		mockRows.On("Next").Return(true).Once()
		mockRows.On("Next").Return(false).Once()
		mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(1).(*prCommit)
			dst.CommitSha = "abc123"
			dst.AuthoredDate = time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
			dst.Message = "Fix handler"
			dst.AuthorName = "dev"
		}).Return(nil)
		mockRows.On("Close").Return(nil)

		commits, err := loadPRCommits(mockDal, "pr-1")
		assert.Nil(t, err)
		assert.Len(t, commits, 1)
		assert.Equal(t, "abc123", commits[0].CommitSha)
		assert.Equal(t, "Fix handler", commits[0].Message)
		mockDal.AssertExpectations(t)
	})

	t.Run("cursor error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Cursor", mock.Anything).Return(nil, errors.Default.New("db error"))

		commits, err := loadPRCommits(mockDal, "pr-1")
		assert.Nil(t, commits)
		assert.NotNil(t, err)
	})

	t.Run("fetch error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockRows := new(mockdal.Rows)

		mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
		mockRows.On("Next").Return(true).Once()
		mockDal.On("Fetch", mockRows, mock.Anything).Return(errors.Default.New("scan error"))
		mockRows.On("Close").Return(nil)

		commits, err := loadPRCommits(mockDal, "pr-1")
		assert.Nil(t, commits)
		assert.NotNil(t, err)
	})
}

func TestLoadCommitFiles(t *testing.T) {
	t.Run("empty commitShas returns nil", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		files, err := loadCommitFiles(mockDal, []string{})
		assert.Nil(t, err)
		assert.Nil(t, files)
	})

	t.Run("success with one row", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockRows := new(mockdal.Rows)

		mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
		mockRows.On("Next").Return(true).Once()
		mockRows.On("Next").Return(false).Once()
		mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(1).(*commitFileChange)
			dst.CommitSha = "abc123"
			dst.FilePath = "pkg/handler.go"
			dst.Additions = 5
			dst.Deletions = 2
		}).Return(nil)
		mockRows.On("Close").Return(nil)

		files, err := loadCommitFiles(mockDal, []string{"abc123"})
		assert.Nil(t, err)
		assert.Len(t, files, 1)
		assert.Equal(t, "pkg/handler.go", files[0].FilePath)
		assert.Equal(t, 5, files[0].Additions)
		mockDal.AssertExpectations(t)
	})

	t.Run("cursor error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("Cursor", mock.Anything).Return(nil, errors.Default.New("db error"))

		files, err := loadCommitFiles(mockDal, []string{"abc123"})
		assert.Nil(t, files)
		assert.NotNil(t, err)
	})
}
