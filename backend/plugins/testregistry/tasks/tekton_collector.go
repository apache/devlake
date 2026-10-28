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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/testregistry/models"
)

const (
	// QuayRegistryURL is the base URL for Quay.io registry
	QuayRegistryURL = "quay.io"
	// RAW_TEKTON_TABLE is the raw data table for storing Tekton PipelineRun JSON
	RAW_TEKTON_TABLE = "cicd_test_jobs"
)

// CollectTektonJobsMeta defines the metadata for the Tekton job collection subtask
var CollectTektonJobsMeta = plugin.SubTaskMeta{
	Name:             "collectTektonJobs",
	EntryPoint:       CollectTektonJobs,
	EnabledByDefault: true,
	Description:      "Collect Tekton PipelineRun jobs from OCI artifacts in Quay.io for the specified organization and repository scope. Pulls artifacts using ORAS and saves both raw data and normalized CI job records.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

// CollectTektonJobs is the main entry point for collecting Tekton jobs from OCI artifacts.
//
// This function:
// 1. Validates that the connection is for Tekton CI
// 2. Sets up ORAS client to pull OCI artifacts from Quay.io
// 3. Pulls artifacts for the specified repository scope
// 4. Parses Tekton PipelineRun data from artifacts
// 5. Saves raw data and normalized CI job records
// 6. Processes associated JUnit XML files if present
//
// Parameters:
//   - taskCtx: The subtask context providing access to logger, database, and other resources
//
// Returns:
//   - errors.Error: Any error encountered during collection, or nil if successful
func CollectTektonJobs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TestRegistryTaskData)
	logger := taskCtx.GetLogger()
	logger.Info("Collecting Tekton CI jobs", "scope", data.Options.FullName)

	// Validate connection type
	if data.Connection.CITool != models.CIToolTektonCI {
		logger.Debug("Connection is not Tekton CI, skipping")
		return nil
	}

	// Extract scope information
	// For Tekton CI, FullName format is "quayOrg/repoName" or "quayOrg/sub-org/repoName"
	// Example: FullName = "konflux-test-storage/konflux-team/release-service"
	//          quayOrg = "konflux-test-storage"
	//          Result: repoName = "konflux-team/release-service"
	quayOrg := strings.TrimSpace(data.Connection.QuayOrganization)
	if quayOrg == "" {
		return errors.BadInput.New("Quay organization is required for Tekton CI")
	}

	fullName := strings.TrimSpace(data.Options.FullName)
	if fullName == "" {
		return errors.BadInput.New("FullName is required")
	}

	// Remove Quay organization prefix from FullName to get repository name
	// Example: FullName = "konflux-test-storage/konflux-team/release-service"
	//          quayOrg = "konflux-test-storage"
	//          Result: repoName = "konflux-team/release-service"
	orgPrefix := quayOrg + "/"
	repoName := strings.TrimPrefix(fullName, orgPrefix)

	if repoName == "" {
		return errors.BadInput.New("Repository name could not be extracted from FullName")
	}

	// Build full repository path for Quay.io: org/repo
	repoFullPath := fmt.Sprintf("%s/%s", quayOrg, repoName)

	// Get logging directory from environment variable
	loggingDir := os.Getenv("LOGGING_DIR")
	if loggingDir == "" {
		loggingDir = "/app/logs"
	}

	// Setup raw data collection
	rawDataSubTask, err := setupRawTektonDataCollection(taskCtx, data)
	if err != nil {
		return err
	}

	// Get sync policy to determine date range for artifact collection
	syncPolicy := taskCtx.TaskContext().SyncPolicy()
	var since, until *time.Time
	if syncPolicy != nil {
		since = syncPolicy.TimeAfter
		if since == nil {
			// Default to last 6 months if no timeAfter is set
			sixMonthsAgo := time.Now().AddDate(0, -6, 0)
			since = &sixMonthsAgo
		}
		until = nil // Collect until now
	} else {
		// Default to last 6 months if no sync policy
		sixMonthsAgo := time.Now().AddDate(0, -6, 0)
		since = &sixMonthsAgo
	}

	// Setup Quay.io API client for listing tags with date filtering
	ctx := taskCtx.GetContext()
	quayClient, err := NewQuayClient(ctx, logger)
	if err != nil {
		return errors.Default.Wrap(err, "failed to create Quay.io client")
	}

	// List all tags within sync policy dates
	quayTags, err := quayClient.ListTags(ctx, quayOrg, repoName, since, until)
	if err != nil {
		logger.Warn(err, "failed to list tags from Quay.io API, will try to pull 'latest'")
		quayTags = []QuayTag{{Name: "latest"}}
	}

	if len(quayTags) == 0 {
		logger.Info("No tags found for repository in the specified date range", "repository", repoFullPath)
		return nil
	}

	logger.Info("Found tags matching date range", "count", len(quayTags), "repository", repoFullPath)

	// Setup ORAS client for pulling artifacts
	orasClient, err := NewORASClient(ctx, QuayRegistryURL, repoFullPath, loggingDir, logger)
	if err != nil {
		return errors.Default.Wrap(err, "failed to create ORAS client")
	}

	// Get database connection and raw data parameters
	db := taskCtx.GetDal()
	rawTable := rawDataSubTask.GetTable()
	rawParams := rawDataSubTask.GetParams()
	apiURL := fmt.Sprintf("oras://%s/%s", QuayRegistryURL, repoFullPath)

	// Process artifacts
	stats := processTektonArtifacts(taskCtx, orasClient, quayTags, data, rawDataSubTask, db, rawTable, rawParams, apiURL, loggingDir, repoFullPath, quayOrg, repoName)

	// Log final statistics
	logger.Info("Completed Tekton job collection", "repository", repoFullPath, "artifacts_processed", len(quayTags), "jobs_saved", stats.savedCount, "raw_records_saved", stats.rawSavedCount, "junit_found", stats.junitFoundCount, "junit_not_found", stats.junitNotFoundCount)

	return nil
}

