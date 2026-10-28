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
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
)

var ExtractAiReviewsMeta = plugin.SubTaskMeta{
	Name:             "extractAiReviews",
	EntryPoint:       ExtractAiReviews,
	EnabledByDefault: true,
	Description:      "Extract AI-generated reviews from pull request comments",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
}

// ExtractAiReviews identifies and extracts AI-generated reviews from PR comments
func ExtractAiReviews(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	// Build query based on mode (projectName or repoId)
	var clauses []dal.Clause

	if data.Options.ProjectName != "" {
		logger.Info("Starting AI review extraction for project: %s", data.Options.ProjectName)
		// Project mode: join with project_mappings to get all repos in project
		clauses = []dal.Clause{
			dal.Select("prc.*, pr.base_repo_id, pr.status as pr_status, pr.merged_date, pr.url as pr_url, a.user_name as account_username"),
			dal.From("pull_request_comments prc"),
			dal.Join("LEFT JOIN pull_requests pr ON prc.pull_request_id = pr.id"),
			dal.Join("LEFT JOIN accounts a ON prc.account_id = a.id"),
			dal.Join("LEFT JOIN project_mapping pm ON pr.base_repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ?", data.Options.ProjectName, "repos"),
		}
	} else {
		logger.Info("Starting AI review extraction for repo: %s", data.Options.RepoId)
		// Single repo mode
		clauses = []dal.Clause{
			dal.Select("prc.*, pr.base_repo_id, pr.status as pr_status, pr.merged_date, pr.url as pr_url, a.user_name as account_username"),
			dal.From("pull_request_comments prc"),
			dal.Join("LEFT JOIN pull_requests pr ON prc.pull_request_id = pr.id"),
			dal.Join("LEFT JOIN accounts a ON prc.account_id = a.id"),
			dal.Where("pr.base_repo_id = ?", data.Options.RepoId),
		}
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query pull request comments")
	}
	defer cursor.Close()

	// Track processed reviews to avoid duplicates
	processedReviews := make(map[string]bool)
	batchSize := 100
	batch := make([]*models.AiReview, 0, batchSize)

	for cursor.Next() {
		var comment struct {
			code.PullRequestComment
			BaseRepoId      string     `gorm:"column:base_repo_id"`
			PrStatus        string     `gorm:"column:pr_status"`
			MergedDate      *time.Time `gorm:"column:merged_date"`
			PrUrl           string     `gorm:"column:pr_url"`
			AccountUsername string     `gorm:"column:account_username"`
		}

		if err := db.Fetch(cursor, &comment); err != nil {
			return errors.Default.Wrap(err, "failed to fetch comment")
		}

		// Use the resolved username from accounts table for reliable tool detection.
		// Fall back to domain account_id if accounts table entry is missing.
		username := comment.AccountUsername
		if username == "" {
			username = comment.AccountId
		}

		// Check if this is an AI-generated review
		aiTool, isAiReview := detectAiTool(data, username, comment.Body)
		if !isAiReview {
			continue
		}

		// Generate unique ID for this review
		reviewId := generateReviewId(comment.PullRequestId, comment.Id, aiTool)
		if processedReviews[reviewId] {
			continue
		}
		processedReviews[reviewId] = true

		// Parse the review content for metrics
		reviewMetrics := parseReviewMetrics(comment.Body)

		// Detect risk level
		riskLevel, riskScore := detectRiskLevel(data, comment.Body)

		// Determine repo ID (from query result in project mode, from options in repo mode)
		repoId := comment.BaseRepoId
		if repoId == "" {
			repoId = data.Options.RepoId
		}

		// Create AI review record
		aiReview := &models.AiReview{
			Id:                         reviewId,
			PullRequestId:              comment.PullRequestId,
			RepoId:                     repoId,
			AiTool:                     aiTool,
			AiToolUser:                 username,
			ReviewId:                   comment.Id,
			Body:                       comment.Body,
			Summary:                    extractSummary(comment.Body),
			CreatedDate:                comment.CreatedDate,
			RiskLevel:                  riskLevel,
			RiskScore:                  riskScore,
			RiskConfidence:             reviewMetrics.Confidence,
			IssuesFound:                reviewMetrics.IssuesFound,
			SuggestionsCount:           reviewMetrics.SuggestionsCount,
			FilesReviewed:              reviewMetrics.FilesReviewed,
			LinesReviewed:              reviewMetrics.LinesReviewed,
			EffortComplexity:           reviewMetrics.Complexity,
			EffortRating:               reviewMetrics.EffortRating,
			EffortMinutes:              reviewMetrics.EffortMinutes,
			SuggestionsAccepted:        reviewMetrics.SuggestionsAccepted,
			PreMergeChecksPassed:       reviewMetrics.PreMergeChecksPassed,
			PreMergeChecksFailed:       reviewMetrics.PreMergeChecksFailed,
			PreMergeChecksInconclusive: reviewMetrics.PreMergeChecksInconclusive,
			ReviewState:                detectReviewState(comment.Body, comment.Status),
			SourcePlatform:             detectSourcePlatform(comment.PullRequestId),
			SourceUrl:                  buildCommentUrl(comment.PrUrl, comment.Id),
		}

		batch = append(batch, aiReview)

		if len(batch) >= batchSize {
			if err := saveBatch(db, batch); err != nil {
				return err
			}
			batch = make([]*models.AiReview, 0, batchSize)
		}
	}

	// Save remaining batch
	if len(batch) > 0 {
		if err := saveBatch(db, batch); err != nil {
			return err
		}
	}

	logger.Info("Completed AI review extraction: %d reviews found", len(processedReviews))
	return nil
}

