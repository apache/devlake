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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/testregistry/models"
)

const (
	// ProwBaseURL is the base URL for the Openshift CI Prow API
	ProwBaseURL = "https://prow.ci.openshift.org"

	// ProwJobsPath is the API endpoint path for fetching all Prow jobs
	ProwJobsPath = "prowjobs.js"

	// RAW_PROW_TABLE is the raw data table name for storing Prow job JSON responses
	RAW_PROW_TABLE = "cicd_test_jobs"
)

// CollectProwJobsMeta defines the metadata for the Prow job collection subtask
var CollectProwJobsMeta = plugin.SubTaskMeta{
	Name:             "collectProwJobs",
	EntryPoint:       CollectProwJobs,
	EnabledByDefault: true,
	Description:      "Collect Prow jobs from Openshift CI (https://prow.ci.openshift.org) for the specified GitHub organization and repository scope. Saves both raw JSON data and normalized CI job records.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

// CollectProwJobs is the main entry point for collecting Prow jobs from Openshift CI.
//
// This function:
// 1. Fetches all Prow jobs from the Openshift CI API
// 2. Filters jobs that match the specified GitHub organization and repository
// 3. Saves raw job JSON to the raw data table
// 4. Converts and saves normalized CI job records
// 5. Attempts to fetch and log JUnit test suite information from GCS
//
// Parameters:
//   - taskCtx: The subtask context providing access to logger, database, and other resources
//
// Returns:
//   - errors.Error: Any error encountered during collection, or nil if successful
func CollectProwJobs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TestRegistryTaskData)
	logger := taskCtx.GetLogger()
	logger.Info("collecting Prow jobs for scope: %s", data.Options.FullName)

	// Validate connection type
	if data.Connection.CITool != models.CIToolOpenshiftCI {
		logger.Info("Connection is not Openshift CI, skipping Prow job collection")
		return nil
	}

	// Extract scope information
	repoName := data.Options.FullName
	githubOrg := data.Connection.GitHubOrganization
	if githubOrg == "" {
		return errors.BadInput.New("GitHub organization is required for Openshift CI")
	}

	// Setup raw data collection
	rawDataSubTask, err := setupRawDataCollection(taskCtx, data)
	if err != nil {
		return err
	}

	// Fetch Prow jobs from API
	allJobs, err := fetchProwJobsFromAPI(taskCtx)
	if err != nil {
		return err
	}

	logger.Info("Fetched %d Prow jobs total, filtering for scope %s/%s", len(allJobs), githubOrg, repoName)

	// Process and save matching jobs
	db := taskCtx.GetDal()
	rawTable := rawDataSubTask.GetTable()
	rawParams := rawDataSubTask.GetParams()
	apiURL := fmt.Sprintf("%s/%s", ProwBaseURL, ProwJobsPath)

	stats := &collectionStats{}
	stats.processJobs(
		taskCtx,
		db,
		allJobs,
		rawTable,
		rawParams,
		apiURL,
		githubOrg,
		repoName,
		data,
	)

	// Log final summary
	logger.Info(
		"Found %d Prow jobs matching scope %s/%s, saved %d CI jobs and %d raw records to database. JUnit XML found for %d jobs, not found for %d jobs",
		stats.matchingCount,
		githubOrg,
		repoName,
		stats.savedCount,
		stats.rawSavedCount,
		stats.junitFoundCount,
		stats.junitNotFoundCount,
	)

	return nil
}

// collectionStats tracks statistics during job collection
type collectionStats struct {
	matchingCount      int
	savedCount         int
	rawSavedCount      int
	processedCount     int
	junitFoundCount    int
	junitNotFoundCount int
}

