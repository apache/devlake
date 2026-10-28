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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/pkg/gcshelper"
)

var FetchMissingCiJobsMeta = plugin.SubTaskMeta{
	Name:             "fetchMissingCiJobs",
	EntryPoint:       FetchMissingCiJobs,
	EnabledByDefault: false,
	Description:      "Backfill CI job results from GCS for PRs that have AI reviews but no CI outcome data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD, plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAiReviewsMeta},
}

// ciBackfillJobRow writes the minimal subset of ci_test_jobs columns required
// by calculateFailurePredictions. Using a local struct avoids importing the
// testregistry plugin (which would create a circular dependency).
// connection_id=0 is the sentinel used for aireview-backfilled rows.
type ciBackfillJobRow struct {
	ConnectionId      int64      `gorm:"primaryKey;column:connection_id"`
	JobId             string     `gorm:"primaryKey;column:job_id"`
	JobName           string     `gorm:"column:job_name"`
	Repository        string     `gorm:"column:repository"`
	PullRequestNumber *int64     `gorm:"column:pull_request_number"`
	TriggerType       string     `gorm:"column:trigger_type"`
	Result            string     `gorm:"column:result"`
	FinishedAt        *time.Time `gorm:"column:finished_at"`
	DurationSec       *float64   `gorm:"column:duration_sec"`
}

func (ciBackfillJobRow) TableName() string { return "ci_test_jobs" }

// missingPR holds the minimum information needed to query GCS for a single PR.
type missingPR struct {
	PullRequestKey string
	OrgName        string
	RepoShortName  string
	RepoFullName   string
}

// FetchMissingCiJobs queries for PRs that have AI reviews but no associated
// CI job rows, then fetches their build results directly from the Openshift CI
// GCS bucket. This is much faster than scanning all GCS directories because we
// look up only the PR numbers we already know about.
func FetchMissingCiJobs(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	cfg := data.Options.ScopeConfig
	if cfg == nil || cfg.CiBackfillDays <= 0 {
		logger.Debug("CI backfill disabled (ciBackfillDays=0), skipping fetchMissingCiJobs")
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -cfg.CiBackfillDays)

	// ---- 1. Find PRs with AI reviews but no CI job rows ----
	// aireview is a metric plugin — it has no per-repo scope, so we query all
	// repos that have AI reviews (or just the one repo if RepoId is set).
	repoId := data.Options.RepoId
	var repoIds []string
	if repoId != "" {
		repoIds = []string{repoId}
	} else {
		var repoRows []struct {
			RepoId string `gorm:"column:repo_id"`
		}
		if dbErr := db.All(&repoRows, dal.Select("DISTINCT repo_id"), dal.From("_tool_aireview_reviews")); dbErr != nil {
			return errors.Default.Wrap(dbErr, "querying distinct repo IDs from aireview reviews")
		}
		for _, r := range repoRows {
			repoIds = append(repoIds, r.RepoId)
		}
	}
	if len(repoIds) == 0 {
		logger.Info("No repos with AI reviews found, nothing to backfill")
		return nil
	}

	var missing []missingPR
	for _, rid := range repoIds {
		prs, findErr := findMissingPRs(db, rid, cutoff)
		if findErr != nil {
			return errors.Default.Wrap(findErr, "querying missing CI PRs for repo "+rid)
		}
		missing = append(missing, prs...)
	}
	if len(missing) == 0 {
		logger.Info("No PRs missing CI data, nothing to backfill")
		return nil
	}
	logger.Info("Found %d PRs with AI reviews but no CI data, fetching from GCS", len(missing))

	// ---- 2. Set up GCS store (real or injected fake for tests) ----
	var store gcshelper.HistoryStore
	if data.GcsStoreOverride != nil {
		store = data.GcsStoreOverride
	} else {
		ctx := context.Background()
		bucket, gcsErr := gcshelper.New(ctx, gcshelper.OpenshiftCIBucketName)
		if gcsErr != nil {
			return errors.Default.Wrap(gcsErr, "creating GCS client")
		}
		defer bucket.Close()
		store = bucket
	}

	// ---- 3. Fetch each PR from GCS ----
	var rows []ciBackfillJobRow
	ctx := context.Background()

	taskCtx.SetProgress(0, len(missing))
	for i, pr := range missing {
		prNum, convErr := strconv.ParseInt(pr.PullRequestKey, 10, 64)
		if convErr != nil {
			logger.Warn(nil, "Skipping non-numeric PR key %q", pr.PullRequestKey)
			taskCtx.IncProgress(1)
			continue
		}

		fetched, fetchErr := fetchPRBuilds(ctx, store, pr, prNum, cutoff, logger)
		if fetchErr != nil {
			logger.Warn(errors.Default.WrapRaw(fetchErr),
				"Failed to fetch GCS builds for %s/%s PR %d",
				pr.OrgName, pr.RepoShortName, prNum)
		}
		rows = append(rows, fetched...)
		taskCtx.SetProgress(i+1, len(missing))
	}

	if len(rows) == 0 {
		logger.Info("No CI job rows fetched from GCS")
		return nil
	}

	// ---- 4. Upsert into ci_test_jobs ----
	if err := db.CreateOrUpdate(rows); err != nil {
		return errors.Default.Wrap(err, "upserting ci_test_jobs from GCS backfill")
	}
	logger.Info("Backfilled %d CI job rows from GCS", len(rows))
	return nil
}

// missingPRRow is used as the query target for findMissingPRs.
type missingPRRow struct {
	PullRequestKey string `gorm:"column:pull_request_key"`
	RepoName       string `gorm:"column:repo_name"`
}

