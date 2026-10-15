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
	"math"
	"reflect"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
)

// CalculateChangeLeadTimeMeta contains metadata for the CalculateChangeLeadTime subtask.
var CalculateChangeLeadTimeMeta = plugin.SubTaskMeta{
	Name:             "calculateChangeLeadTime",
	EntryPoint:       CalculateChangeLeadTime,
	EnabledByDefault: true,
	Description:      "Calculate change lead time",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD, plugin.DOMAIN_TYPE_CODE},
}

// CalculateChangeLeadTime calculates change lead time for a project.
func CalculateChangeLeadTime(taskCtx plugin.SubTaskContext) errors.Error {
	// Get instances of the DAL and logger
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*DoraTaskData)
	// Clear previous results from the project
	err := db.Exec("DELETE FROM project_pr_metrics WHERE project_name = ? ", data.Options.ProjectName)
	if err != nil {
		return errors.Default.Wrap(err, "error deleting previous project_pr_metrics")
	}

	// Batch fetch all required data upfront for better performance
	startTime := time.Now()
	logger.Info("Batch fetching data for project: %s", data.Options.ProjectName)

	firstCommitsMap, err := batchFetchFirstCommits(data.Options.ProjectName, db)
	if err != nil {
		return errors.Default.Wrap(err, "failed to batch fetch first commits")
	}
	logger.Info("Fetched %d first commits in %v", len(firstCommitsMap), time.Since(startTime))

	reviewStartTime := time.Now()
	firstReviewsMap, err := batchFetchFirstReviews(data.Options.ProjectName, db)
	if err != nil {
		return errors.Default.Wrap(err, "failed to batch fetch first reviews")
	}
	logger.Info("Fetched %d first reviews in %v", len(firstReviewsMap), time.Since(reviewStartTime))

	deploymentStartTime := time.Now()
	deploymentsMap, err := batchFetchDeployments(data.Options.ProjectName, db)
	if err != nil {
		return errors.Default.Wrap(err, "failed to batch fetch deployments")
	}
	logger.Info("Fetched %d deployments in %v", len(deploymentsMap), time.Since(deploymentStartTime))
	logger.Info("Total batch fetch time: %v", time.Since(startTime))

	// Get pull requests by repo project_name
	var clauses = []dal.Clause{
		dal.Select("pr.id, pr.pull_request_key, pr.author_id, pr.merge_commit_sha, pr.created_date, pr.merged_date, pr.head_ref"),
		dal.From("pull_requests pr"),
		dal.Join(`LEFT JOIN project_mapping pm ON (pm.row_id = pr.base_repo_id)`),
		dal.Where("pr.merged_date IS NOT NULL AND pm.project_name = ? AND pm.table = 'repos'", data.Options.ProjectName),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	var total, excluded int
	converter, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			// table and params are essential for deleting data from the target table
			Params: DoraApiParams{
				ProjectName: data.Options.ProjectName,
			},
			Table: "pull_requests",
		},
		BatchSize:    100,
		InputRowType: reflect.TypeOf(code.PullRequest{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			pr := inputRow.(*code.PullRequest)
			total++
			if !MatchPrHeadRef(data.PrHeadRefRegexp, pr.HeadRef) {
				excluded++
				return nil, nil
			}
			// Initialize a new ProjectPrMetric
			projectPrMetric := &crossdomain.ProjectPrMetric{}
			projectPrMetric.Id = pr.Id
			projectPrMetric.ProjectName = data.Options.ProjectName

			// Get the first commit for the PR from batch-fetched map
			firstCommit := firstCommitsMap[pr.Id]
			// Calculate PR coding time
			if firstCommit != nil {
				projectPrMetric.PrCodingTime = computeTimeSpan(&firstCommit.CommitAuthoredDate, &pr.CreatedDate)
				projectPrMetric.FirstCommitSha = firstCommit.CommitSha
				projectPrMetric.FirstCommitAuthoredDate = &firstCommit.CommitAuthoredDate
			}

			// Get the first review for the PR from batch-fetched map
			firstReview := firstReviewsMap[pr.Id]
			// Calculate PR pickup time and PR review time
			prDuring := computeTimeSpan(&pr.CreatedDate, pr.MergedDate)
			if firstReview != nil {
				projectPrMetric.PrPickupTime = computeTimeSpan(&pr.CreatedDate, &firstReview.CreatedDate)
				projectPrMetric.PrReviewTime = computeTimeSpan(&firstReview.CreatedDate, pr.MergedDate)
				projectPrMetric.FirstReviewId = firstReview.Id
				projectPrMetric.FirstCommentDate = &firstReview.CreatedDate
			}

			projectPrMetric.PrCreatedDate = &pr.CreatedDate
			projectPrMetric.PrMergedDate = pr.MergedDate

			// Get the deployment for the PR from batch-fetched map
			deployment := deploymentsMap[pr.MergeCommitSha]

			// Calculate PR deploy time
			if deployment != nil && deployment.FinishedDate != nil {
				projectPrMetric.PrDeployTime = computeTimeSpan(pr.MergedDate, deployment.FinishedDate)
				projectPrMetric.DeploymentCommitId = deployment.Id
				projectPrMetric.PrDeployedDate = deployment.FinishedDate
			} else {
				logger.Debug("deploy time of pr %v is nil\n", pr.PullRequestKey)
			}

			// Calculate PR cycle time
			var cycleTime int64
			if projectPrMetric.PrCodingTime != nil {
				cycleTime += *projectPrMetric.PrCodingTime
			}
			if prDuring != nil {
				cycleTime += *prDuring
			}
			if projectPrMetric.PrDeployTime != nil {
				cycleTime += *projectPrMetric.PrDeployTime
			}
			projectPrMetric.PrCycleTime = &cycleTime

			// Return the projectPrMetric
			return []interface{}{projectPrMetric}, nil
		},
	})
	if err != nil {
		return err
	}
	// Execute the data converter
	if err := converter.Execute(); err != nil {
		return err
	}
	if data.PrHeadRefRegexp != nil {
		logger.Info("prHeadRefPattern %q excluded %d of %d merged pull requests", data.PrHeadRefRegexp.String(), excluded, total)
	}
	return nil
}