// processJobs iterates through all Prow jobs, filters matching ones, and saves them to the database
func (stats *collectionStats) processJobs(
	taskCtx plugin.SubTaskContext,
	db dal.Dal,
	allJobs []ProwJob,
	rawTable string,
	rawParams string,
	apiURL string,
	githubOrg string,
	repoName string,
	data *TestRegistryTaskData,
) {
	logger := taskCtx.GetLogger()
	taskCtx.SetProgress(0, len(allJobs))

	// Create GCS client once for the entire task run
	gcsClient, gcsErr := NewGCSBucketClient(taskCtx.GetContext())
	if gcsErr != nil {
		logger.Warn(gcsErr, "failed to create GCS client, JUnit collection will be skipped")
	}
	if gcsClient != nil {
		defer func() { _ = gcsClient.Close() }()
	}

	for _, job := range allJobs {
		stats.processedCount++

		// Update progress periodically
		if stats.processedCount%100 == 0 || stats.processedCount == len(allJobs) {
			taskCtx.SetProgress(stats.processedCount, len(allJobs))
		}

		// Process matching jobs only
		if !matchesScope(&job, githubOrg, repoName) {
			continue
		}

		stats.matchingCount++

		// Save raw job JSON
		if err := saveRawJobData(db, rawTable, rawParams, apiURL, &job); err != nil {
			logger.Warn(err, "failed to save raw Prow job data")
		} else {
			stats.rawSavedCount++
		}

		// Convert and save normalized CI job
		ciJob, err := convertProwJobToCIJob(&job, data.Options.ConnectionId, data.Options.FullName, githubOrg, repoName)
		if err != nil {
			logger.Warn(err, "failed to convert Prow job to CI job")
			continue
		}

		if err := db.CreateOrUpdate(ciJob); err != nil {
			logger.Warn(err, "failed to save CI job to database", "job_id", ciJob.JobId)
			continue
		}

		stats.savedCount++

		// Fetch and log JUnit test suites using configured regex
		if gcsClient == nil {
			stats.junitNotFoundCount++
			continue
		}
		logger.Debug("Attempting to fetch JUnit XML for job", "job_id", ciJob.JobId, "job_name", ciJob.JobName, "trigger_type", ciJob.TriggerType)
		if fetchAndPrintJUnitSuites(taskCtx, gcsClient, &job, githubOrg, repoName, ciJob, data.JUnitRegex) {
			stats.junitFoundCount++
		} else {
			stats.junitNotFoundCount++
		}
	}

	// Final progress update
	taskCtx.SetProgress(len(allJobs), len(allJobs))
}

// setupRawDataCollection initializes the raw data collection subtask
func setupRawDataCollection(taskCtx plugin.SubTaskContext, data *TestRegistryTaskData) (*helper.RawDataSubTask, errors.Error) {
	return helper.NewRawDataSubTask(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: TestRegistryApiParams{
			ConnectionId: data.Options.ConnectionId,
			FullName:     data.Options.FullName,
		},
		Table: RAW_PROW_TABLE,
	})
}

const (
	prowMaxRetries    = 5
	prowRetryBaseWait = 10 * time.Second
)

// isTransientStatusCode returns true for HTTP status codes that warrant a retry.
func isTransientStatusCode(code int) bool {
	return code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout ||
		code == http.StatusTooManyRequests
}

// fetchProwJobsFromAPI retrieves all Prow jobs from the Openshift CI API.
// Transient errors (502, 503, 504, 429) are retried up to prowMaxRetries times
// with exponential backoff starting at prowRetryBaseWait.
func fetchProwJobsFromAPI(taskCtx plugin.SubTaskContext) ([]ProwJob, errors.Error) {
	logger := taskCtx.GetLogger()

	apiClient, err := helper.NewApiClient(taskCtx.GetContext(), ProwBaseURL, nil, 0, "", taskCtx)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create API client for Prow")
	}

	var lastErr errors.Error
	for attempt := 1; attempt <= prowMaxRetries; attempt++ {
		resp, fetchErr := apiClient.Get(ProwJobsPath, nil, nil)
		if fetchErr != nil {
			lastErr = errors.Default.Wrap(fetchErr, "failed to fetch Prow jobs")
			wait := prowRetryBaseWait * time.Duration(attempt)
			logger.Warn(lastErr, "Prow API request failed (attempt %d/%d), retrying in %s", attempt, prowMaxRetries, wait)
			time.Sleep(wait)
			continue
		}

		if isTransientStatusCode(resp.StatusCode) {
			_ = resp.Body.Close()
			lastErr = errors.Default.New(fmt.Sprintf("Prow API returned status %d", resp.StatusCode))
			wait := prowRetryBaseWait * time.Duration(attempt)
			logger.Warn(lastErr, "Prow API transient error (attempt %d/%d), retrying in %s", attempt, prowMaxRetries, wait)
			time.Sleep(wait)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, errors.Default.New(fmt.Sprintf("Prow API returned status %d", resp.StatusCode))
		}

		// Parse response — Prow API returns {"items": [...]} format.
		var results map[string][]ProwJob
		if parseErr := helper.UnmarshalResponse(resp, &results); parseErr != nil {
			return nil, errors.Default.Wrap(parseErr, "failed to parse Prow jobs response")
		}

		allJobs := results["items"]
		if allJobs == nil {
			allJobs = []ProwJob{}
		}
		return allJobs, nil
	}

	return nil, errors.Default.Wrap(lastErr, fmt.Sprintf("Prow API failed after %d attempts", prowMaxRetries))
}