// detectAiTool checks if the comment is from an AI review tool
func detectAiTool(data *AiReviewTaskData, accountId, body string) (string, bool) {
	// Check CodeRabbit
	if data.Options.ScopeConfig.CodeRabbitEnabled {
		if data.CodeRabbitUsernameRegex != nil && data.CodeRabbitUsernameRegex.MatchString(accountId) {
			return models.AiToolCodeRabbit, true
		}
		if data.CodeRabbitPatternRegex != nil && data.CodeRabbitPatternRegex.MatchString(body) {
			return models.AiToolCodeRabbit, true
		}
	}

	// Check Cursor Bugbot
	if data.Options.ScopeConfig.CursorBugbotEnabled {
		if data.CursorBugbotUsernameRegex != nil && data.CursorBugbotUsernameRegex.MatchString(accountId) {
			return models.AiToolCursorBugbot, true
		}
		if data.CursorBugbotPatternRegex != nil && data.CursorBugbotPatternRegex.MatchString(body) {
			return models.AiToolCursorBugbot, true
		}
	}

	// Check Qodo (formerly Codium)
	if data.Options.ScopeConfig.QodoEnabled {
		if data.QodoUsernameRegex != nil && data.QodoUsernameRegex.MatchString(accountId) {
			return models.AiToolQodo, true
		}
		if data.QodoPatternRegex != nil && data.QodoPatternRegex.MatchString(body) {
			return models.AiToolQodo, true
		}
	}

	// Check Gemini Code Assist
	if data.Options.ScopeConfig.GeminiEnabled {
		if data.GeminiUsernameRegex != nil && data.GeminiUsernameRegex.MatchString(accountId) {
			return models.AiToolGemini, true
		}
		if data.GeminiPatternRegex != nil && data.GeminiPatternRegex.MatchString(body) {
			return models.AiToolGemini, true
		}
	}

	return "", false
}

// generateReviewId creates a deterministic ID for an AI review
func generateReviewId(prId, commentId, aiTool string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", prId, commentId, aiTool)))
	return "aireview:" + hex.EncodeToString(hash[:16])
}