func computeTimeSpan(start, end *time.Time) *int64 {
	if start == nil || end == nil {
		return nil
	}
	span := end.Sub(*start)
	minutes := int64(math.Ceil(span.Minutes()))
	if minutes < 0 {
		return nil
	}
	return &minutes
}

// deploymentCommitWithMergeSha is a helper struct to capture both the deployment commit
// and the associated merge_sha from the commits_diffs join query.
type deploymentCommitWithMergeSha struct {
	devops.CicdDeploymentCommit
	MergeSha string `gorm:"column:merge_sha"`
}

// batchFetchFirstCommits retrieves the first commit for all pull requests in the given project.
// Returns a map indexed by PR ID for O(1) lookup performance.
//
// The query uses a subquery to find the minimum commit_authored_date for each PR,
// then joins back to get the full commit record. This is more efficient than
// fetching all commits and filtering in memory.
func batchFetchFirstCommits(projectName string, db dal.Dal) (map[string]*code.PullRequestCommit, errors.Error) {
	var results []*code.PullRequestCommit

	// Use a subquery to find the earliest commit for each PR, then join to get full commit details.
	// This avoids scanning all commits and is optimized by the database engine.
	err := db.All(
		&results,
		dal.Select("prc.*"),
		dal.From("pull_request_commits prc"),
		dal.Join(`INNER JOIN (
			SELECT pull_request_id, MIN(commit_authored_date) as min_date
			FROM pull_request_commits
			GROUP BY pull_request_id
		) first_commits ON prc.pull_request_id = first_commits.pull_request_id
		AND prc.commit_authored_date = first_commits.min_date`),
		dal.Join("INNER JOIN pull_requests pr ON pr.id = prc.pull_request_id"),
		dal.Join("LEFT JOIN project_mapping pm ON pm.row_id = pr.base_repo_id AND pm.table = 'repos'"),
		dal.Where("pm.project_name = ?", projectName),
		dal.Orderby("prc.pull_request_id, prc.commit_authored_date ASC"),
	)

	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to batch fetch first commits")
	}

	// Build the map for O(1) lookup by PR ID
	commitMap := make(map[string]*code.PullRequestCommit, len(results))
	for _, commit := range results {
		// Only keep the first commit if multiple commits have the same timestamp
		if _, exists := commitMap[commit.PullRequestId]; !exists {
			commitMap[commit.PullRequestId] = commit
		}
	}

	return commitMap, nil
}