func (missingPRRow) TableName() string { return "_tool_aireview_reviews" }

// findMissingPRs returns PRs in the given repo that have at least one non-skipped
// AI review but no matching row in ci_test_jobs for a pull_request trigger.
func findMissingPRs(db dal.Dal, repoId string, cutoff time.Time) ([]missingPR, error) {
	// Pre-compute the short repo name (part after the last "/") to avoid
	// MySQL-specific SUBSTRING_INDEX inside the SQL subquery.
	var repoRows []struct {
		Name string `gorm:"column:name"`
	}
	if dbErr := db.All(&repoRows, dal.Select("name"), dal.From("repos"), dal.Where("id = ?", repoId), dal.Limit(1)); dbErr != nil {
		return nil, fmt.Errorf("querying repo name for %s: %w", repoId, dbErr)
	}
	if len(repoRows) == 0 {
		return nil, fmt.Errorf("repo %s not found", repoId)
	}
	repoShort := repoRows[0].Name
	if i := strings.LastIndex(repoShort, "/"); i >= 0 {
		repoShort = repoShort[i+1:]
	}

	var rawRows []missingPRRow
	err := db.All(&rawRows,
		dal.Select("DISTINCT pr.pull_request_key, r.name AS repo_name"),
		dal.From("_tool_aireview_reviews ar"),
		dal.Join("JOIN pull_requests pr ON ar.pull_request_id = pr.id"),
		dal.Join("JOIN repos r ON ar.repo_id = r.id"),
		dal.Where(`ar.repo_id = ?
			AND ar.body NOT LIKE '%Review skipped%'
			AND pr.created_date >= ?
			AND NOT EXISTS (
				SELECT 1 FROM ci_test_jobs j
				WHERE CAST(j.pull_request_number AS CHAR) = pr.pull_request_key
				  AND j.repository = ?
				  AND j.trigger_type = 'pull_request'
			)`, repoId, cutoff, repoShort),
	)
	if err != nil {
		return nil, fmt.Errorf("querying missing PRs: %w", err)
	}

	result := make([]missingPR, 0, len(rawRows))
	for _, r := range rawRows {
		parts := strings.SplitN(r.RepoName, "/", 2)
		org, repo := "", r.RepoName
		if len(parts) == 2 {
			org, repo = parts[0], parts[1]
		}
		result = append(result, missingPR{
			PullRequestKey: r.PullRequestKey,
			OrgName:        org,
			RepoShortName:  repo,
			RepoFullName:   r.RepoName,
		})
	}
	return result, nil
}

// fetchPRBuilds queries GCS for all build results for a single PR number and
// returns them as ciBackfillJobRow records ready for upsert.
func fetchPRBuilds(
	ctx context.Context,
	store gcshelper.HistoryStore,
	pr missingPR,
	prNum int64,
	cutoff time.Time,
	logger log.Logger,
) ([]ciBackfillJobRow, error) {
	// GCS path: pr-logs/pull/{org}_{repo}/{pr_number}/
	prPrefix := fmt.Sprintf("pr-logs/pull/%s_%s/%d/", pr.OrgName, pr.RepoShortName, prNum)

	jobPrefixes, err := store.ListSubdirectories(ctx, prPrefix)
	if err != nil {
		return nil, fmt.Errorf("listing jobs at %s: %w", prPrefix, err)
	}

	var rows []ciBackfillJobRow
	for _, jobPrefix := range jobPrefixes {
		jobName := gcshelper.LastSegment(jobPrefix)

		buildPrefixes, err := store.ListSubdirectories(ctx, jobPrefix)
		if err != nil {
			logger.Warn(errors.Default.WrapRaw(err), "Listing builds at %s", jobPrefix)
			continue
		}

		gcshelper.SortBuildIDsDescending(buildPrefixes)

		for _, buildPrefix := range buildPrefixes {
			buildID := gcshelper.LastSegment(buildPrefix)

			finishedData, err := store.ReadFile(ctx, buildPrefix+"finished.json")
			if err != nil {
				// Not finished yet — skip silently.
				continue
			}
			finished, err := gcshelper.ParseFinishedJSON(finishedData)
			if err != nil {
				logger.Warn(errors.Default.WrapRaw(err), "Parsing finished.json at %s", buildPrefix)
				continue
			}

			finishedAt := time.Unix(finished.Timestamp, 0).UTC()
			if finishedAt.Before(cutoff) {
				// Build IDs are monotonically increasing — stop scanning older builds.
				break
			}

			var startedAt *time.Time
			if started, startErr := gcshelper.ReadStartedJSON(ctx, store, buildPrefix); startErr == nil {
				t := time.Unix(started.Timestamp, 0).UTC()
				startedAt = &t
			}

			result := gcshelper.MapProwResult(finished.Result, finished.Passed)

			var durationSec *float64
			if startedAt != nil && !finishedAt.IsZero() {
				d := finishedAt.Sub(*startedAt).Seconds()
				durationSec = &d
			}

			prNumCopy := prNum
			jobId := fmt.Sprintf("prow:%s_%s:%d:%s:%s", pr.OrgName, pr.RepoShortName, prNum, jobName, buildID)

			rows = append(rows, ciBackfillJobRow{
				ConnectionId:      0, // sentinel: backfilled by aireview
				JobId:             jobId,
				JobName:           jobName,
				Repository:        pr.RepoShortName,
				TriggerType:       "pull_request",
				PullRequestNumber: &prNumCopy,
				Result:            result,
				FinishedAt:        &finishedAt,
				DurationSec:       durationSec,
			})
		}
	}
	return rows, nil
}