// ReviewMetrics holds parsed metrics from review content
type ReviewMetrics struct {
	IssuesFound                int
	SuggestionsCount           int
	SuggestionsAccepted        int
	FilesReviewed              int
	LinesReviewed              int
	Complexity                 string
	EffortRating               int // 1-5 scale numeric rating
	EffortMinutes              int
	Confidence                 int
	PreMergeChecksPassed       int
	PreMergeChecksFailed       int
	PreMergeChecksInconclusive int
}

// parseReviewMetrics extracts metrics from review body
func parseReviewMetrics(body string) ReviewMetrics {
	metrics := ReviewMetrics{
		Confidence: 70, // Default confidence
	}

	// Parse CodeRabbit numeric effort rating (e.g., "🎯 3 (Moderate)" or "🎯 3")
	// This format appears in CodeRabbit reviews
	effortRatingRe := regexp.MustCompile(`🎯\s*(\d)(?:\s*\([^)]+\))?`)
	if match := effortRatingRe.FindStringSubmatch(body); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil && val >= 1 && val <= 5 {
			metrics.EffortRating = val
		}
	}

	// Parse Qodo effort rating (e.g., "Estimated effort to review: 3" or with emoji dots)
	if metrics.EffortRating == 0 {
		qodoEffortRe := regexp.MustCompile(`(?i)estimated effort[^:]*:\s*(\d)`)
		if match := qodoEffortRe.FindStringSubmatch(body); len(match) > 1 {
			if val, err := strconv.Atoi(match[1]); err == nil && val >= 1 && val <= 5 {
				metrics.EffortRating = val
			}
		}
	}

	// Parse effort/complexity (CodeRabbit format)
	complexityRe := regexp.MustCompile(`(?i)(simple|moderate|complex|trivial)`)
	if match := complexityRe.FindString(body); match != "" {
		metrics.Complexity = strings.ToLower(match)
		switch metrics.Complexity {
		case "trivial", "simple":
			metrics.EffortMinutes = 5
		case "moderate":
			metrics.EffortMinutes = 15
		case "complex":
			metrics.EffortMinutes = 30
		}
	}

	// Infer complexity from effort rating if not explicitly stated
	if metrics.Complexity == "" && metrics.EffortRating > 0 {
		switch {
		case metrics.EffortRating <= 2:
			metrics.Complexity = "simple"
		case metrics.EffortRating <= 3:
			metrics.Complexity = "moderate"
		default:
			metrics.Complexity = "complex"
		}
	}

	// Parse time estimate (e.g., "~12 minutes" or "⏱️ ~20 minutes")
	timeRe := regexp.MustCompile(`(?:⏱️\s*)?~?(\d+)\s*minutes?`)
	if match := timeRe.FindStringSubmatch(body); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			metrics.EffortMinutes = val
		}
	}

	// Parse pre-merge check results (CodeRabbit format: "2 passed, 1 inconclusive")
	// Also handles: "✅ 2 checks passed" or "❌ 1 check failed"
	parsePreMergeChecks(body, &metrics)

	// Parse suggestion acceptance signals from AI tool comment bodies.
	// AI tools update their comments to reflect which suggestions were applied:
	// - CodeRabbit: "✅ Resolved" markers in tracking tables, resolved thread counts
	// - Qodo: checkbox rows marked with ✅ in suggestion tables
	parseSuggestionAcceptance(body, &metrics)

	// Count issue patterns
	issuePatterns := []string{
		`(?i)\b(bug|error|issue|problem|warning)\b`,
		`(?i)❌`,
		`(?i)⚠️`,
	}
	for _, pattern := range issuePatterns {
		re := regexp.MustCompile(pattern)
		metrics.IssuesFound += len(re.FindAllString(body, -1))
	}

	// Count suggestions
	suggestionRe := regexp.MustCompile(`(?i)(suggest|recommend|consider|should|could)`)
	metrics.SuggestionsCount = len(suggestionRe.FindAllString(body, -1))

	// Count file references
	fileRe := regexp.MustCompile(`\b[\w/]+\.(go|ts|js|py|java|rs|cpp|c|h)\b`)
	files := make(map[string]bool)
	for _, match := range fileRe.FindAllString(body, -1) {
		files[match] = true
	}
	metrics.FilesReviewed = len(files)

	// Parse lines changed (e.g., "+50 −36")
	linesRe := regexp.MustCompile(`\+(\d+)\s*[−-](\d+)`)
	if match := linesRe.FindStringSubmatch(body); len(match) > 2 {
		added, err1 := strconv.Atoi(match[1])
		removed, err2 := strconv.Atoi(match[2])
		if err1 == nil && err2 == nil {
			metrics.LinesReviewed = added + removed
		}
	}

	return metrics
}

