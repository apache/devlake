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
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
)

var CalculateFailurePredictionsMeta = plugin.SubTaskMeta{
	Name:             "calculateFailurePredictions",
	EntryPoint:       CalculateFailurePredictions,
	EnabledByDefault: true,
	Description:      "Calculate AI failure prediction outcomes against actual CI test results",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAiReviewsMeta},
}

// prAiSummary holds the aggregated AI review outcome for one (PR, AI tool) pair.
type prAiSummary struct {
	PullRequestId  string
	PullRequestKey string
	RepoId         string
	RepoShortName  string
	RepoName       string
	AiTool         string
	MaxRiskScore   int
	CreatedDate    time.Time
	PrTitle        string
	PrUrl          string
	PrAuthor       string
	PrCreatedAt    time.Time
	Additions      int
	Deletions      int
}

// prCiKey identifies a PR in the ci_test_jobs table.
type prCiKey struct {
	PullRequestNumber string
	Repository        string
}

// CalculateFailurePredictions joins AI risk assessments with actual CI test outcomes.
//
// Algorithm:
//  1. Determine which CI source(s) to use from scope config (test_cases / job_result / both).
//  2. Load all AI-reviewed PRs for the repo, grouped by (PR, AI tool).
//  3. For each enabled source, load CI outcomes and persist one AiFailurePrediction
//     per (PR, AI tool, CI source).
func CalculateFailurePredictions(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	warningThreshold := data.Options.ScopeConfig.WarningThreshold
	if warningThreshold == 0 {
		warningThreshold = 50
	}

	ciFailureSource := data.Options.ScopeConfig.CiFailureSource
	if ciFailureSource == "" {
		ciFailureSource = models.CiSourceJobResult
	}

	excludeFlakyTests := data.Options.ScopeConfig.ExcludeFlakyTests
	excludeInfraFailures := data.Options.ScopeConfig.ExcludeInfraFailures

	sources := []string{ciFailureSource}
	if ciFailureSource == models.CiSourceBoth {
		sources = []string{models.CiSourceTestCases, models.CiSourceJobResult}
	}

	logger.Info("Calculating failure predictions for repo %s (warning_threshold=%d, ci_source=%s, exclude_flaky=%v, exclude_infra=%v)",
		data.Options.RepoId, warningThreshold, ciFailureSource, excludeFlakyTests, excludeInfraFailures)

	if excludeInfraFailures && ciFailureSource == models.CiSourceJobResult {
		logger.Info("Warning: excludeInfraFailures requires ci_test_cases to be populated. If test case data is not collected, all job failures will be treated as infrastructure failures and excluded.")
	}

	// Load AI-reviewed PR summaries (same for all sources).
	prSummaries, err := loadAiReviewPrSummaries(db, data.Options.RepoId, data.Options.ProjectName)
	if err != nil {
		return err
	}
	if len(prSummaries) == 0 {
		logger.Info("No AI-reviewed PRs found for repo %s", data.Options.RepoId)
		return nil
	}
	logger.Info("Loaded %d (PR, AI tool) pairs", len(prSummaries))

	repoShortNames := uniqueRepoShortNames(prSummaries)

	// Pre-build flaky sets only when the exclude_flaky_tests flag is enabled.
	var flakyTests map[prCiKey]bool
	var flakyJobs map[string]bool
	if excludeFlakyTests {
		for _, source := range sources {
			switch source {
			case models.CiSourceTestCases:
				if flakyTests == nil {
					flakyTests, err = buildFlakyTestSet(db)
					if err != nil {
						return err
					}
					logger.Info("Loaded %d flaky test entries", len(flakyTests))
				}
			case models.CiSourceJobResult:
				if flakyJobs == nil {
					flakyJobs, err = buildFlakyJobSet(db)
					if err != nil {
						return err
					}
					logger.Info("Loaded %d flaky job entries", len(flakyJobs))
				}
			}
		}
	} else {
		logger.Info("Flaky test/job filtering is disabled")
	}

	now := time.Now()
	totalWritten := 0

	// Track which (pullRequestId, aiTool) pairs matched CI data in at least one source.
	// Unmatched pairs get a NO_CI record so they are visible in drill-down dashboards.
	coveredKeys := make(map[string]bool, len(prSummaries))

	for _, source := range sources {
		var ciOutcomes map[prCiKey]ciOutcomeEntry
		switch source {
		case models.CiSourceTestCases:
			ciOutcomes, err = loadCiOutcomesByTestCases(db, repoShortNames, flakyTests)
		case models.CiSourceJobResult:
			ciOutcomes, err = loadCiOutcomesByJobResult(db, repoShortNames, flakyJobs, excludeInfraFailures)
		}
		if err != nil {
			return err
		}
		logger.Info("Source %s: loaded CI outcomes for %d (PR, repo) pairs", source, len(ciOutcomes))

		batch := make([]*models.AiFailurePrediction, 0, 100)
		writtenThisSource := 0
		for i := range prSummaries {
			ps := &prSummaries[i]
			ciKey := prCiKey{PullRequestNumber: ps.PullRequestKey, Repository: ps.RepoShortName}
			outcome, hasCiData := ciOutcomes[ciKey]

			if !hasCiData {
				continue
			}
			coveredKeys[ps.PullRequestId+":"+ps.AiTool] = true

			wasFlaggedRisky := ps.MaxRiskScore >= warningThreshold
			hadCiFailure := outcome.HadNonFlakyFailure

			batch = append(batch, &models.AiFailurePrediction{
				Id:                    generatePredictionId(ps.PullRequestId, ps.AiTool, source),
				PullRequestId:         ps.PullRequestId,
				PullRequestKey:        ps.PullRequestKey,
				RepoId:                ps.RepoId,
				RepoShortName:         ps.RepoShortName,
				RepoName:              ps.RepoName,
				AiTool:                ps.AiTool,
				CiFailureSource:       source,
				WasFlaggedRisky:       wasFlaggedRisky,
				RiskScore:             ps.MaxRiskScore,
				FlaggedAt:             ps.CreatedDate,
				HadCiFailure:          hadCiFailure,
				PredictionOutcome:     calculateOutcome(wasFlaggedRisky, hadCiFailure),
				PrTitle:               ps.PrTitle,
				PrUrl:                 ps.PrUrl,
				PrAuthor:              ps.PrAuthor,
				PrCreatedAt:           ps.PrCreatedAt,
				Additions:             ps.Additions,
				Deletions:             ps.Deletions,
				ObservationWindowDays: 0,
				CreatedAt:             now,
			})
			writtenThisSource++

			if len(batch) >= 100 {
				if saveErr := savePredictionsBatch(db, batch); saveErr != nil {
					return saveErr
				}
				batch = batch[:0]
			}
		}

		if len(batch) > 0 {
			if saveErr := savePredictionsBatch(db, batch); saveErr != nil {
				return saveErr
			}
		}
		totalWritten += writtenThisSource
	}

	// Write one NO_CI record per (PR, AI tool) pair that had no CI data in any source.
	// This makes unclassified PRs visible in drill-down dashboards.
	noCiBatch := make([]*models.AiFailurePrediction, 0, 100)
	for i := range prSummaries {
		ps := &prSummaries[i]
		if coveredKeys[ps.PullRequestId+":"+ps.AiTool] {
			continue
		}
		noCiBatch = append(noCiBatch, &models.AiFailurePrediction{
			Id:                    generatePredictionId(ps.PullRequestId, ps.AiTool, models.CiSourceNone),
			PullRequestId:         ps.PullRequestId,
			PullRequestKey:        ps.PullRequestKey,
			RepoId:                ps.RepoId,
			RepoShortName:         ps.RepoShortName,
			RepoName:              ps.RepoName,
			AiTool:                ps.AiTool,
			CiFailureSource:       models.CiSourceNone,
			WasFlaggedRisky:       ps.MaxRiskScore >= warningThreshold,
			RiskScore:             ps.MaxRiskScore,
			FlaggedAt:             ps.CreatedDate,
			HadCiFailure:          false,
			PredictionOutcome:     models.PredictionNoCi,
			PrTitle:               ps.PrTitle,
			PrUrl:                 ps.PrUrl,
			PrAuthor:              ps.PrAuthor,
			PrCreatedAt:           ps.PrCreatedAt,
			Additions:             ps.Additions,
			Deletions:             ps.Deletions,
			ObservationWindowDays: 0,
			CreatedAt:             now,
		})
		totalWritten++

		if len(noCiBatch) >= 100 {
			if saveErr := savePredictionsBatch(db, noCiBatch); saveErr != nil {
				return saveErr
			}
			noCiBatch = noCiBatch[:0]
		}
	}
	if len(noCiBatch) > 0 {
		if saveErr := savePredictionsBatch(db, noCiBatch); saveErr != nil {
			return saveErr
		}
	}

	logger.Info("Completed failure prediction calculation: %d predictions written (%d NO_CI)",
		totalWritten, len(noCiBatch))
	return nil
}

