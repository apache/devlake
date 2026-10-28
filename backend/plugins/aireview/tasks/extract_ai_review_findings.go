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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
)

var ExtractAiReviewFindingsMeta = plugin.SubTaskMeta{
	Name:             "extractAiReviewFindings",
	EntryPoint:       ExtractAiReviewFindings,
	EnabledByDefault: true,
	Description:      "Extract individual findings from AI reviews",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAiReviewsMeta},
}

// ExtractAiReviewFindings parses AI reviews and extracts individual findings
func ExtractAiReviewFindings(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	logger.Info("Extracting findings from AI reviews for repo: %s", data.Options.RepoId)

	// Query AI reviews
	cursor, err := db.Cursor(
		dal.From(&models.AiReview{}),
		dal.Where("repo_id = ?", data.Options.RepoId),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query AI reviews")
	}
	defer cursor.Close()

	totalFindings := 0
	batchSize := 100
	batch := make([]*models.AiReviewFinding, 0, batchSize)

	for cursor.Next() {
		var review models.AiReview
		if err := db.Fetch(cursor, &review); err != nil {
			return errors.Default.Wrap(err, "failed to fetch AI review")
		}

		// Parse findings from review body
		findings := parseFindings(&review)
		totalFindings += len(findings)

		for _, finding := range findings {
			batch = append(batch, finding)
			if len(batch) >= batchSize {
				if err := saveFindingsBatch(db, batch); err != nil {
					return err
				}
				batch = make([]*models.AiReviewFinding, 0, batchSize)
			}
		}
	}

	// Save remaining batch
	if len(batch) > 0 {
		if err := saveFindingsBatch(db, batch); err != nil {
			return err
		}
	}

	logger.Info("Completed finding extraction: %d findings found", totalFindings)
	return nil
}

// parseFindings extracts individual findings from an AI review
func parseFindings(review *models.AiReview) []*models.AiReviewFinding {
	var findings []*models.AiReviewFinding
	body := normalizeBody(review.Body)

	// Parse CodeRabbit-style findings (file blocks, etc.)
	if review.AiTool == models.AiToolCodeRabbit {
		findings = append(findings, parseCodeRabbitFindings(review, body)...)
	}

	// Parse ```suggestion blocks — GitHub-native feature used by all AI tools
	findings = append(findings, parseSuggestionBlocks(review, body)...)

	// Parse generic inline comment findings
	findings = append(findings, parseGenericFindings(review, body)...)

	return findings
}

// parseCodeRabbitFindings extracts findings from CodeRabbit format
func parseCodeRabbitFindings(review *models.AiReview, body string) []*models.AiReviewFinding {
	var findings []*models.AiReviewFinding

	// Pattern for file-level findings
	// Example: "📁 file.go\n- Issue description"
	filePattern := regexp.MustCompile(`(?m)(?:📁|File:)\s*([^\n]+)\n((?:[-*•]\s*[^\n]+\n?)+)`)
	fileMatches := filePattern.FindAllStringSubmatch(body, -1)

	for _, match := range fileMatches {
		if len(match) < 3 {
			continue
		}
		filePath := strings.TrimSpace(match[1])
		issuesBlock := match[2]

		// Parse individual issues within the file block
		issuePattern := regexp.MustCompile(`(?m)[-*•]\s*(.+)`)
		issueMatches := issuePattern.FindAllStringSubmatch(issuesBlock, -1)

		for idx, issue := range issueMatches {
			if len(issue) < 2 {
				continue
			}
			description := strings.TrimSpace(issue[1])

			finding := &models.AiReviewFinding{
				Id:            generateFindingId(review.Id, filePath, idx),
				AiReviewId:    review.Id,
				PullRequestId: review.PullRequestId,
				RepoId:        review.RepoId,
				AiTool:        review.AiTool,
				FilePath:      filePath,
				Description:   description,
				Category:      detectFindingCategory(description),
				Severity:      detectFindingSeverity(description),
				Type:          detectFindingType(description),
				Title:         truncateTitle(description),
				CreatedDate:   review.CreatedDate,
			}
			findings = append(findings, finding)
		}
	}

	return findings
}

// parseSuggestionBlocks extracts ```suggestion code blocks from any AI review comment.
// This is a GitHub-native feature used by CodeRabbit, Gemini Code Assist, Qodo, and others.
func parseSuggestionBlocks(review *models.AiReview, body string) []*models.AiReviewFinding {
	var findings []*models.AiReviewFinding

	suggestionPattern := regexp.MustCompile("(?s)```suggestion\\s*\\n(.+?)```")
	suggestionMatches := suggestionPattern.FindAllStringSubmatch(body, -1)

	for idx, match := range suggestionMatches {
		if len(match) < 2 {
			continue
		}
		suggestedCode := strings.TrimSpace(match[1])

		finding := &models.AiReviewFinding{
			Id:                generateFindingId(review.Id, "suggestion", idx),
			AiReviewId:        review.Id,
			PullRequestId:     review.PullRequestId,
			RepoId:            review.RepoId,
			AiTool:            review.AiTool,
			SuggestedCode:     suggestedCode,
			SuggestionApplied: detectSuggestionApplied(body, idx),
			Category:          models.FindingCategoryBestPractice,
			Severity:          models.FindingSeverityInfo,
			Type:              models.FindingTypeSuggestion,
			Title:             "Code suggestion",
			Description:       "AI-suggested code change",
			CreatedDate:       review.CreatedDate,
		}
		findings = append(findings, finding)
	}

	return findings
}