// parsePreMergeChecks extracts pre-merge check results from CodeRabbit format
// Handles formats like: "2 passed, 1 inconclusive" or "✅ 2 checks passed"
func parsePreMergeChecks(body string, metrics *ReviewMetrics) {
	// CodeRabbit format: "N passed" or "✅ N checks passed" or "N checks passed"
	passedRe := regexp.MustCompile(`(?i)(?:✅\s*)?(\d+)\s*(?:checks?\s+)?passed`)
	if match := passedRe.FindStringSubmatch(body); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			metrics.PreMergeChecksPassed = val
		}
	}

	// CodeRabbit format: "N failed" or "❌ N checks failed" or "N checks failed"
	failedRe := regexp.MustCompile(`(?i)(?:❌\s*)?(\d+)\s*(?:checks?\s+)?failed`)
	if match := failedRe.FindStringSubmatch(body); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			metrics.PreMergeChecksFailed = val
		}
	}

	// CodeRabbit format: "N inconclusive"
	inconclusiveRe := regexp.MustCompile(`(?i)(\d+)\s*(?:checks?\s+)?inconclusive`)
	if match := inconclusiveRe.FindStringSubmatch(body); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			metrics.PreMergeChecksInconclusive = val
		}
	}
}

// parseSuggestionAcceptance counts accepted/applied suggestions from AI tool comment bodies.
//
// AI review tools update their own comments to reflect suggestion outcomes:
//
// CodeRabbit patterns:
//   - Tracking table rows with "✅ Resolved" or "✅" status markers
//   - "N suggestions applied" or "N/M suggestions accepted"
//   - "Applied suggestion" in resolution tracking
//
// Qodo patterns:
//   - Suggestion table rows with "✅" checkbox markers (applied)
//   - "[x]" checked checkboxes in suggestion lists (applied via Apply button)
//   - "N suggestions implemented" summary lines
func parseSuggestionAcceptance(body string, metrics *ReviewMetrics) {
	accepted := 0

	// Pattern 1: Explicit "N suggestions applied/accepted/implemented" summary lines
	// Matches: "3 suggestions applied", "2/5 suggestions accepted", "1 suggestion implemented"
	summaryRe := regexp.MustCompile(`(?i)(\d+)(?:/\d+)?\s+suggestions?\s+(?:applied|accepted|implemented)`)
	if match := summaryRe.FindStringSubmatch(body); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			accepted += val
		}
	}

	// Pattern 2: "Applied suggestion" markers (CodeRabbit tracking tables)
	// Matches individual "Applied suggestion" or "Suggestion applied" entries
	appliedRe := regexp.MustCompile(`(?i)(?:applied suggestion|suggestion applied|✅\s*Resolved)`)
	accepted += len(appliedRe.FindAllString(body, -1))

	// Pattern 3: Checked checkboxes in suggestion lists (Qodo "Apply" checkboxes)
	// Matches: "- [x] **suggestion title**" but NOT "- [ ] **suggestion title**"
	// These appear in Qodo's persistent suggestion table when a developer clicks Apply.
	// Note: comment bodies from CSV/DB may store newlines as literal "\n" (escaped),
	// so we match both real newlines and the "- " prefix mid-line.
	checkedBoxRe := regexp.MustCompile(`(?m)(?:^|\\n)[\s-]*\[x\]\s+`)
	uncheckedBoxRe := regexp.MustCompile(`(?m)(?:^|\\n)[\s-]*\[ \]\s+`)
	checkedCount := len(checkedBoxRe.FindAllString(body, -1))
	uncheckedCount := len(uncheckedBoxRe.FindAllString(body, -1))
	// Only count checkboxes as accepted suggestions if there's a mix of checked/unchecked
	// (pure checked lists are likely self-review checkboxes, not suggestion tracking)
	if checkedCount > 0 && uncheckedCount > 0 {
		accepted += checkedCount
	}

	// Pattern 4: CodeRabbit "committed suggestion" references
	// When a user clicks "Commit suggestion" on GitHub, the thread may contain
	// "Suggestion was applied in commit <sha>" or similar
	committedRe := regexp.MustCompile(`(?i)suggestion\s+(?:was\s+)?(?:applied|committed)\s+in\s+commit`)
	accepted += len(committedRe.FindAllString(body, -1))

	metrics.SuggestionsAccepted = accepted
}