// buildFlakyTestSet returns a set of (testName, repository) pairs that failed
// on periodic or push runs in the last 30 days. Used by loadCiOutcomesByTestCases
// to exclude environment-flaky test failures from PR outcome determination.
func buildFlakyTestSet(db dal.Dal) (map[prCiKey]bool, errors.Error) {
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	var rows []struct {
		Name       string `gorm:"column:name"`
		Repository string `gorm:"column:repository"`
	}

	err := db.All(&rows,
		dal.Select("DISTINCT tc.name, j.repository"),
		dal.From("ci_test_cases tc"),
		dal.Join("JOIN ci_test_jobs j ON tc.connection_id = j.connection_id AND tc.job_id = j.job_id"),
		dal.Where("tc.status = 'failed' AND j.trigger_type IN ('periodic', 'push') AND j.finished_at >= ?", thirtyDaysAgo),
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to build flaky test set")
	}

	flakyTests := make(map[prCiKey]bool, len(rows))
	for _, r := range rows {
		flakyTests[prCiKey{PullRequestNumber: r.Name, Repository: r.Repository}] = true
	}
	return flakyTests, nil
}

// buildFlakyJobSet returns a set of "job_name|repository" keys for CI jobs that
// failed on periodic or push runs in the last 30 days. Used by loadCiOutcomesByJobResult
// to exclude environment-flaky job failures from PR outcome determination.
func buildFlakyJobSet(db dal.Dal) (map[string]bool, errors.Error) {
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	var rows []struct {
		JobName    string `gorm:"column:job_name"`
		Repository string `gorm:"column:repository"`
	}

	err := db.All(&rows,
		dal.Select("DISTINCT job_name, repository"),
		dal.From("ci_test_jobs"),
		dal.Where("result != 'SUCCESS' AND trigger_type IN ('periodic', 'push') AND finished_at >= ?", thirtyDaysAgo),
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to build flaky job set")
	}

	flakyJobs := make(map[string]bool, len(rows))
	for _, r := range rows {
		flakyJobs[r.JobName+"|"+r.Repository] = true
	}
	return flakyJobs, nil
}

