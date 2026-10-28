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
	"crypto/rand"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/testregistry/models"
)

const (
	// uidChars are the characters used for generating unique IDs
	uidChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// uidLength is the length of generated UIDs
	uidLength = 16
)

// isJobAlreadyProcessed checks if a CI job already has test suites and test cases in the database.
// This helps avoid duplicate fetching and processing of JUnit XML data.
//
// Parameters:
//   - db: Database connection
//   - connectionId: The DevLake connection ID
//   - jobId: The CI job ID to check
//
// Returns:
//   - bool: true if the job already has test suites in the database, false otherwise
func isJobAlreadyProcessed(db dal.Dal, connectionId uint64, jobId string) bool {
	// Check if any test suites exist for this job
	suiteCount, err := db.Count(
		dal.From(&models.TestSuite{}),
		dal.Where("connection_id = ? AND job_id = ?", connectionId, jobId),
	)
	if err != nil {
		// If query fails, assume not processed (safer to fetch again)
		return false
	}

	// If we have suites, the job is already processed
	return suiteCount > 0
}

// fetchAndPrintJUnitSuites fetches JUnit XML from GCS and logs test suite information.
//
// This function:
// 1. Checks if the job is already processed (avoids duplicate fetching)
// 2. Creates a GCS client for accessing Openshift CI test results
// 3. Determines the correct GCS path based on job type (presubmit/postsubmit/periodic)
// 4. Fetches and parses JUnit XML files
// 5. Logs comprehensive suite information including nested suites
// 6. Saves test suites and test cases to the database
//
// For non-periodic jobs, it extracts org/repo from Prow job refs (matching quality-dashboard behavior).
// Reference: https://github.com/konflux-ci/quality-dashboard/blob/e846aa2dd9b3c1cad9ac4d16d18ddf677e3e6247/backend/api/server/prow_rotate.go#L64-L67
//
// Parameters:
//   - taskCtx: The subtask context
//   - job: The source Prow job
//   - githubOrg: Default GitHub organization (used as fallback)
//   - repoName: Default repository name (used as fallback)
//   - ciJob: The CI job model
//   - junitRegex: Compiled regex pattern for matching JUnit file names (uses default if nil)
//
// Returns:
//   - bool: true if JUnit XML was found and parsed successfully, false otherwise
func fetchAndPrintJUnitSuites(taskCtx plugin.SubTaskContext, gcsClient *GCSBucket, job *ProwJob, githubOrg, repoName string, ciJob *models.TestRegistryCIJob, junitRegex *regexp.Regexp) bool {
	logger := taskCtx.GetLogger()
	db := taskCtx.GetDal()

	// Use default regex if not provided
	if junitRegex == nil {
		junitRegex = JUnitRegexpSearch
	}

	// Check if this job is already processed (has test suites in database)
	if isJobAlreadyProcessed(db, ciJob.ConnectionId, ciJob.JobId) {
		logger.Info("Job already processed, skipping JUnit fetch", "job_id", ciJob.JobId, "job_name", ciJob.JobName)
		return true // Return true since we consider it "found" (already in DB)
	}

	// Determine job type for GCS path construction
	jobTypeForGCS, err := determineJobTypeForGCS(ciJob, job)
	if err != nil {
		logger.Info("unknown trigger type, skipping JUnit fetch", "trigger_type", ciJob.TriggerType, "job_id", ciJob.JobId, "job_name", ciJob.JobName)
		return false
	}

	// Extract PR number for presubmit jobs
	pullNumber := extractPullRequestNumber(ciJob)

	// Fetch all JUnit XML files from GCS using configurable regex
	ctx := taskCtx.GetContext()
	junitFiles := fetchJUnitFromGCS(ctx, gcsClient, job, ciJob, jobTypeForGCS, githubOrg, repoName, pullNumber, logger, junitRegex)

	if len(junitFiles) == 0 {
		logger.Info("No JUnit XML found for job", "job_id", ciJob.JobId, "job_name", ciJob.JobName, "trigger_type", ciJob.TriggerType)
		return false
	}

	logger.Info("Found JUnit XML files for job", "job_id", ciJob.JobId, "job_name", ciJob.JobName, "file_count", len(junitFiles))

	// Parse, log, and save suite information from all files
	anySuccess := false
	for _, jf := range junitFiles {
		if parseAndSaveJUnitSuites(taskCtx, logger, jf.Content, jf.Path, ciJob, githubOrg, repoName) {
			anySuccess = true
		}
	}
	return anySuccess
}