// processTektonArtifacts processes Tekton OCI artifacts and extracts PipelineRun data
//
// Parameters:
//   - taskCtx: The subtask context
//   - orasClient: ORAS client for pulling artifacts
//   - artifacts: List of QuayTag objects to process (includes tag name and date)
//   - data: The task data
//   - rawDataSubTask: Raw data subtask for saving raw JSON
//   - loggingDir: Directory to store artifacts
//   - repoFullPath: Full repository path (org/repo) - used for ORAS pull and logging
//   - quayOrg: Quay.io organization name (for CI job organization field)
//   - repoName: Repository name (for CI job repository field)
//
// Returns:
//   - collectionStats: Statistics about the processed artifacts
func processTektonArtifacts(
	taskCtx plugin.SubTaskContext,
	orasClient *ORASClient,
	artifacts []QuayTag,
	data *TestRegistryTaskData,
	rawDataSubTask *helper.RawDataSubTask,
	db dal.Dal,
	rawTable string,
	rawParams string,
	apiURL string,
	loggingDir string,
	repoFullPath string,
	quayOrg string,
	repoName string,
) collectionStats {
	logger := taskCtx.GetLogger()
	ctx := taskCtx.GetContext()

	stats := collectionStats{}
	processedCount := 0

	// Ensure tmp directory cleanup happens even if processing fails
	tmpDir := filepath.Join(loggingDir, "tmp")
	defer func() {
		if cleanupErr := os.RemoveAll(tmpDir); cleanupErr != nil {
			logger.Warn(cleanupErr, "failed to cleanup tmp directory")
		}
	}()

	taskCtx.SetProgress(0, len(artifacts))

	for _, tag := range artifacts {
		processedCount++
		if processedCount%10 == 0 || processedCount == len(artifacts) {
			taskCtx.SetProgress(processedCount, len(artifacts))
		}

		artifactRef := tag.Name

		// Check if tag (job_id) already exists in database before pulling artifact
		// The tag name from Quay is the same as the job_id (PipelineRunName) in the database
		if isTektonJobAlreadyProcessed(db, data.Options.ConnectionId, artifactRef) {
			logger.Debug("Tag already processed as job_id, skipping artifact pull", "tag", artifactRef)
			continue
		}

		logger.Info("Processing artifact [%d/%d]: quay.io/%s:%s", processedCount, len(artifacts), repoFullPath, artifactRef)

		// Pull artifact using ORAS
		artifactPath, err := orasClient.PullArtifact(ctx, artifactRef)
		if err != nil {
			logger.Warn(err, "failed to pull artifact", "ref", artifactRef)
			continue
		}

		// Extract and parse PipelineRun data from artifact
		pipelineRuns, err := extractTektonPipelineRuns(ctx, orasClient, artifactPath, loggingDir, logger)
		if err != nil {
			logger.Warn(err, "failed to extract PipelineRuns from artifact", "ref", artifactRef)
			// Cleanup and skip this artifact
			if artifactPath != "" {
				os.RemoveAll(artifactPath)
			}
			continue
		}

		// If no valid pipeline runs found or structure doesn't match, cleanup and skip
		if len(pipelineRuns) == 0 {
			logger.Warn(nil, "no valid PipelineRuns found in artifact", "ref", artifactRef)
			if artifactPath != "" {
				os.RemoveAll(artifactPath)
			}
			continue
		}

		logger.Debug("Found %d PipelineRuns in artifact", len(pipelineRuns), "ref", artifactRef)

		// Process each PipelineRun (keep artifactPath until all jobs are processed for JUnit extraction)
		for _, pipelineRun := range pipelineRuns {
			if pipelineRun == nil {
				continue
			}

			// Extract job ID early to check if already processed
			jobId := pipelineRun.PipelineRunName
			if jobId == "" {
				logger.Warn(nil, "PipelineRun missing PipelineRunName, skipping")
				continue
			}

			// Check if job already processed
			if isTektonJobAlreadyProcessed(db, data.Options.ConnectionId, jobId) {
				logger.Debug("Tekton job already processed, skipping", "job_id", jobId)
				continue
			}

			// Save raw PipelineRun JSON
			if err := saveRawTektonData(db, logger, pipelineRun, rawParams, rawTable, apiURL); err != nil {
				logger.Warn(err, "failed to save raw Tekton PipelineRun data")
			} else {
				stats.rawSavedCount++
			}

			// Convert to normalized CI job
			ciJob, err := convertTektonPipelineRunToCIJob(pipelineRun, data.Options.ConnectionId, data.Options.FullName, quayOrg, repoName)
			if err != nil {
				logger.Warn(err, "failed to convert Tekton PipelineRun to CI job")
				continue
			}

			// Validate required fields
			missingFields := validateRequiredCIJobFields(ciJob)
			if len(missingFields) > 0 {
				logger.Warn(nil, "CI job missing required fields, skipping", "job_id", ciJob.JobId, "missing_fields", missingFields)
				continue
			}

			// Save to database
			if err := db.CreateOrUpdate(ciJob); err != nil {
				logger.Warn(err, "failed to save CI job to database", "job_id", ciJob.JobId)
				continue
			}

			stats.savedCount++
			logger.Debug("Saved Tekton CI job", "job_id", ciJob.JobId, "job_name", ciJob.JobName, "result", ciJob.Result)

			// Save Tekton task runs
			if err := saveTektonTasks(db, logger, data.Options.ConnectionId, ciJob.JobId, pipelineRun.TaskRuns); err != nil {
				logger.Warn(err, "failed to save Tekton tasks", "job_id", ciJob.JobId)
			}

			// Find and process JUnit XML files from artifact using configured regex
			if findAndProcessJUnitFiles(taskCtx, artifactPath, ciJob, quayOrg, repoName, data.JUnitRegex) {
				stats.junitFoundCount++
			} else {
				stats.junitNotFoundCount++
			}
		}

		// Cleanup artifact after processing all PipelineRuns
		if artifactPath != "" {
			os.RemoveAll(artifactPath)
		}
	}

	return stats
}