// extractSummary extracts a clean markdown summary from the review body
func extractSummary(body string) string {
	// First, convert HTML to markdown
	cleaned := htmlToMarkdown(body)

	// Try to extract specific sections based on AI tool format
	var summaryParts []string

	// Qodo format: Extract key sections
	if strings.Contains(cleaned, "PR Reviewer Guide") || strings.Contains(cleaned, "Estimated effort") {
		// Extract effort estimate
		effortRe := regexp.MustCompile(`(?i)\*\*Estimated effort[^*]*\*\*[:\s]*(\d+)`)
		if match := effortRe.FindStringSubmatch(cleaned); len(match) > 1 {
			summaryParts = append(summaryParts, "Effort: "+match[1]+"/5")
		}

		// Extract security status
		if strings.Contains(cleaned, "No security concerns") {
			summaryParts = append(summaryParts, "Security: OK")
		} else if strings.Contains(cleaned, "security") {
			summaryParts = append(summaryParts, "Security: Review needed")
		}

		// Extract focus area titles (bold items after "Recommended focus areas")
		focusRe := regexp.MustCompile(`\*\*([^*]+)\*\*`)
		focusMatches := focusRe.FindAllStringSubmatch(cleaned, -1)
		for _, match := range focusMatches {
			title := strings.TrimSpace(match[1])
			// Skip generic headers
			if title != "" && !strings.Contains(strings.ToLower(title), "estimated effort") &&
				!strings.Contains(strings.ToLower(title), "security") &&
				!strings.Contains(strings.ToLower(title), "ticket") &&
				!strings.Contains(strings.ToLower(title), "recommended focus") &&
				len(title) < 100 {
				summaryParts = append(summaryParts, title)
				if len(summaryParts) >= 4 {
					break
				}
			}
		}
	}

	// Gemini Code Assist format: PR-level summary comment
	// Format: "## Summary of Changes\n\nHello @user, I'm Gemini Code Assist...\n\nThis pull request [summary]...\n\n### Highlights\n* ..."
	if len(summaryParts) == 0 && strings.Contains(cleaned, "Summary of Changes") && strings.Contains(cleaned, "Gemini Code Assist") {
		// Extract the first substantive paragraph after the greeting line
		paragraphs := regexp.MustCompile(`\n\s*\n`).Split(cleaned, -1)
		for _, p := range paragraphs {
			p = strings.TrimSpace(p)
			// Skip headers, greeting line, and short lines
			if strings.HasPrefix(p, "#") || strings.Contains(p, "Gemini Code Assist") || len(p) < 30 {
				continue
			}
			summaryParts = append(summaryParts, p)
			break
		}
		// Also extract Highlights items
		highlightsRe := regexp.MustCompile(`(?m)^\*\s+\*\*([^*]+)\*\*`)
		for _, match := range highlightsRe.FindAllStringSubmatch(cleaned, 3) {
			summaryParts = append(summaryParts, match[1])
		}
	}

	// Gemini Code Assist format: Inline review comment with priority badge
	// Format: "![medium](https://www.gstatic.com/codereviewagent/medium-priority.svg)\n\n[review text]"
	// May have multiple badges on one line: "![security-high](...) ![high](...)\n\n[review text]"
	if len(summaryParts) == 0 && strings.Contains(cleaned, "gstatic.com/codereviewagent") {
		// Strip all image badge references (may appear multiple times on a line)
		badgeRe := regexp.MustCompile(`!\[[^\]]*\]\([^)]*gstatic\.com/codereviewagent[^)]*\)\s*`)
		stripped := strings.TrimSpace(badgeRe.ReplaceAllString(cleaned, ""))
		if len(stripped) > 20 {
			summaryParts = append(summaryParts, stripped)
		}
	}

	// CodeRabbit format: Look for "Walkthrough" section
	if len(summaryParts) == 0 && strings.Contains(cleaned, "Walkthrough") {
		walkRe := regexp.MustCompile(`(?is)Walkthrough\s*\n+(.+?)(\n\n|$)`)
		if match := walkRe.FindStringSubmatch(cleaned); len(match) > 1 {
			summaryParts = append(summaryParts, strings.TrimSpace(match[1]))
		}
	}

	// CodeRabbit: Look for potential issue
	if strings.Contains(cleaned, "Potential issue") {
		// Find the main recommendation after "Potential issue"
		issueRe := regexp.MustCompile(`(?is)Potential issue.*?\*\*([^*]+)\*\*`)
		if match := issueRe.FindStringSubmatch(cleaned); len(match) > 1 {
			summaryParts = append(summaryParts, "Issue: "+strings.TrimSpace(match[1]))
		}
	}

	// Generic: Look for summary/overview sections
	if len(summaryParts) == 0 {
		// Match "Summary:" or "Overview:" followed by content, but not "Summary of Changes" (Gemini heading)
		summaryRe := regexp.MustCompile(`(?i)(summary|overview)[:\s]+(?:of\s+changes\s*)?(.{10,300})`)
		if match := summaryRe.FindStringSubmatch(cleaned); len(match) > 2 {
			text := strings.TrimSpace(match[2])
			// Skip if it's just a Gemini heading remainder
			if !strings.HasPrefix(strings.ToLower(text), "of changes") {
				summaryParts = append(summaryParts, text)
			}
		}
	}

	// Fallback: first meaningful paragraph
	if len(summaryParts) == 0 {
		paragraphs := regexp.MustCompile(`\n\s*\n`).Split(cleaned, -1)
		for _, p := range paragraphs {
			p = strings.TrimSpace(p)
			// Skip headers, short lines, URLs, and metadata
			if len(p) > 30 && len(p) < 500 &&
				!strings.HasPrefix(p, "#") &&
				!strings.HasPrefix(p, "http") &&
				!strings.HasPrefix(p, "[") &&
				!strings.HasPrefix(p, "![") &&
				!strings.Contains(p, "```") {
				summaryParts = append(summaryParts, p)
				break
			}
		}
	}

	// Build summary
	summary := strings.Join(summaryParts, " | ")

	// Final fallback
	if summary == "" {
		// Take first 300 chars, avoiding URLs
		for _, line := range strings.Split(cleaned, "\n") {
			line = strings.TrimSpace(line)
			if len(line) > 20 && !strings.HasPrefix(line, "http") && !strings.HasPrefix(line, "[") {
				summary = line
				break
			}
		}
	}

	// Truncate if too long
	if len(summary) > 500 {
		if idx := strings.LastIndex(summary[:500], ". "); idx > 200 {
			summary = summary[:idx+1]
		} else if idx := strings.LastIndex(summary[:500], " | "); idx > 100 {
			summary = summary[:idx]
		} else {
			summary = summary[:497] + "..."
		}
	}

	return strings.TrimSpace(summary)
}