// determineJobTypeForGCS maps our trigger type to GCS job type format.
//
// Mapping:
//   - "pull_request" -> "presubmit"
//   - "push" -> "postsubmit"
//   - "periodic" -> "periodic"
//
// Parameters:
//   - ciJob: The CI job model
//   - job: The source Prow job (used as fallback)
//
// Returns:
//   - string: The job type for GCS ("presubmit", "postsubmit", or "periodic")
//   - errors.Error: Error if type cannot be determined
func determineJobTypeForGCS(ciJob *models.TestRegistryCIJob, job *ProwJob) (string, errors.Error) {
	switch ciJob.TriggerType {
	case "pull_request":
		return "presubmit", nil
	case "push":
		return "postsubmit", nil
	case "periodic":
		return "periodic", nil
	default:
		// Fallback: try to infer from Prow job spec type
		if job.Spec.Type != "" {
			return strings.ToLower(job.Spec.Type), nil
		}
		return "", errors.Default.New("cannot determine job type for GCS")
	}
}

// extractPullRequestNumber extracts the PR number as a string for GCS path construction.
//
// Parameters:
//   - ciJob: The CI job model
//
// Returns:
//   - string: PR number as string, or empty string if not available
func extractPullRequestNumber(ciJob *models.TestRegistryCIJob) string {
	if ciJob.TriggerType == "pull_request" && ciJob.PullRequestNumber != nil {
		return strconv.Itoa(*ciJob.PullRequestNumber)
	}
	return ""
}

// fetchJUnitFromGCS fetches all matching JUnit XML files from Google Cloud Storage.
//
// For non-periodic jobs, it extracts org/repo from Prow job refs to match quality-dashboard behavior.
// Reference: https://github.com/konflux-ci/quality-dashboard/blob/e846aa2dd9b3c1cad9ac4d16d18ddf677e3e6247/backend/api/server/prow_rotate.go#L64-L67
func fetchJUnitFromGCS(
	ctx context.Context,
	gcsClient *GCSBucket,
	job *ProwJob,
	ciJob *models.TestRegistryCIJob,
	jobTypeForGCS string,
	githubOrg string,
	repoName string,
	pullNumber string,
	logger log.Logger,
	junitRegex *regexp.Regexp,
) []JUnitFile {
	logger.Debug("Searching for JUnit XML in GCS", "job_id", ciJob.JobId, "job_name", ciJob.JobName, "job_type_for_gcs", jobTypeForGCS, "org", githubOrg, "repo", repoName, "pull_number", pullNumber)

	var files []JUnitFile
	var gcsErr error

	// Periodic jobs: empty org/repo/pr
	if jobTypeForGCS == "periodic" {
		files, gcsErr = gcsClient.GetJobJunitContent(ctx, "", "", "", ciJob.JobId, "periodic", ciJob.JobName, junitRegex)
	} else {
		// For non-periodic jobs, extract org/repo from Prow job refs
		orgForGCS, repoForGCS := extractOrgRepoForGCS(job, githubOrg, repoName, ciJob.JobId, logger)

		// Presubmit: need org, repo, and PR number
		if jobTypeForGCS == "presubmit" {
			if pullNumber == "" {
				logger.Info("Missing PR number for presubmit job, skipping JUnit fetch", "job_id", ciJob.JobId, "job_name", ciJob.JobName)
				return nil
			}
			files, gcsErr = gcsClient.GetJobJunitContent(ctx, orgForGCS, repoForGCS, pullNumber, ciJob.JobId, "presubmit", ciJob.JobName, junitRegex)
		} else {
			// Postsubmit: need org and repo, but no PR number
			files, gcsErr = gcsClient.GetJobJunitContent(ctx, orgForGCS, repoForGCS, "", ciJob.JobId, "postsubmit", ciJob.JobName, junitRegex)
		}
	}

	if gcsErr != nil {
		logger.Info("GCS listing error (partial results may be returned)", "error", gcsErr, "job_id", ciJob.JobId, "files_found", len(files))
	}

	return files
}

// extractOrgRepoForGCS extracts organization and repository names for GCS path construction.
//
// For non-periodic jobs, this function extracts org/repo from Prow job refs (matching quality-dashboard).
// Falls back to connection values if refs are not available.
//
// Parameters:
//   - job: The source Prow job
//   - githubOrg: Default GitHub organization (used as fallback)
//   - repoName: Default repository name (used as fallback)
//   - jobId: Job ID for logging
//   - logger: Logger for debug messages
//
// Returns:
//   - string: Organization name
//   - string: Repository name
func extractOrgRepoForGCS(job *ProwJob, githubOrg, repoName, jobId string, logger log.Logger) (string, string) {
	if job.Spec.Refs != nil && job.Spec.Refs.Org != "" && job.Spec.Refs.Repo != "" {
		return job.Spec.Refs.Org, job.Spec.Refs.Repo
	}

	// Fallback to connection values
	logger.Debug("Using connection org/repo as fallback", "org", githubOrg, "repo", repoName, "job_id", jobId)
	return githubOrg, repoName
}