// TektonPipelineRun represents a Tekton PipelineRun structure
// This is a placeholder - the actual structure should match Tekton API schema
// TektonTaskRun represents a task run within a PipelineRun
type TektonTaskRun struct {
	Name     string `json:"name"`     // Task run name (e.g., "deploy-konflux")
	Status   string `json:"status"`   // Task status: "Succeeded", "Failed", etc.
	Duration string `json:"duration"` // Duration in seconds (e.g., "483s")
}

// TektonGitInfo represents Git organization and repository information
type TektonGitInfo struct {
	GitOrganization   string `json:"gitOrganization"`   // GitHub organization (e.g., "konflux-ci")
	GitRepository     string `json:"gitRepository"`     // Repository name (e.g., "integration-service")
	PullRequestNumber string `json:"pullRequestNumber"` // PR number as string (e.g., "1315")
	CommitSha         string `json:"commitSha"`         // Git commit SHA (e.g., "b4f3f3f")
	PullRequestAuthor string `json:"pullRequestAuthor"` // PR author (e.g., "bot-konflux")
}

// TektonTimestamps represents timestamp information for a PipelineRun
type TektonTimestamps struct {
	CreatedAt  string `json:"createdAt"`  // Creation timestamp in RFC3339 format (e.g., "2024-06-18T10:15:30Z")
	StartedAt  string `json:"startedAt"`  // Start timestamp in RFC3339 format (e.g., "2024-06-18T10:16:00Z")
	FinishedAt string `json:"finishedAt"` // Finish timestamp in RFC3339 format (e.g., "2024-06-18T11:20:06Z")
}