// htmlToMarkdown converts HTML elements to markdown equivalents while preserving structure
func htmlToMarkdown(body string) string {
	// Strip surrounding JSON quotes if present (bodies may be stored with JSON quoting)
	if len(body) >= 2 && body[0] == '"' && body[len(body)-1] == '"' {
		body = body[1 : len(body)-1]
	}

	// Replace escaped newlines and quotes (from JSON encoding)
	body = strings.ReplaceAll(body, "\\n", "\n")
	body = strings.ReplaceAll(body, "\\r", "")
	body = strings.ReplaceAll(body, "\\\"", "\"")

	// Remove HTML comments first
	commentRe := regexp.MustCompile(`<!--.*?-->`)
	body = commentRe.ReplaceAllString(body, "")

	// Handle nested tags: <a><strong>text</strong></a> -> extract text first
	// Convert links with nested content - capture everything between <a> and </a>
	nestedLinkRe := regexp.MustCompile(`(?is)<a\s+href=['"]([^'"]+)['"][^>]*>(.*?)</a>`)
	body = nestedLinkRe.ReplaceAllStringFunc(body, func(match string) string {
		parts := nestedLinkRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		url := parts[1]
		text := parts[2]
		// Strip any HTML tags from the link text
		text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
		if text == "" {
			return ""
		}
		return "[" + text + "](" + url + ")"
	})

	// Convert <strong> and <b> to **bold**
	strongRe := regexp.MustCompile(`(?is)<(?:strong|b)>(.*?)</(?:strong|b)>`)
	body = strongRe.ReplaceAllString(body, "**$1**")

	// Convert <em> and <i> to *italic*
	emRe := regexp.MustCompile(`(?is)<(?:em|i)>(.*?)</(?:em|i)>`)
	body = emRe.ReplaceAllString(body, "*$1*")

	// Convert <code> to `code`
	codeRe := regexp.MustCompile(`(?is)<code>(.*?)</code>`)
	body = codeRe.ReplaceAllString(body, "`$1`")

	// Convert <br> and <br/> to newlines
	brRe := regexp.MustCompile(`(?i)<br\s*/?>`)
	body = brRe.ReplaceAllString(body, "\n")

	// Extract content from <details><summary>...</summary>content</details>
	// Handle summary with nested HTML
	detailsRe := regexp.MustCompile(`(?is)<details>\s*<summary>(.*?)</summary>\s*(.*?)\s*</details>`)
	body = detailsRe.ReplaceAllStringFunc(body, func(match string) string {
		parts := detailsRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		title := parts[1]
		content := parts[2]
		// Clean up the title - remove HTML tags but keep markdown
		title = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(title, "")
		title = strings.TrimSpace(title)
		content = strings.TrimSpace(content)
		if title == "" {
			return content
		}
		return "\n**" + title + "**\n" + content + "\n"
	})

	// Remove remaining <summary> tags
	summaryTagRe := regexp.MustCompile(`(?i)</?summary>`)
	body = summaryTagRe.ReplaceAllString(body, "")

	// Convert simple tables: extract cell content
	tdRe := regexp.MustCompile(`(?is)<t[dh][^>]*>(.*?)</t[dh]>`)
	body = tdRe.ReplaceAllStringFunc(body, func(match string) string {
		content := tdRe.FindStringSubmatch(match)
		if len(content) > 1 {
			return strings.TrimSpace(content[1]) + "\n"
		}
		return ""
	})

	// Remove table structure tags
	tableTagsRe := regexp.MustCompile(`(?i)</?(?:table|tr|thead|tbody|tfoot)[^>]*>`)
	body = tableTagsRe.ReplaceAllString(body, "\n")

	// Remove any remaining HTML tags
	htmlRe := regexp.MustCompile(`<[^>]+>`)
	body = htmlRe.ReplaceAllString(body, "")

	// Clean up HTML entities
	body = strings.ReplaceAll(body, "&nbsp;", " ")
	body = strings.ReplaceAll(body, "&lt;", "<")
	body = strings.ReplaceAll(body, "&gt;", ">")
	body = strings.ReplaceAll(body, "&amp;", "&")
	body = strings.ReplaceAll(body, "&quot;", "\"")

	// Collapse multiple blank lines
	blankLinesRe := regexp.MustCompile(`\n{3,}`)
	body = blankLinesRe.ReplaceAllString(body, "\n\n")

	// Clean up lines - remove empty ones but preserve structure
	lines := strings.Split(body, "\n")
	var cleanedLines []string
	prevEmpty := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			if !prevEmpty {
				cleanedLines = append(cleanedLines, "")
				prevEmpty = true
			}
		} else {
			cleanedLines = append(cleanedLines, line)
			prevEmpty = false
		}
	}

	return strings.TrimSpace(strings.Join(cleanedLines, "\n"))
}