// parseAndSaveJUnitSuites parses JUnit XML, logs comprehensive test suite information, and saves to database.
//
// Parameters:
//   - taskCtx: The subtask context (for database access)
//   - logger: Logger for output
//   - suites: JUnit XML content (can be nil)
//   - xmlFileName: Name of the XML file (for logging)
//   - ciJob: The CI job model
//   - githubOrg: GitHub organization (for logging)
//   - repoName: Repository name (for logging)
//
// Returns:
//   - bool: true if JUnit XML was successfully parsed, logged, and saved, false otherwise
func parseAndSaveJUnitSuites(taskCtx plugin.SubTaskContext, logger log.Logger, suites []byte, xmlFileName string, ciJob *models.TestRegistryCIJob, githubOrg, repoName string) bool {
	if len(suites) == 0 {
		logger.Info("No JUnit XML found for job", "job_id", ciJob.JobId, "job_name", ciJob.JobName, "trigger_type", ciJob.TriggerType)
		return false
	}

	// Parse XML — JUnit files can have either <testsuites> (wrapper) or bare <testsuite> as root
	var suitesXml TestSuites
	if err := xml.Unmarshal(suites, &suitesXml); err != nil {
		logger.Debug("failed to parse JUnit XML", "error", err, "job_id", ciJob.JobId, "xml_file", xmlFileName)
		return false
	}

	// If no suites found, try parsing as a single bare <testsuite> root element
	// (e.g., prowjob_junit.xml uses <testsuite> instead of <testsuites>)
	if len(suitesXml.Suites) == 0 {
		var singleSuite TestSuite
		if err := xml.Unmarshal(suites, &singleSuite); err == nil && singleSuite.Name != "" {
			suitesXml.Suites = []*TestSuite{&singleSuite}
		}
	}

	// Log job context
	logger.Info("JUnit XML found for job",
		"job_id", ciJob.JobId,
		"job_name", ciJob.JobName,
		"organization", githubOrg,
		"repository", repoName,
		"trigger_type", ciJob.TriggerType,
		"xml_file", xmlFileName,
		"result", ciJob.Result)

	// Check if we have any suites
	if len(suitesXml.Suites) == 0 {
		logger.Info("No test suites found in JUnit XML", "job_id", ciJob.JobId, "job_name", ciJob.JobName, "xml_file", xmlFileName)
		return false
	}

	logger.Info("Processing test suites", "job_id", ciJob.JobId, "total_suites", len(suitesXml.Suites))

	// Get database connection
	db := taskCtx.GetDal()

	// Process and save each suite (including nested ones)
	savedSuites := 0
	savedTestCases := 0
	for idx, suite := range suitesXml.Suites {
		if suite != nil && suite.Name != "" {
			logSuiteInfo(logger, suite, ciJob.JobId, idx+1, 0)

			// Save top-level suite and all nested suites recursively
			suiteCount, testCaseCount := saveSuiteRecursively(db, logger, suite, ciJob.ConnectionId, ciJob.JobId, nil)
			savedSuites += suiteCount
			savedTestCases += testCaseCount
		}
	}

	logger.Info("Saved JUnit data to database",
		"job_id", ciJob.JobId,
		"suites_saved", savedSuites,
		"test_cases_saved", savedTestCases)

	return true
}

// logSuiteInfo logs information about a test suite.
//
// Parameters:
//   - logger: Logger for output
//   - suite: The test suite to log
//   - jobId: Job ID for correlation
//   - suiteIndex: Index of the suite (1-based)
//   - depth: Nesting depth (0 for top-level)
func logSuiteInfo(logger log.Logger, suite *TestSuite, jobId string, suiteIndex int, depth int) {
	logger.Info("Test Suite",
		"job_id", jobId,
		"suite_index", suiteIndex,
		"suite_name", suite.Name,
		"tests", suite.NumTests,
		"failures", suite.NumFailed,
		"skipped", suite.NumSkipped,
		"duration_sec", suite.Duration)
}

// generateUID generates a unique identifier for database records using crypto/rand.
func generateUID() string {
	b := make([]byte, uidLength)
	randomBytes := make([]byte, uidLength)
	if _, err := rand.Read(randomBytes); err != nil {
		// crypto/rand.Read only fails on catastrophic system issues; use fallback
		for i := range b {
			b[i] = uidChars[0]
		}
		return string(b)
	}
	for i := range b {
		b[i] = uidChars[int(randomBytes[i])%len(uidChars)]
	}
	return string(b)
}