// TektonPipelineRun represents a Tekton PipelineRun from pipeline-status.json
// This structure matches the JSON format found in OCI artifacts
type TektonPipelineRun struct {
	PipelineRunName string           `json:"pipelineRunName"` // Pipeline run name (e.g., "konflux-e2e-z28lw")
	Namespace       string           `json:"namespace"`       // Kubernetes namespace (e.g., "konflux-ci")
	Duration        string           `json:"duration"`        // Total duration in seconds (e.g., "3846s")
	Status          string           `json:"status"`          // Overall status: "Succeeded", "Failed", etc.
	EventType       string           `json:"eventType"`       // Event type: "push", "pull_request", etc.
	Scenario        string           `json:"scenario"`        // Test scenario name (e.g., "konflux-e2e")
	ConsoleUrl      string           `json:"consoleUrl"`      // URL to view the pipeline in console (e.g., "https://ci.konflux-ci.dev/...")
	Git             TektonGitInfo    `json:"git"`             // Git organization and repository info
	Timestamps      TektonTimestamps `json:"timestamps"`      // Timestamp information
	TaskRuns        []TektonTaskRun  `json:"taskRuns"`        // List of task runs within the pipeline
}

// extractTektonPipelineRuns extracts Tekton PipelineRun data from OCI artifact
// Looks for pipeline-status.json files in the pulled artifact and parses them
// If the JSON structure doesn't match the expected format, it will be skipped
//
// Parameters:
//   - ctx: Context for the operation
//   - orasClient: ORAS client
//   - artifactPath: Local path where artifact was pulled (tmp/{uuid}/)
//   - loggingDir: Base logging directory (for logging purposes)
//   - logger: Logger for error reporting
//
// Returns:
//   - []*TektonPipelineRun: List of PipelineRun objects found in the artifact
//   - errors.Error: Any error encountered during extraction (should trigger cleanup)
func extractTektonPipelineRuns(ctx context.Context, orasClient *ORASClient, artifactPath, loggingDir string, logger log.Logger) ([]*TektonPipelineRun, errors.Error) {
	var pipelineRuns []*TektonPipelineRun

	// ORAS extracts files directly to artifactPath, so we search there
	// Walk the artifact directory to find pipeline-status.json files
	err := filepath.Walk(artifactPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr // Continue on error, but log it
		}

		// Look for pipeline-status.json files
		if !info.IsDir() && filepath.Base(path) == "pipeline-status.json" {
			// Read and parse the pipeline-status.json file
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				logger.Warn(readErr, "failed to read pipeline-status.json file")
				return nil // Continue processing other files
			}

			var pipelineRun TektonPipelineRun
			if unmarshalErr := json.Unmarshal(content, &pipelineRun); unmarshalErr != nil {
				logger.Warn(unmarshalErr, "failed to parse pipeline-status.json")
				return nil // Continue processing other files
			}

			// Verify it's a valid PipelineRun (has required fields)
			if pipelineRun.PipelineRunName == "" || pipelineRun.Status == "" {
				logger.Warn(nil, "pipeline-status.json missing required fields", "pipelineRunName", pipelineRun.PipelineRunName, "status", pipelineRun.Status)
				return nil // Skip this invalid pipeline run
			}

			pipelineRuns = append(pipelineRuns, &pipelineRun)
			logger.Debug("Parsed pipeline-status.json", "pipelineRunName", pipelineRun.PipelineRunName, "status", pipelineRun.Status)
		}

		return nil
	})

	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to walk artifact directory")
	}

	return pipelineRuns, nil
}

// isTektonJobAlreadyProcessed checks if a Tekton CI job already exists in the database.
//
// Parameters:
//   - db: Database connection
//   - connectionId: The DevLake connection ID
//   - jobId: The CI job ID to check
//
// Returns:
//   - bool: true if the job already exists in the database, false otherwise
func isTektonJobAlreadyProcessed(db dal.Dal, connectionId uint64, jobId string) bool {
	// Check if job exists in ci_test_jobs table
	jobCount, err := db.Count(
		dal.From(&models.TestRegistryCIJob{}),
		dal.Where("connection_id = ? AND job_id = ?", connectionId, jobId),
	)
	if err != nil {
		// If query fails, assume not processed (safer to process again)
		return false
	}

	// If job exists, it's already processed
	return jobCount > 0
}