// saveRawJobData saves the raw Prow job JSON to the raw data table
//
// Parameters:
//   - db: Database connection
//   - rawTable: Name of the raw data table
//   - rawParams: Parameters identifying this collection run
//   - apiURL: The API URL from which this data was fetched
//   - job: The Prow job to save
//
// Returns:
//   - errors.Error: Any error encountered during saving, or nil if successful
func saveRawJobData(db dal.Dal, rawTable, rawParams, apiURL string, job *ProwJob) errors.Error {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return errors.Default.Wrap(err, "failed to marshal Prow job to JSON")
	}

	rawData := &helper.RawData{
		Params:    rawParams,
		Data:      jobJSON,
		Url:       apiURL,
		CreatedAt: time.Now(),
	}

	return db.Create(rawData, dal.From(rawTable))
}

// matchesScope checks if a Prow job matches the given GitHub organization and repository.
//
// This function checks multiple sources in order of reliability:
// 1. Prow job labels (most reliable): "prow.k8s.io/refs.org" and "prow.k8s.io/refs.repo"
// 2. Main refs: job.Spec.Refs.Org and job.Spec.Refs.Repo
// 3. Extra refs: Any matching org/repo in job.Spec.ExtraRefs
//
// Additionally, jobs in "aborted", "pending", or "triggered" states are excluded,
// matching the behavior of the quality-dashboard implementation.
//
// Parameters:
//   - job: The Prow job to check
//   - githubOrg: Expected GitHub organization name
//   - repoName: Expected repository name
//
// Returns:
//   - bool: true if the job matches the scope and is in a valid state, false otherwise
func matchesScope(job *ProwJob, githubOrg, repoName string) bool {
	// Check labels first (most reliable method, used by quality-dashboard)
	if job.Labels != nil {
		prowOrg := job.Labels["prow.k8s.io/refs.org"]
		prowRepo := job.Labels["prow.k8s.io/refs.repo"]

		if prowOrg == githubOrg && prowRepo == repoName {
			return isValidJobState(job.Status.State)
		}
	}

	// Fallback: Check main refs
	if job.Spec.Refs != nil {
		if job.Spec.Refs.Org == githubOrg && job.Spec.Refs.Repo == repoName {
			return isValidJobState(job.Status.State)
		}
	}

	// Fallback: Check extra refs
	for _, extraRef := range job.Spec.ExtraRefs {
		if extraRef != nil && extraRef.Org == githubOrg && extraRef.Repo == repoName {
			return isValidJobState(job.Status.State)
		}
	}

	return false
}

// isValidJobState checks if a Prow job state is valid for processing.
//
// Jobs in "aborted", "pending", or "triggered" states are excluded as they
// don't represent completed job runs.
//
// Parameters:
//   - state: The Prow job state string (e.g., "success", "failure", "aborted")
//
// Returns:
//   - bool: true if the state is valid for processing, false otherwise
func isValidJobState(state string) bool {
	stateLower := strings.ToLower(state)
	return stateLower != "aborted" && stateLower != "pending" && stateLower != "triggered"
}