// batchFetchFirstReviews retrieves the first review comment for all pull requests in the given project.
// Returns a map indexed by PR ID for O(1) lookup performance.
//
// The query uses a subquery to find the minimum created_date for each PR (excluding the PR author),
// then joins back to get the full comment record.
func batchFetchFirstReviews(projectName string, db dal.Dal) (map[string]*code.PullRequestComment, errors.Error) {
	var results []*code.PullRequestComment

	// Use a subquery to find the earliest review comment for each PR (excluding author's comments),
	// then join to get full comment details.
	err := db.All(
		&results,
		dal.Select("prc.*"),
		dal.From("pull_request_comments prc"),
		dal.Join(`INNER JOIN (
			SELECT prc2.pull_request_id, MIN(prc2.created_date) as min_date
			FROM pull_request_comments prc2
			INNER JOIN pull_requests pr2 ON pr2.id = prc2.pull_request_id
			LEFT JOIN accounts acc2 ON acc2.id = prc2.account_id
			WHERE (pr2.author_id IS NULL OR pr2.author_id = '' OR prc2.account_id != pr2.author_id)
			AND COALESCE(acc2.is_bot, false) = false
			GROUP BY prc2.pull_request_id
		) first_reviews ON prc.pull_request_id = first_reviews.pull_request_id
		AND prc.created_date = first_reviews.min_date`),
		dal.Join("INNER JOIN pull_requests pr ON pr.id = prc.pull_request_id"),
		dal.Join("LEFT JOIN project_mapping pm ON pm.row_id = pr.base_repo_id AND pm.table = 'repos'"),
		dal.Where("pm.project_name = ? AND (pr.author_id IS NULL OR pr.author_id = '' OR prc.account_id != pr.author_id)", projectName),
		dal.Orderby("prc.pull_request_id, prc.created_date ASC"),
	)

	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to batch fetch first reviews")
	}

	// Build the map for O(1) lookup by PR ID
	reviewMap := make(map[string]*code.PullRequestComment, len(results))
	for _, review := range results {
		// Only keep the first review if multiple reviews have the same timestamp
		if _, exists := reviewMap[review.PullRequestId]; !exists {
			reviewMap[review.PullRequestId] = review
		}
	}

	return reviewMap, nil
}

// batchFetchDeployments retrieves deployment commits for all merge commits in the given project.
// Returns a map indexed by merge commit SHA for O(1) lookup performance.
//
// Uses a two-phase strategy to avoid the "first deployment over-mapping" problem:
//
// Phase 1 - Direct match: find successful PRODUCTION deployments whose commit_sha
// directly equals a PR's merge_commit_sha. Safe even for the very first deployment.
//
// Phase 2 - Diff-based fallback: use the commits_diffs join strategy, but deliberately
// skip the first deployment (prev_success_deployment_commit_id == "") to avoid over-mapping.
func batchFetchDeployments(projectName string, db dal.Dal) (map[string]*devops.CicdDeploymentCommit, errors.Error) {
	deploymentMap := make(map[string]*devops.CicdDeploymentCommit)
	var directResults []*devops.CicdDeploymentCommit
	err := db.All(
		&directResults,
		dal.Select("dc.*"),
		dal.From("cicd_deployment_commits dc"),
		dal.Join("LEFT JOIN project_mapping pm ON pm.table = 'cicd_scopes' AND pm.row_id = dc.cicd_scope_id"),
		dal.Where("dc.environment = 'PRODUCTION'"), // TODO: remove when multi-environment is supported
		dal.Where("dc.result = ? AND pm.project_name = ?", devops.RESULT_SUCCESS, projectName),
		dal.Orderby("dc.started_date ASC, dc.id ASC"),
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to batch fetch direct deployments")
	}
	for _, dc := range directResults {
		if _, exists := deploymentMap[dc.CommitSha]; !exists {
			deploymentCopy := *dc
			deploymentMap[dc.CommitSha] = &deploymentCopy
		}
	}
	var diffResults []*deploymentCommitWithMergeSha
	err = db.All(
		&diffResults,
		dal.Select("dc.*, cd.commit_sha as merge_sha"),
		dal.From("cicd_deployment_commits dc"),
		dal.Join("LEFT JOIN cicd_deployment_commits p ON dc.prev_success_deployment_commit_id = p.id"),
		dal.Join("INNER JOIN commits_diffs cd ON cd.new_commit_sha = dc.commit_sha AND cd.old_commit_sha = COALESCE(p.commit_sha, '')"),
		dal.Join("LEFT JOIN project_mapping pm ON pm.table = 'cicd_scopes' AND pm.row_id = dc.cicd_scope_id"),
		dal.Where("dc.prev_success_deployment_commit_id <> ''"),
		dal.Where("dc.environment = 'PRODUCTION'"), // TODO: remove when multi-environment is supported
		dal.Where("dc.result = ? AND pm.project_name = ?", devops.RESULT_SUCCESS, projectName),
		dal.Orderby("cd.commit_sha, dc.started_date ASC, dc.id ASC"),
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to batch fetch diff-based deployments")
	}
	for _, result := range diffResults {
		if _, exists := deploymentMap[result.MergeSha]; !exists {
			deploymentCopy := result.CicdDeploymentCommit
			deploymentMap[result.MergeSha] = &deploymentCopy
		}
	}
	return deploymentMap, nil
}