// setupRawTektonDataCollection initializes the raw data collection subtask for Tekton
func setupRawTektonDataCollection(taskCtx plugin.SubTaskContext, data *TestRegistryTaskData) (*helper.RawDataSubTask, errors.Error) {
	return helper.NewRawDataSubTask(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: TestRegistryApiParams{
			ConnectionId: data.Options.ConnectionId,
			FullName:     data.Options.FullName,
		},
		Table: RAW_TEKTON_TABLE,
	})
}

// saveRawTektonData saves the raw Tekton PipelineRun JSON to the raw data table
//
// Parameters:
//   - db: Database connection
//   - logger: Logger for error reporting
//   - pipelineRun: The Tekton PipelineRun to save
//   - rawParams: Parameters identifying this collection run
//   - rawTable: Name of the raw data table
//   - apiURL: The API URL from which this data was fetched
//
// Returns:
//   - errors.Error: Any error encountered during saving, or nil if successful
func saveRawTektonData(db dal.Dal, logger log.Logger, pipelineRun *TektonPipelineRun, rawParams string, rawTable, apiURL string) errors.Error {
	pipelineRunJSON, err := json.Marshal(pipelineRun)
	if err != nil {
		return errors.Default.Wrap(err, "failed to marshal Tekton PipelineRun to JSON")
	}

	rawData := &helper.RawData{
		Params:    rawParams,
		Data:      pipelineRunJSON,
		Url:       apiURL,
		CreatedAt: time.Now(),
	}

	return db.Create(rawData, dal.From(rawTable))
}