// convertProwJobToCIJob converts a ProwJob structure to a TestRegistryCIJob database model.
//
// This function extracts relevant information from the Prow job and maps it to
// the unified CI job structure. It handles:
// - Job identification (ID, name, type)
// - Repository information (org, repo)
// - Git references (commit SHA, PR information)
// - Trigger type mapping (presubmit -> pull_request, postsubmit -> push, periodic -> periodic)
// - Status and timing information
//
// Parameters:
//   - prowJob: The source Prow job structure
//   - connectionId: The DevLake connection ID
//   - scopeId: The scope identifier (repository full name)
//   - organization: Default organization name (used as fallback)
//   - repository: Default repository name (used as fallback)
//
// Returns:
//   - *models.TestRegistryCIJob: The converted CI job model
//   - errors.Error: Any error encountered during conversion, or nil if successful
func convertProwJobToCIJob(prowJob *ProwJob, connectionId uint64, scopeId, organization, repository string) (*models.TestRegistryCIJob, errors.Error) {
	ciJob := &models.TestRegistryCIJob{
		ConnectionId: connectionId,
		JobType:      "prow",
		Organization: organization,
		Repository:   repository,
		ScopeId:      scopeId,
	}

	// Extract job ID
	ciJob.JobId = extractJobID(prowJob)
	ciJob.JobName = prowJob.Spec.Job

	// Extract organization and repository from Prow job metadata
	extractOrgRepo(ciJob, prowJob)

	// Extract git commit SHA and PR information
	extractGitInfo(ciJob, prowJob)

	// Map trigger type
	mapTriggerType(ciJob, prowJob)

	// Map job status
	mapJobStatus(ciJob, prowJob)

	// Set namespace
	ciJob.Namespace = prowJob.Spec.Namespace

	// Parse and set timestamps
	parseTimestamps(ciJob, prowJob)

	// Calculate durations
	calculateDurations(ciJob)

	// Set view URL
	ciJob.ViewURL = prowJob.Status.URL

	return ciJob, nil
}

// extractJobID extracts a unique job identifier from the Prow job.
//
// Priority order:
// 1. BuildID (most reliable)
// 2. PodName (if BuildID is not available)
// 3. Generated ID from job name + timestamp (fallback)
//
// Parameters:
//   - prowJob: The Prow job to extract ID from
//
// Returns:
//   - string: The extracted or generated job ID
func extractJobID(prowJob *ProwJob) string {
	if prowJob.Status.BuildID != "" {
		return prowJob.Status.BuildID
	}
	if prowJob.Status.PodName != "" {
		return prowJob.Status.PodName
	}
	// Fallback: use job name + timestamp
	return fmt.Sprintf("%s-%d", prowJob.Spec.Job, time.Now().Unix())
}

// extractOrgRepo extracts organization and repository information from Prow job labels or spec refs.
//
// This function prioritizes labels over spec refs, as labels are more reliable.
//
// Parameters:
//   - ciJob: The CI job model to populate
//   - prowJob: The source Prow job
func extractOrgRepo(ciJob *models.TestRegistryCIJob, prowJob *ProwJob) {
	// First, try to extract from labels (most reliable)
	if prowJob.Labels != nil {
		if org := prowJob.Labels["prow.k8s.io/refs.org"]; org != "" {
			ciJob.Organization = org
		}
		if repo := prowJob.Labels["prow.k8s.io/refs.repo"]; repo != "" {
			ciJob.Repository = repo
		}
	}

	// Fallback to spec refs if labels don't have org/repo
	if ciJob.Organization == "" || ciJob.Repository == "" {
		if prowJob.Spec.Refs != nil {
			if ciJob.Organization == "" && prowJob.Spec.Refs.Org != "" {
				ciJob.Organization = prowJob.Spec.Refs.Org
			}
			if ciJob.Repository == "" && prowJob.Spec.Refs.Repo != "" {
				ciJob.Repository = prowJob.Spec.Refs.Repo
			}
		}
	}
}

// extractGitInfo extracts git commit SHA and pull request information from Prow job refs.
//
// For presubmit jobs, it extracts PR number and author from the pulls array.
// For postsubmit jobs, it uses the base SHA.
//
// Parameters:
//   - ciJob: The CI job model to populate
//   - prowJob: The source Prow job
func extractGitInfo(ciJob *models.TestRegistryCIJob, prowJob *ProwJob) {
	if prowJob.Spec.Refs == nil {
		return
	}

	// Extract commit SHA: prefer PR SHA if available, otherwise use base SHA
	if len(prowJob.Spec.Refs.Pulls) > 0 {
		ciJob.CommitSHA = prowJob.Spec.Refs.Pulls[0].SHA
	} else if prowJob.Spec.Refs.BaseSHA != "" {
		ciJob.CommitSHA = prowJob.Spec.Refs.BaseSHA
	}

	// Extract pull request information
	if len(prowJob.Spec.Refs.Pulls) > 0 {
		prNumber := prowJob.Spec.Refs.Pulls[0].Number
		ciJob.PullRequestNumber = &prNumber
		ciJob.PullRequestAuthor = prowJob.Spec.Refs.Pulls[0].Author
	}
}