// parseGenericFindings extracts findings from generic comment format
func parseGenericFindings(review *models.AiReview, body string) []*models.AiReviewFinding {
	var findings []*models.AiReviewFinding

	// Bullet point patterns
	bulletPattern := regexp.MustCompile(`(?m)^[-*•]\s+(.+)$`)
	bulletMatches := bulletPattern.FindAllStringSubmatch(body, -1)

	for idx, match := range bulletMatches {
		if len(match) < 2 {
			continue
		}
		description := strings.TrimSpace(match[1])

		// Skip if too short or likely a header
		if len(description) < 20 {
			continue
		}

		// Detect file path in the line
		filePath := ""
		filePattern := regexp.MustCompile(`\b([\w/.-]+\.(?:go|ts|js|py|java|rs|cpp|c|h))\b`)
		if fileMatch := filePattern.FindString(description); fileMatch != "" {
			filePath = fileMatch
		}

		finding := &models.AiReviewFinding{
			Id:            generateFindingId(review.Id, "bullet", idx),
			AiReviewId:    review.Id,
			PullRequestId: review.PullRequestId,
			RepoId:        review.RepoId,
			AiTool:        review.AiTool,
			FilePath:      filePath,
			Description:   description,
			Category:      detectFindingCategory(description),
			Severity:      detectFindingSeverity(description),
			Type:          detectFindingType(description),
			Title:         truncateTitle(description),
			CreatedDate:   review.CreatedDate,
		}
		findings = append(findings, finding)
	}

	return findings
}

// generateFindingId creates a deterministic ID for a finding
func generateFindingId(reviewId, context string, index int) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", reviewId, context, index)))
	return "aifinding:" + hex.EncodeToString(hash[:16])
}

// detectFindingCategory determines the category of a finding
func detectFindingCategory(text string) string {
	text = strings.ToLower(text)

	categoryPatterns := map[string]*regexp.Regexp{
		models.FindingCategorySecurity:        regexp.MustCompile(`(?i)(security|vulnerab|xss|sql.?inject|auth|creds|secret|token|password)`),
		models.FindingCategoryPerformance:     regexp.MustCompile(`(?i)(performance|slow|optimi|efficien|memory|leak|cache|latency)`),
		models.FindingCategoryBug:             regexp.MustCompile(`(?i)(bug|error|crash|fail|broken|undefined|null.?pointer|exception)`),
		models.FindingCategoryStyle:           regexp.MustCompile(`(?i)(style|format|indent|naming|convention|lint)`),
		models.FindingCategoryDocumentation:   regexp.MustCompile(`(?i)(doc|comment|readme|describe|explain)`),
		models.FindingCategoryMaintainability: regexp.MustCompile(`(?i)(maintain|refactor|complex|duplicate|dry|solid|clean)`),
	}

	for category, pattern := range categoryPatterns {
		if pattern.MatchString(text) {
			return category
		}
	}

	return models.FindingCategoryBestPractice
}

// detectFindingSeverity determines the severity of a finding
func detectFindingSeverity(text string) string {
	text = strings.ToLower(text)

	if regexp.MustCompile(`(?i)(critical|severe|security|vulnerab|crash|data.?loss)`).MatchString(text) {
		return models.FindingSeverityCritical
	}
	if regexp.MustCompile(`(?i)(error|bug|fail|broken|must|required)`).MatchString(text) {
		return models.FindingSeverityError
	}
	if regexp.MustCompile(`(?i)(warning|should|recommend|consider)`).MatchString(text) {
		return models.FindingSeverityWarning
	}

	return models.FindingSeverityInfo
}

// detectFindingType determines the type of a finding
func detectFindingType(text string) string {
	text = strings.ToLower(text)

	if regexp.MustCompile(`(?i)(suggest|recommend|consider|could|might)`).MatchString(text) {
		return models.FindingTypeSuggestion
	}
	if regexp.MustCompile(`(?i)(issue|bug|error|problem|fail)`).MatchString(text) {
		return models.FindingTypeIssue
	}

	return models.FindingTypeComment
}

// detectSuggestionApplied checks whether a suggestion block within a comment
// has been marked as applied/resolved by the AI tool.
// The index parameter identifies which suggestion block in the body we're checking.
func detectSuggestionApplied(body string, _ int) bool {
	// Check for global acceptance markers in the comment body.
	// When the entire comment has acceptance signals, individual suggestions
	// within it are considered applied.
	appliedRe := regexp.MustCompile(`(?i)(?:applied suggestion|suggestion applied|✅\s*Resolved|suggestion\s+(?:was\s+)?(?:applied|committed)\s+in\s+commit)`)
	return appliedRe.MatchString(body)
}

// normalizeBody strips JSON quoting and converts escaped newlines to real newlines.
// Comment bodies from the domain table may be stored with JSON encoding
// (surrounding quotes, escaped newlines as \\n).
func normalizeBody(body string) string {
	// Strip surrounding JSON quotes
	if len(body) >= 2 && body[0] == '"' && body[len(body)-1] == '"' {
		body = body[1 : len(body)-1]
	}
	// Convert escaped newlines/quotes
	body = strings.ReplaceAll(body, "\\n", "\n")
	body = strings.ReplaceAll(body, "\\r", "")
	body = strings.ReplaceAll(body, "\\\"", "\"")
	return body
}

// truncateTitle creates a short title from description
func truncateTitle(description string) string {
	// Get first sentence or 80 chars
	if idx := strings.IndexAny(description, ".!?\n"); idx > 0 && idx < 80 {
		return description[:idx]
	}
	if len(description) > 80 {
		return description[:77] + "..."
	}
	return description
}

// saveFindingsBatch saves a batch of findings to the database
func saveFindingsBatch(db dal.Dal, batch []*models.AiReviewFinding) errors.Error {
	for _, finding := range batch {
		err := db.CreateOrUpdate(finding)
		if err != nil {
			return errors.Default.Wrap(err, "failed to save AI review finding")
		}
	}
	return nil
}
