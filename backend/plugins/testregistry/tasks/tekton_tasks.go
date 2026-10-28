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
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/testregistry/models"
)

// saveTektonTasks saves all task runs from a Tekton PipelineRun to the database
//
// Parameters:
//   - db: Database connection
//   - logger: Logger for error reporting
//   - connectionId: The DevLake connection ID
//   - jobId: The CI job ID (PipelineRunName)
//   - taskRuns: List of TektonTaskRun objects from pipeline-status.json
//
// Returns:
//   - errors.Error: Any error encountered during saving, or nil if successful
func saveTektonTasks(db dal.Dal, logger log.Logger, connectionId uint64, jobId string, taskRuns []TektonTaskRun) errors.Error {
	for _, taskRun := range taskRuns {
		if taskRun.Name == "" {
			logger.Warn(nil, "Task run missing name, skipping", "job_id", jobId)
			continue
		}

		// Parse duration from string (e.g., "499s") to float64
		var durationSec float64
		if taskRun.Duration != "" {
			// Remove "s" suffix and parse to float64
			durationStr := strings.TrimSuffix(taskRun.Duration, "s")
			if duration, parseErr := strconv.ParseFloat(durationStr, 64); parseErr == nil {
				durationSec = duration
			} else {
				logger.Debug("Failed to parse task duration", "job_id", jobId, "task_name", taskRun.Name, "duration", taskRun.Duration)
			}
		}

		task := &models.TektonTask{
			ConnectionId: connectionId,
			JobId:        jobId,
			TaskName:     taskRun.Name,
			Status:       taskRun.Status,
			DurationSec:  durationSec,
		}

		if err := db.CreateOrUpdate(task); err != nil {
			logger.Warn(err, "failed to save Tekton task", "job_id", jobId, "task_name", taskRun.Name)
			continue
		}

		logger.Debug("Saved Tekton task", "job_id", jobId, "task_name", taskRun.Name, "status", taskRun.Status, "duration_sec", durationSec)
	}

	return nil
}

// findAndProcessJUnitFiles finds JUnit XML files in the artifact directory and processes them
//
// Parameters:
//   - taskCtx: The subtask context
//   - artifactPath: Local path where artifact was pulled (tmp/{uuid}/)
//   - ciJob: The CI job model
//   - organization: The organization name (for logging)
//   - repository: The repository name (for logging)
//   - junitRegex: Compiled regex pattern for matching JUnit file names
//
// Returns:
//   - bool: true if at least one JUnit XML file was found and processed successfully, false otherwise
func findAndProcessJUnitFiles(taskCtx plugin.SubTaskContext, artifactPath string, ciJob *models.TestRegistryCIJob, organization, repository string, junitRegex *regexp.Regexp) bool {
	logger := taskCtx.GetLogger()

	// Use default regex if not provided
	if junitRegex == nil {
		junitRegex = JUnitRegexpSearch
	}

	// Collect all JUnit files found in the artifact directory
	type junitFile struct {
		content  []byte
		fileName string
		path     string
	}
	var junitFiles []junitFile

	// Walk the artifact directory to find all JUnit XML files matching the regex
	err := filepath.Walk(artifactPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Look for files matching the JUnit regex pattern
		if !info.IsDir() {
			fileName := filepath.Base(path)
			if junitRegex.MatchString(fileName) {
				logger.Debug("Found JUnit XML file", "file", fileName, "path", path, "job_id", ciJob.JobId)

				// Read the JUnit XML content
				content, readErr := os.ReadFile(path)
				if readErr != nil {
					logger.Warn(readErr, "failed to read JUnit XML file", "path", path, "job_id", ciJob.JobId)
					return nil // Continue processing other files
				}

				// Collect all matching JUnit files
				junitFiles = append(junitFiles, junitFile{
					content:  content,
					fileName: fileName,
					path:     path,
				})
			}
		}

		return nil
	})

	if err != nil {
		logger.Warn(err, "failed to walk artifact directory for JUnit files", "job_id", ciJob.JobId)
		return false
	}

	if len(junitFiles) == 0 {
		logger.Debug("No JUnit XML files found in artifact", "job_id", ciJob.JobId, "artifact_path", artifactPath)
		return false
	}

	if len(junitFiles) > 1 {
		logger.Info("Found multiple JUnit XML files, processing all", "job_id", ciJob.JobId, "file_count", len(junitFiles))
	} else {
		logger.Info("Found JUnit XML file, processing", "job_id", ciJob.JobId, "file", junitFiles[0].fileName)
	}

	// Process each JUnit file found
	successCount := 0
	for idx, junitFile := range junitFiles {
		logger.Debug("Processing JUnit XML file", "job_id", ciJob.JobId, "file", junitFile.fileName, "index", idx+1, "total", len(junitFiles))

		// Process and save JUnit XML using the same function as Prow
		if parseAndSaveJUnitSuites(taskCtx, logger, junitFile.content, junitFile.fileName, ciJob, organization, repository) {
			successCount++
		} else {
			logger.Warn(nil, "failed to process JUnit XML file", "job_id", ciJob.JobId, "file", junitFile.fileName)
		}
	}

	logger.Info("Finished processing JUnit XML files", "job_id", ciJob.JobId, "total_files", len(junitFiles), "successful", successCount)

	// Return true if at least one file was successfully processed
	return successCount > 0
}
