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

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/dbhelper"
	"github.com/apache/devlake/plugins/testregistry/models"
	"github.com/apache/devlake/plugins/testregistry/tasks"
)

const maxJUnitFilesPerRequest = 100

// PostTestResults handles pushing CI test results via the testregistry plugin by connection ID.
// Accepts multipart/form-data with job metadata as form fields and JUnit XML as file uploads.
func PostTestResults(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.TestRegistryConnection{}
	err := connectionHelper.First(connection, input.Params)
	if err != nil {
		return nil, err
	}
	return postTestResultsImpl(input, connection.ID)
}

// PostTestResultsByName handles pushing CI test results via the testregistry plugin by connection name.
func PostTestResultsByName(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.TestRegistryConnection{}
	err := connectionHelper.FirstByName(connection, input.Params)
	if err != nil {
		return nil, err
	}
	return postTestResultsImpl(input, connection.ID)
}

func postTestResultsImpl(input *plugin.ApiResourceInput, connectionId uint64) (*plugin.ApiResourceOutput, errors.Error) {
	if input.Request == nil {
		return nil, errors.BadInput.New("request must be multipart/form-data with job metadata as form fields and JUnit XML as file uploads (field name: junit)")
	}

	if err := input.Request.ParseMultipartForm(32 << 20); err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to parse multipart form")
	}

	// Read required form fields
	jobId := input.Request.FormValue("jobId")
	jobName := input.Request.FormValue("jobName")
	organization := input.Request.FormValue("organization")
	repository := input.Request.FormValue("repository")
	result := input.Request.FormValue("result")

	if jobId == "" || jobName == "" || organization == "" || repository == "" || result == "" {
		return nil, errors.BadInput.New("required form fields: jobId, jobName, organization, repository, result")
	}

	// Validate field lengths to prevent DB column overflow (job_id is varchar(255), job_name is varchar(500))
	domainJobId := fmt.Sprintf("testregistry:%d:%s", connectionId, jobId)
	if len(domainJobId) > 255 {
		return nil, errors.BadInput.New("jobId is too long; the prefixed ID must fit in 255 characters")
	}

	// Read optional form fields
	jobType := input.Request.FormValue("jobType")
	commitSha := input.Request.FormValue("commitSha")
	pullRequestAuthor := input.Request.FormValue("pullRequestAuthor")
	triggerType := input.Request.FormValue("triggerType")
	viewUrl := input.Request.FormValue("viewUrl")
	scopeId := input.Request.FormValue("scopeId")

	var pullRequestNumber *int
	if prStr := input.Request.FormValue("pullRequestNumber"); prStr != "" {
		prNum, parseErr := strconv.Atoi(prStr)
		if parseErr != nil {
			return nil, errors.BadInput.New(fmt.Sprintf("pullRequestNumber must be a valid integer, got %q", prStr))
		}
		pullRequestNumber = &prNum
	}

	var startedAt, finishedAt *time.Time
	if s := input.Request.FormValue("startedAt"); s != "" {
		t, parseErr := common.ConvertStringToTime(s)
		if parseErr != nil {
			return nil, errors.BadInput.New(fmt.Sprintf("startedAt must be a valid ISO 8601 timestamp, got %q", s))
		}
		startedAt = &t
	}
	if s := input.Request.FormValue("finishedAt"); s != "" {
		t, parseErr := common.ConvertStringToTime(s)
		if parseErr != nil {
			return nil, errors.BadInput.New(fmt.Sprintf("finishedAt must be a valid ISO 8601 timestamp, got %q", s))
		}
		finishedAt = &t
	}

	var durationSec *float64
	if s := input.Request.FormValue("durationSec"); s != "" {
		d, parseErr := strconv.ParseFloat(s, 64)
		if parseErr != nil {
			return nil, errors.BadInput.New(fmt.Sprintf("durationSec must be a valid number, got %q", s))
		}
		durationSec = &d
	} else if startedAt != nil && finishedAt != nil {
		d := finishedAt.Sub(*startedAt).Seconds()
		durationSec = &d
	}

	if scopeId == "" {
		scopeId = repository
	}

	// Enforce file count limit
	junitFiles := input.Request.MultipartForm.File["junit"]
	if len(junitFiles) > maxJUnitFilesPerRequest {
		return nil, errors.BadInput.New(fmt.Sprintf("too many JUnit files; maximum is %d", maxJUnitFilesPerRequest))
	}

	// Clean up multipart temp files when done
	defer func() {
		if input.Request.MultipartForm != nil {
			_ = input.Request.MultipartForm.RemoveAll()
		}
	}()

	// Wrap all writes in a transaction for consistency
	var err errors.Error
	txHelper := dbhelper.NewTxHelper(basicRes, &err)
	defer txHelper.End()
	db := txHelper.Begin()

	// Delete existing suites and cases for this job to ensure idempotent retries
	if delErr := db.Delete(&models.TestCase{}, dal.Where("connection_id = ? AND job_id = ?", connectionId, domainJobId)); delErr != nil {
		err = errors.Default.Wrap(delErr, "failed to delete existing test cases")
		return nil, err
	}
	if delErr := db.Delete(&models.TestSuite{}, dal.Where("connection_id = ? AND job_id = ?", connectionId, domainJobId)); delErr != nil {
		err = errors.Default.Wrap(delErr, "failed to delete existing test suites")
		return nil, err
	}

	// Save CI job
	ciJob := &models.TestRegistryCIJob{
		ConnectionId:      connectionId,
		JobId:             domainJobId,
		JobName:           jobName,
		JobType:           jobType,
		Organization:      organization,
		Repository:        repository,
		CommitSHA:         commitSha,
		PullRequestNumber: pullRequestNumber,
		PullRequestAuthor: pullRequestAuthor,
		TriggerType:       triggerType,
		Result:            result,
		StartedAt:         startedAt,
		FinishedAt:        finishedAt,
		DurationSec:       durationSec,
		ViewURL:           viewUrl,
		ScopeId:           scopeId,
	}

	if dbErr := db.CreateOrUpdate(ciJob); dbErr != nil {
		err = errors.Default.Wrap(dbErr, "failed to save CI job")
		return nil, err
	}

	savedSuites := 0
	savedCases := 0

	// Parse JUnit XML files from multipart uploads
	for _, fileHeader := range junitFiles {
		f, openErr := fileHeader.Open()
		if openErr != nil {
			err = errors.Default.Wrap(openErr, fmt.Sprintf("failed to open uploaded file %s", fileHeader.Filename))
			return nil, err
		}

		xmlContent, readErr := io.ReadAll(io.LimitReader(f, 10*1024*1024))
		_ = f.Close()
		if readErr != nil {
			err = errors.Default.Wrap(readErr, fmt.Sprintf("failed to read uploaded file %s", fileHeader.Filename))
			return nil, err
		}

		// Parse XML — try <testsuites> first, then bare <testsuite>
		var suitesXml tasks.TestSuites
		firstErr := xml.Unmarshal(xmlContent, &suitesXml)

		// Handle bare <testsuite> root element (or failed <testsuites> parse)
		if len(suitesXml.Suites) == 0 {
			var singleSuite tasks.TestSuite
			if xmlErr := xml.Unmarshal(xmlContent, &singleSuite); xmlErr == nil && singleSuite.Name != "" {
				suitesXml.Suites = []*tasks.TestSuite{&singleSuite}
			}
		}

		// Return error if file could not be parsed as JUnit XML
		if len(suitesXml.Suites) == 0 {
			errMsg := fmt.Sprintf("failed to parse JUnit XML from file %s", fileHeader.Filename)
			if firstErr != nil {
				err = errors.BadInput.Wrap(firstErr, errMsg)
			} else {
				err = errors.BadInput.New(errMsg + ": no test suites found")
			}
			return nil, err
		}

		// Save each suite and its test cases
		for _, suite := range suitesXml.Suites {
			if suite == nil || suite.Name == "" {
				continue
			}

			suiteId, uidErr := generateUID()
			if uidErr != nil {
				err = uidErr
				return nil, err
			}
			testSuite := &models.TestSuite{
				ConnectionId: connectionId,
				JobId:        domainJobId,
				SuiteId:      suiteId,
				Name:         suite.Name,
				NumTests:     suite.NumTests,
				NumFailed:    suite.NumFailed,
				NumSkipped:   suite.NumSkipped,
				Duration:     suite.Duration,
			}

			if dbErr := db.CreateOrUpdate(testSuite); dbErr != nil {
				err = errors.Default.Wrap(dbErr, fmt.Sprintf("failed to save test suite %s", suite.Name))
				return nil, err
			}
			savedSuites++

			for _, tc := range suite.TestCases {
				if tc == nil {
					continue
				}

				testCaseId, uidErr := generateUID()
				if uidErr != nil {
					err = uidErr
					return nil, err
				}
				status := "passed"
				var failureMsg, failureOut, skipMsg *string

				if tc.FailureOutput != nil {
					status = "failed"
					msg := tc.FailureOutput.Message
					failureMsg = &msg
					out := tc.FailureOutput.Output
					failureOut = &out
				} else if tc.SkipMessage != nil {
					status = "skipped"
					msg := tc.SkipMessage.Message
					skipMsg = &msg
				}

				testCase := &models.TestCase{
					ConnectionId:   connectionId,
					JobId:          domainJobId,
					SuiteId:        suiteId,
					TestCaseId:     testCaseId,
					Name:           tc.Name,
					Classname:      tc.Classname,
					Duration:       tc.Duration,
					Status:         status,
					FailureMessage: failureMsg,
					FailureOutput:  failureOut,
					SkipMessage:    skipMsg,
				}

				if dbErr := db.CreateOrUpdate(testCase); dbErr != nil {
					err = errors.Default.Wrap(dbErr, fmt.Sprintf("failed to save test case %s", tc.Name))
					return nil, err
				}
				savedCases++
			}
		}
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"jobId":       domainJobId,
			"suitesSaved": savedSuites,
			"casesSaved":  savedCases,
			"filesRead":   len(junitFiles),
		},
		Status: http.StatusOK,
	}, nil
}

// generateUID creates a random 16-char hex string for unique IDs.
func generateUID() (string, errors.Error) {
	return generateUIDFrom(rand.Reader)
}

func generateUIDFrom(r io.Reader) (string, errors.Error) {
	b := make([]byte, 8)
	if _, readErr := r.Read(b); readErr != nil {
		return "", errors.Default.Wrap(readErr, "failed to generate random UID")
	}
	return hex.EncodeToString(b), nil
}