// loadAiReviewPrSummaries returns one row per (pull_request_id, ai_tool) with
// the max risk_score and the PR key / repo short name needed to join CI data.
// Supports both single-repo mode (repoId set) and project mode (projectName set).
func loadAiReviewPrSummaries(db dal.Dal, repoId, projectName string) ([]prAiSummary, errors.Error) {
	var rows []struct {
		PullRequestId  string    `gorm:"column:pull_request_id"`
		PullRequestKey string    `gorm:"column:pull_request_key"`
		RepoId         string    `gorm:"column:repo_id"`
		RepoName       string    `gorm:"column:repo_name"`
		AiTool         string    `gorm:"column:ai_tool"`
		MaxRiskScore   int       `gorm:"column:max_risk_score"`
		CreatedDate    time.Time `gorm:"column:created_date"`
		PrTitle        string    `gorm:"column:pr_title"`
		PrUrl          string    `gorm:"column:pr_url"`
		PrAuthor       string    `gorm:"column:pr_author"`
		PrCreatedAt    time.Time `gorm:"column:pr_created_at"`
		Additions      int       `gorm:"column:additions"`
		Deletions      int       `gorm:"column:deletions"`
	}

	const selectCols = "ar.pull_request_id, pr.pull_request_key, ar.repo_id, r.name AS repo_name, ar.ai_tool," +
		" MAX(ar.risk_score) AS max_risk_score, MIN(ar.created_date) AS created_date," +
		" MAX(pr.title) AS pr_title, MAX(pr.url) AS pr_url, MAX(pr.author_name) AS pr_author," +
		" MAX(pr.created_date) AS pr_created_at, MAX(pr.additions) AS additions, MAX(pr.deletions) AS deletions"

	var clauses []dal.Clause
	if repoId != "" {
		clauses = []dal.Clause{
			dal.Select(selectCols),
			dal.From("_tool_aireview_reviews ar"),
			dal.Join("JOIN pull_requests pr ON ar.pull_request_id = pr.id"),
			dal.Join("JOIN repos r ON ar.repo_id = r.id"),
			dal.Where("ar.repo_id = ? AND ar.body NOT LIKE '%Review skipped%'", repoId),
			dal.Groupby("ar.pull_request_id, pr.pull_request_key, ar.repo_id, r.name, ar.ai_tool"),
		}
	} else {
		clauses = []dal.Clause{
			dal.Select(selectCols),
			dal.From("_tool_aireview_reviews ar"),
			dal.Join("JOIN pull_requests pr ON ar.pull_request_id = pr.id"),
			dal.Join("JOIN repos r ON ar.repo_id = r.id"),
			dal.Join("JOIN project_mapping pm ON ar.repo_id = pm.row_id AND pm.`table` = 'repos'"),
			dal.Where("pm.project_name = ? AND ar.body NOT LIKE '%Review skipped%'", projectName),
			dal.Groupby("ar.pull_request_id, pr.pull_request_key, ar.repo_id, r.name, ar.ai_tool"),
		}
	}

	err := db.All(&rows, clauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to load AI review PR summaries")
	}

	summaries := make([]prAiSummary, len(rows))
	for i, r := range rows {
		summaries[i] = prAiSummary{
			PullRequestId:  r.PullRequestId,
			PullRequestKey: r.PullRequestKey,
			RepoId:         r.RepoId,
			RepoShortName:  repoShortNameFrom(r.RepoName),
			RepoName:       r.RepoName,
			AiTool:         r.AiTool,
			MaxRiskScore:   r.MaxRiskScore,
			CreatedDate:    r.CreatedDate,
			PrTitle:        r.PrTitle,
			PrUrl:          r.PrUrl,
			PrAuthor:       r.PrAuthor,
			PrCreatedAt:    r.PrCreatedAt,
			Additions:      r.Additions,
			Deletions:      r.Deletions,
		}
	}
	return summaries, nil
}