// convertTektonPipelineRunToCIJob converts a TektonPipelineRun to a TestRegistryCIJob model
//
// Parameters:
//   - pipelineRun: The source Tekton PipelineRun from pipeline-status.json
//   - connectionId: The DevLake connection ID
//   - scopeId: The scope ID (repository full name)
//   - organization: The Quay.io organization name
//   - repository: The repository name
//
// Returns:
//   - *models.TestRegistryCIJob: The converted CI job model
//   - errors.Error: An error if conversion fails
func convertTektonPipelineRunToCIJob(pipelineRun *TektonPipelineRun, connectionId uint64, scopeId, organization, repository string) (*models.TestRegistryCIJob, errors.Error) {
	ciJob := &models.TestRegistryCIJob{
		ConnectionId: connectionId,
		JobType:      "tekton",
		Organization: organization,
		Repository:   repository,
		ScopeId:      scopeId,
	}

	// Extract job ID from PipelineRunName, and job name from Scenario
	ciJob.JobId = pipelineRun.PipelineRunName
	ciJob.JobName = pipelineRun.Scenario

	// Use Git info from pipeline-status.json if available, otherwise use provided org/repo
	if pipelineRun.Git.GitOrganization != "" {
		ciJob.Organization = pipelineRun.Git.GitOrganization
	}
	if pipelineRun.Git.GitRepository != "" {
		ciJob.Repository = pipelineRun.Git.GitRepository
	}

	// Extract commit SHA
	if pipelineRun.Git.CommitSha != "" {
		ciJob.CommitSHA = pipelineRun.Git.CommitSha
	}

	// Extract namespace
	if pipelineRun.Namespace != "" {
		ciJob.Namespace = pipelineRun.Namespace
	}

	// Map trigger type from EventType field
	if pipelineRun.EventType == "pull_request" {
		ciJob.TriggerType = "pull_request"

		// Only extract PR info when event type is pull_request
		if pipelineRun.Git.PullRequestNumber != "" {
			if prNum, parseErr := strconv.Atoi(pipelineRun.Git.PullRequestNumber); parseErr == nil {
				ciJob.PullRequestNumber = &prNum
			}
		}

		// Extract pull request author only for pull_request events
		if pipelineRun.Git.PullRequestAuthor != "" {
			ciJob.PullRequestAuthor = pipelineRun.Git.PullRequestAuthor
		}
	} else {
		ciJob.TriggerType = "push" // Default for "push" or any other event type
	}

	// Map status from Status field to database result
	// Based on Tekton PipelineRun status values:
	// Reference: https://github.com/konflux-ci/tekton-integration-catalog/blob/main/tasks/store-pipeline-status/0.1/store-pipeline-status.yaml
	// Possible values: "Succeeded", "Failed", "Cancelled", "Running", "Pending", etc.
	switch pipelineRun.Status {
	case "Succeeded":
		ciJob.Result = "SUCCESS"
	case "Failed":
		ciJob.Result = "FAILURE"
	case "Cancelled":
		ciJob.Result = "ABORTED" // Map cancelled to ABORTED (standard CI/CD status)
	case "Running":
		ciJob.Result = "OTHER" // Running jobs are typically not stored, but handle if found
	case "Pending":
		ciJob.Result = "OTHER" // Pending jobs are typically not stored, but handle if found
	default:
		ciJob.Result = "OTHER" // For "Unknown", or any other unexpected status
	}

	// Parse duration from string (e.g., "3846s") to seconds
	// Duration format is like "3846s" - extract the number and convert to float64
	if pipelineRun.Duration != "" {
		// Remove "s" suffix and parse to float64
		durationStr := strings.TrimSuffix(pipelineRun.Duration, "s")
		if duration, parseErr := strconv.ParseFloat(durationStr, 64); parseErr == nil {
			ciJob.DurationSec = &duration
		}
	}

	// Parse timestamps from Timestamps field
	if pipelineRun.Timestamps.CreatedAt != "" {
		if t, err := common.ConvertStringToTime(pipelineRun.Timestamps.CreatedAt); err == nil {
			ciJob.QueuedAt = &t
		}
	}
	if pipelineRun.Timestamps.StartedAt != "" {
		if t, err := common.ConvertStringToTime(pipelineRun.Timestamps.StartedAt); err == nil {
			ciJob.StartedAt = &t
		}
	}
	if pipelineRun.Timestamps.FinishedAt != "" {
		if t, err := common.ConvertStringToTime(pipelineRun.Timestamps.FinishedAt); err == nil {
			ciJob.FinishedAt = &t
		}
	}

	// Calculate queued duration (time between creation and start)
	if ciJob.QueuedAt != nil && ciJob.StartedAt != nil {
		queuedDuration := ciJob.StartedAt.Sub(*ciJob.QueuedAt).Seconds()
		ciJob.QueuedDurationSec = &queuedDuration
	}

	// Extract console URL (ViewURL)
	if pipelineRun.ConsoleUrl != "" {
		ciJob.ViewURL = pipelineRun.ConsoleUrl
	}

	return ciJob, nil
}

// validateRequiredCIJobFields validates that all required fields are present in a CI job
// Returns a list of missing required field names
func validateRequiredCIJobFields(ciJob *models.TestRegistryCIJob) []string {
	var missingFields []string

	// Primary key fields (always required)
	if ciJob.ConnectionId == 0 {
		missingFields = append(missingFields, "ConnectionId")
	}
	if ciJob.JobId == "" {
		missingFields = append(missingFields, "JobId")
	}

	// Required indexed fields
	if ciJob.JobName == "" {
		missingFields = append(missingFields, "JobName")
	}
	if ciJob.JobType == "" {
		missingFields = append(missingFields, "JobType")
	}
	if ciJob.Organization == "" {
		missingFields = append(missingFields, "Organization")
	}
	if ciJob.Repository == "" {
		missingFields = append(missingFields, "Repository")
	}
	if ciJob.TriggerType == "" {
		missingFields = append(missingFields, "TriggerType")
	}
	if ciJob.Result == "" {
		missingFields = append(missingFields, "Result")
	}
	if ciJob.ScopeId == "" {
		missingFields = append(missingFields, "ScopeId")
	}

	// Conditional required fields based on trigger type
	if ciJob.TriggerType == "pull_request" && ciJob.PullRequestNumber == nil {
		missingFields = append(missingFields, "PullRequestNumber (required for pull_request)")
	}

	// Required timestamps (at least FinishedAt should be present)
	if ciJob.FinishedAt == nil {
		missingFields = append(missingFields, "FinishedAt")
	}
	if ciJob.StartedAt == nil {
		missingFields = append(missingFields, "StartedAt")
	}

	// CommitSHA is required for traceability
	if ciJob.CommitSHA == "" {
		missingFields = append(missingFields, "CommitSHA")
	}

	return missingFields
}