// detectRiskLevel analyzes the review body for risk indicators
func detectRiskLevel(data *AiReviewTaskData, body string) (string, int) {
	// Check patterns in order of severity
	if data.RiskHighPatternRegex != nil && data.RiskHighPatternRegex.MatchString(body) {
		return models.RiskLevelHigh, 80
	}
	if data.RiskMediumPatternRegex != nil && data.RiskMediumPatternRegex.MatchString(body) {
		return models.RiskLevelMedium, 50
	}
	if data.RiskLowPatternRegex != nil && data.RiskLowPatternRegex.MatchString(body) {
		return models.RiskLevelLow, 20
	}

	// Default to low risk if no patterns match
	return models.RiskLevelLow, 10
}

// detectReviewState determines the review outcome
func detectReviewState(body, status string) string {
	body = strings.ToLower(body)

	if strings.Contains(body, "approved") || strings.Contains(body, "lgtm") {
		return models.ReviewStateApproved
	}
	if strings.Contains(body, "changes requested") || strings.Contains(body, "request changes") {
		return models.ReviewStateChangesRequested
	}

	// Check status field
	switch strings.ToUpper(status) {
	case "APPROVED":
		return models.ReviewStateApproved
	case "CHANGES_REQUESTED":
		return models.ReviewStateChangesRequested
	}

	return models.ReviewStateCommented
}