// ciOutcomeEntry records whether a PR had at least one non-flaky CI failure.
type ciOutcomeEntry struct {
	HadNonFlakyFailure bool
}

// loadCiOutcomesByTestCases joins ci_test_jobs with ci_test_cases and returns
// a map indicating whether each PR had a non-flaky test-case-level failure.
// Requires ci_test_cases to be populated (needs full testregistry collection).
func loadCiOutcomesByTestCases(db dal.Dal, repoShortNames []string, flakyTests map[prCiKey]bool) (map[prCiKey]ciOutcomeEntry, errors.Error) {
	if len(repoShortNames) == 0 {
		return map[prCiKey]ciOutcomeEntry{}, nil
	}

	var rows []struct {
		PullRequestNumber int64  `gorm:"column:pull_request_number"`
		Repository        string `gorm:"column:repository"`
		TestName          string `gorm:"column:test_name"`
		Status            string `gorm:"column:status"`
	}

	err := db.All(&rows,
		dal.Select("j.pull_request_number, j.repository, tc.name AS test_name, tc.status"),
		dal.From("ci_test_jobs j"),
		dal.Join("JOIN ci_test_cases tc ON j.connection_id = tc.connection_id AND j.job_id = tc.job_id"),
		dal.Where("j.trigger_type = 'pull_request' AND j.pull_request_number > 0 AND j.repository IN ? AND j.finished_at >= ?", repoShortNames, time.Now().AddDate(0, -3, 0)),
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to load CI test case outcomes")
	}

	outcomes := make(map[prCiKey]ciOutcomeEntry)
	for _, r := range rows {
		key := prCiKey{
			PullRequestNumber: strconv.FormatInt(r.PullRequestNumber, 10),
			Repository:        r.Repository,
		}
		if _, exists := outcomes[key]; !exists {
			outcomes[key] = ciOutcomeEntry{}
		}
		if r.Status != "failed" {
			continue
		}
		// Check if this failed test is flaky.
		flakyKey := prCiKey{PullRequestNumber: r.TestName, Repository: r.Repository}
		if flakyTests[flakyKey] {
			continue
		}
		entry := outcomes[key]
		entry.HadNonFlakyFailure = true
		outcomes[key] = entry
	}
	return outcomes, nil
}

// ciJobRow is one row from the ci_test_jobs table.
type ciJobRow struct {
	PullRequestNumber int64  `gorm:"column:pull_request_number"`
	Repository        string `gorm:"column:repository"`
	JobId             string `gorm:"column:job_id"`
	JobName           string `gorm:"column:job_name"`
	Result            string `gorm:"column:result"`
}