// saveSuiteRecursively saves a test suite and all its nested suites and test cases to the database.
//
// This function recursively processes nested suites and saves them with proper parent-child relationships.
//
// Parameters:
//   - db: Database connection
//   - logger: Logger for output
//   - suite: The test suite XML structure to save
//   - connectionId: The DevLake connection ID
//   - jobId: The CI job ID
//   - parentSuiteId: The parent suite ID (nil for top-level suites)
//
// Returns:
//   - int: Number of suites saved (including nested ones)
//   - int: Number of test cases saved
func saveSuiteRecursively(db dal.Dal, logger log.Logger, suite *TestSuite, connectionId uint64, jobId string, parentSuiteId *string) (int, int) {
	if suite == nil || suite.Name == "" {
		return 0, 0
	}

	// Always create a new suite — dedup across JUnit files is intentionally skipped so that
	// suites with the same name from different files (e.g., same test suite run with different
	// parameters) are stored independently. The job-level isJobAlreadyProcessed check prevents
	// re-processing across blueprint runs.
	suiteId := generateUID()

	// Convert properties to JSON string
	propertiesJSON := ""
	if len(suite.Properties) > 0 {
		propertiesBytes, err := json.Marshal(suite.Properties)
		if err != nil {
			logger.Debug("failed to marshal suite properties", "suite_name", suite.Name, "job_id", jobId, "error", err)
		} else {
			propertiesJSON = string(propertiesBytes)
		}
	}

	// Create database model
	testSuite := &models.TestSuite{
		ConnectionId:  connectionId,
		JobId:         jobId,
		SuiteId:       suiteId,
		Name:          suite.Name,
		NumTests:      suite.NumTests,
		NumSkipped:    suite.NumSkipped,
		NumFailed:     suite.NumFailed,
		Duration:      suite.Duration,
		Properties:    propertiesJSON,
		ParentSuiteId: parentSuiteId,
	}

	// Save suite to database
	if err := db.CreateOrUpdate(testSuite); err != nil {
		logger.Warn(err, "failed to save test suite", "suite_id", suiteId, "suite_name", suite.Name, "job_id", jobId)
		return 0, 0
	}

	suiteCount := 1
	testCaseCount := 0

	// Save test cases for this suite
	for _, testCase := range suite.TestCases {
		if testCase != nil {
			if err := saveTestCase(db, logger, testCase, connectionId, jobId, suiteId); err == nil {
				testCaseCount++
			}
		}
	}

	// Recursively save nested suites
	for _, child := range suite.Children {
		if child != nil {
			childSuiteId := suiteId // Pass current suite ID as parent
			nestedSuiteCount, nestedTestCaseCount := saveSuiteRecursively(db, logger, child, connectionId, jobId, &childSuiteId)
			suiteCount += nestedSuiteCount
			testCaseCount += nestedTestCaseCount
		}
	}

	return suiteCount, testCaseCount
}

// saveTestCase saves a single test case to the database.
//
// Parameters:
//   - db: Database connection
//   - logger: Logger for output
//   - testCase: The test case XML structure to save
//   - connectionId: The DevLake connection ID
//   - jobId: The CI job ID
//   - suiteId: The parent suite ID
//
// Returns:
//   - errors.Error: Any error encountered during saving, or nil if successful
func saveTestCase(db dal.Dal, logger log.Logger, testCase *TestCase, connectionId uint64, jobId, suiteId string) errors.Error {
	// Always create a new test case — each suite has a unique ID so test cases are
	// naturally scoped to their source JUnit file. No cross-file dedup needed.
	testCaseId := generateUID()

	// Determine test case status
	status := "passed"
	var failureMessage, failureOutput *string
	var skipMessage *string

	if testCase.FailureOutput != nil {
		status = "failed"
		failureMsg := testCase.FailureOutput.Message
		failureMessage = &failureMsg
		failureOut := testCase.FailureOutput.Output
		failureOutput = &failureOut
	} else if testCase.SkipMessage != nil {
		status = "skipped"
		skipMsg := testCase.SkipMessage.Message
		skipMessage = &skipMsg
	}

	// Create database model
	testCaseModel := &models.TestCase{
		ConnectionId:   connectionId,
		JobId:          jobId,
		SuiteId:        suiteId,
		TestCaseId:     testCaseId,
		Name:           testCase.Name,
		Classname:      testCase.Classname,
		Duration:       testCase.Duration,
		Status:         status,
		FailureMessage: failureMessage,
		FailureOutput:  failureOutput,
		SkipMessage:    skipMessage,
		SystemOut:      stringPtrOrNil(testCase.SystemOut),
		SystemErr:      stringPtrOrNil(testCase.SystemErr),
	}

	// Save test case to database
	if err := db.CreateOrUpdate(testCaseModel); err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("failed to save test case %s", testCase.Name))
	}

	return nil
}

// stringPtrOrNil converts a string to a pointer, returning nil if the string is empty
func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