// detectSourcePlatform determines if the PR is from GitHub or GitLab
func detectSourcePlatform(prId string) string {
	if strings.HasPrefix(prId, "github:") {
		return "github"
	}
	if strings.HasPrefix(prId, "gitlab:") {
		return "gitlab"
	}
	return "unknown"
}

// buildCommentUrl constructs a direct URL to the comment
// commentId format: "github:GithubPrComment:1:123456789" or "gitlab:GitlabMrComment:1:123456"
func buildCommentUrl(prUrl, commentId string) string {
	if prUrl == "" {
		return ""
	}

	// Extract the numeric comment ID from the DevLake ID
	parts := strings.Split(commentId, ":")
	if len(parts) < 4 {
		return prUrl
	}
	numericId := parts[len(parts)-1]

	// GitHub format: {pr_url}#issuecomment-{id}
	if strings.Contains(commentId, "github") || strings.Contains(commentId, "Github") {
		return prUrl + "#issuecomment-" + numericId
	}

	// GitLab format: {mr_url}#note_{id}
	if strings.Contains(commentId, "gitlab") || strings.Contains(commentId, "Gitlab") {
		return prUrl + "#note_" + numericId
	}

	return prUrl
}

// saveBatch saves a batch of AI reviews to the database
func saveBatch(db dal.Dal, batch []*models.AiReview) errors.Error {
	for _, review := range batch {
		err := db.CreateOrUpdate(review)
		if err != nil {
			return errors.Default.Wrap(err, "failed to save AI review")
		}
	}
	return nil
}