// loadCiOutcomesByJobResult queries ci_test_jobs.result for PR-triggered jobs
// and returns a map indicating whether each PR had a non-flaky job-level failure.
// When excludeInfraFailures is true, individual job runs that failed but have no
// failed test cases are considered infrastructure failures and excluded.
// Works without ci_test_cases being populated (unless excludeInfraFailures is enabled).
func loadCiOutcomesByJobResult(db dal.Dal, repoShortNames []string, flakyJobs map[string]bool, excludeInfraFailures bool) (map[prCiKey]ciOutcomeEntry, errors.Error) {
	if len(repoShortNames) == 0 {
		return map[prCiKey]ciOutcomeEntry{}, nil
	}

	var rows []ciJobRow
	err := db.All(&rows,
		dal.Select("pull_request_number, repository, job_id, job_name, result"),
		dal.From("ci_test_jobs"),
		dal.Where("trigger_type = 'pull_request' AND pull_request_number > 0 AND repository IN ? AND finished_at >= ?", repoShortNames, time.Now().AddDate(0, -3, 0)),
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to load CI job outcomes")
	}

	// When infra filtering is enabled, pre-load individual job run IDs that have
	// at least one failed test case. This allows us to distinguish infra failures
	// (job failed but no tests failed) from real test failures on a per-run basis.
	var jobRunsWithTestFailures map[string]bool
	if excludeInfraFailures {
		jobRunsWithTestFailures, err = buildJobRunsWithTestFailures(db, repoShortNames)
		if err != nil {
			return nil, err
		}
	}

	outcomes := make(map[prCiKey]ciOutcomeEntry)
	for _, r := range rows {
		key := prCiKey{
			PullRequestNumber: strconv.FormatInt(r.PullRequestNumber, 10),
			Repository:        r.Repository,
		}
		if _, exists := outcomes[key]; !exists {
			outcomes[key] = ciOutcomeEntry{}
		}
		if r.Result == "SUCCESS" {
			continue
		}
		// Check if this job is known to be flaky on non-PR runs.
		flakyKey := r.JobName + "|" + r.Repository
		if flakyJobs[flakyKey] {
			continue
		}
		// Check if this specific job run is an infrastructure failure
		// (failed but no test cases actually failed in this run).
		if excludeInfraFailures && !jobRunsWithTestFailures[r.JobId] {
			continue
		}
		entry := outcomes[key]
		entry.HadNonFlakyFailure = true
		outcomes[key] = entry
	}
	return outcomes, nil
}

// buildJobRunsWithTestFailures returns a set of job_id values for PR-triggered
// CI job runs that have at least one failed test case. Used to identify individual
// infrastructure failures — a specific job run that failed but had no test failures.
func buildJobRunsWithTestFailures(db dal.Dal, repoShortNames []string) (map[string]bool, errors.Error) {
	var rows []struct {
		JobId string `gorm:"column:job_id"`
	}

	err := db.All(&rows,
		dal.Select("DISTINCT j.job_id"),
		dal.From("ci_test_jobs j"),
		dal.Join("JOIN ci_test_cases tc ON j.connection_id = tc.connection_id AND j.job_id = tc.job_id"),
		dal.Where("j.trigger_type = 'pull_request' AND j.pull_request_number > 0 AND j.repository IN ? AND tc.status = 'failed' AND j.finished_at >= ?", repoShortNames, time.Now().AddDate(0, -3, 0)),
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to build job runs with test failures set")
	}

	result := make(map[string]bool, len(rows))
	for _, r := range rows {
		result[r.JobId] = true
	}
	return result, nil
}

// uniqueRepoShortNames returns the distinct repo short names from the summaries.
func uniqueRepoShortNames(summaries []prAiSummary) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range summaries {
		if s.RepoShortName != "" && !seen[s.RepoShortName] {
			seen[s.RepoShortName] = true
			result = append(result, s.RepoShortName)
		}
	}
	return result
}

// calculateOutcome determines the confusion-matrix label for a prediction.
func calculateOutcome(wasFlaggedRisky, actualFailure bool) string {
	if wasFlaggedRisky && actualFailure {
		return models.PredictionTP
	}
	if wasFlaggedRisky && !actualFailure {
		return models.PredictionFP
	}
	if !wasFlaggedRisky && actualFailure {
		return models.PredictionFN
	}
	return models.PredictionTN
}

// generatePredictionId creates a deterministic ID for a prediction.
func generatePredictionId(prId, aiTool, ciFailureSource string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", prId, aiTool, ciFailureSource)))
	return "aipred:" + hex.EncodeToString(hash[:16])
}

// repoShortNameFrom extracts the repository short name (the part after the last "/")
// from a full "org/repo" name. This avoids MySQL-specific SUBSTRING_INDEX in SQL.
func repoShortNameFrom(fullName string) string {
	if i := strings.LastIndex(fullName, "/"); i >= 0 {
		return fullName[i+1:]
	}
	return fullName
}

// savePredictionsBatch upserts a batch of predictions.
func savePredictionsBatch(db dal.Dal, batch []*models.AiFailurePrediction) errors.Error {
	for _, p := range batch {
		if err := db.CreateOrUpdate(p); err != nil {
			return errors.Default.Wrap(err, "failed to save failure prediction")
		}
	}
	return nil
}