// mapTriggerType maps Prow job type to our unified trigger type format.
//
// Mapping:
//   - "presubmit" -> "pull_request"
//   - "postsubmit" -> "push"
//   - "periodic" -> "periodic"
//
// If the type is missing, it infers from the presence of PR information.
//
// Parameters:
//   - ciJob: The CI job model to populate
//   - prowJob: The source Prow job
func mapTriggerType(ciJob *models.TestRegistryCIJob, prowJob *ProwJob) {
	prowType := strings.ToLower(prowJob.Spec.Type)
	switch prowType {
	case "presubmit":
		ciJob.TriggerType = "pull_request"
	case "postsubmit":
		ciJob.TriggerType = "push"
	case "periodic":
		ciJob.TriggerType = "periodic"
	default:
		// Fallback: infer from presence of PR
		if ciJob.PullRequestNumber != nil {
			ciJob.TriggerType = "pull_request"
		} else {
			ciJob.TriggerType = "push" // Default assumption if type is missing
		}
	}
}

// mapJobStatus maps Prow job state to our unified result format.
//
// Mapping:
//   - "success" -> "SUCCESS"
//   - "failure", "error" -> "FAILURE"
//   - "aborted" -> "ABORTED"
//   - Other states are uppercased as-is
//
// Parameters:
//   - ciJob: The CI job model to populate
//   - prowJob: The source Prow job
func mapJobStatus(ciJob *models.TestRegistryCIJob, prowJob *ProwJob) {
	state := strings.ToLower(prowJob.Status.State)
	switch state {
	case "success":
		ciJob.Result = "SUCCESS"
	case "failure", "error":
		ciJob.Result = "FAILURE"
	case "aborted":
		ciJob.Result = "ABORTED"
	default:
		ciJob.Result = strings.ToUpper(state)
	}
}

// parseTimestamps parses ISO 8601 timestamp strings from Prow job status into Go time.Time values.
//
// Parameters:
//   - ciJob: The CI job model to populate
//   - prowJob: The source Prow job
func parseTimestamps(ciJob *models.TestRegistryCIJob, prowJob *ProwJob) {
	if prowJob.Status.PendingTime != "" {
		if t, err := common.ConvertStringToTime(prowJob.Status.PendingTime); err == nil {
			ciJob.QueuedAt = &t
		}
	}
	if prowJob.Status.StartTime != "" {
		if t, err := common.ConvertStringToTime(prowJob.Status.StartTime); err == nil {
			ciJob.StartedAt = &t
		}
	}
	if prowJob.Status.CompletionTime != "" {
		if t, err := common.ConvertStringToTime(prowJob.Status.CompletionTime); err == nil {
			ciJob.FinishedAt = &t
		}
	}
}

// calculateDurations calculates job execution durations from timestamp differences.
//
// It calculates:
//   - DurationSec: Time from start to finish
//   - QueuedDurationSec: Time spent waiting in queue before execution
//
// Parameters:
//   - ciJob: The CI job model to populate (timestamps must already be set)
func calculateDurations(ciJob *models.TestRegistryCIJob) {
	if ciJob.StartedAt != nil && ciJob.FinishedAt != nil {
		duration := ciJob.FinishedAt.Sub(*ciJob.StartedAt).Seconds()
		ciJob.DurationSec = &duration
	}
	if ciJob.QueuedAt != nil && ciJob.StartedAt != nil {
		queuedDuration := ciJob.StartedAt.Sub(*ciJob.QueuedAt).Seconds()
		ciJob.QueuedDurationSec = &queuedDuration
	}
}

// TestRegistryApiParams represents parameters for identifying a raw data collection run
type TestRegistryApiParams struct {
	ConnectionId uint64 `json:"connectionId"`
	FullName     string `json:"fullName"`
}
