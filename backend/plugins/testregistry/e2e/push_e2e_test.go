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

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/plugin"
	pluginhelper "github.com/apache/devlake/helpers/pluginhelper/api"
	trmodels "github.com/apache/devlake/plugins/testregistry/models"
	testregistry "github.com/apache/devlake/plugins/testregistry/impl"
	"github.com/apache/devlake/test/helper"
	"github.com/stretchr/testify/require"
)

const junitSingle = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="unit" tests="3" failures="1" skipped="0" time="10">
    <testcase name="TestPass" classname="pkg.SomeTest" time="1.0"/>
    <testcase name="TestFail" classname="pkg.SomeTest" time="2.0">
      <failure message="assertion failed">expected 1, got 2</failure>
    </testcase>
    <testcase name="TestSkip" classname="pkg.SomeTest" time="0">
      <skipped message="not ready"/>
    </testcase>
  </testsuite>
</testsuites>`

// postTestResults sends a multipart POST to the testregistry push endpoint.
// fields are form fields; junitXML is optional JUnit content (empty string = no file).
func postTestResults(t *testing.T, endpoint string, connID uint64, fields map[string]string, junitXML string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	if junitXML != "" {
		fw, err := w.CreateFormFile("junit", "results.xml")
		require.NoError(t, err)
		_, err = io.WriteString(fw, junitXML)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	url := fmt.Sprintf("%s/plugins/testregistry/connections/%d/test_results", endpoint, connID)
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func countRows[T any](t *testing.T, db dal.Dal, dest *[]T, clauses ...dal.Clause) {
	t.Helper()
	require.NoError(t, db.All(dest, clauses...))
}

func TestTestRegistryPush(t *testing.T) {
	client := helper.StartDevLakeServer(t, []plugin.PluginMeta{
		testregistry.TestRegistry{},
	})
	db := client.GetDal()

	conn := client.CreateConnection("testregistry", trmodels.TestRegistryConnection{
		BaseConnection: pluginhelper.BaseConnection{Name: "e2e-push"},
		CITool:         trmodels.CIToolOpenshiftCI,
		Project:        "e2e-project",
		GitHubOrganization: "e2e-org",
		GitHubToken:    "fake-token",
	})
	connID := conn.ID

	baseFields := map[string]string{
		"jobId":        "e2e-job-1",
		"jobName":      "e2e-unit-tests",
		"organization": "test-org",
		"repository":   "test-repo",
		"result":       "FAILURE",
		"jobType":      "github_actions",
		"triggerType":  "pull_request",
	}

	t.Run("happy_path_with_junit", func(t *testing.T) {
		resp := postTestResults(t, client.Endpoint, connID, baseFields, junitSingle)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.Equal(t, float64(1), body["suitesSaved"])
		require.Equal(t, float64(3), body["casesSaved"])
		require.Equal(t, float64(1), body["filesRead"])

		// Verify ci_test_jobs
		var jobs []trmodels.TestRegistryCIJob
		countRows(t, db, &jobs, dal.Where("connection_id = ? AND job_name = ?", connID, "e2e-unit-tests"))
		require.Len(t, jobs, 1)
		require.Equal(t, "FAILURE", jobs[0].Result)
		require.Equal(t, "test-org", jobs[0].Organization)

		// Verify ci_test_suites
		domainJobID := fmt.Sprintf("testregistry:%d:e2e-job-1", connID)
		var suites []trmodels.TestSuite
		countRows(t, db, &suites, dal.Where("connection_id = ? AND job_id = ?", connID, domainJobID))
		require.Len(t, suites, 1)
		require.Equal(t, "unit", suites[0].Name)
		require.Equal(t, uint(3), suites[0].NumTests)
		require.Equal(t, uint(1), suites[0].NumFailed)

		// Verify ci_test_cases
		var cases []trmodels.TestCase
		countRows(t, db, &cases, dal.Where("connection_id = ? AND job_id = ?", connID, domainJobID))
		require.Len(t, cases, 3)
		statuses := make(map[string]int)
		for _, c := range cases {
			statuses[c.Status]++
		}
		require.Equal(t, 1, statuses["passed"])
		require.Equal(t, 1, statuses["failed"])
		require.Equal(t, 1, statuses["skipped"])
	})

	t.Run("no_junit_files_only_job_saved", func(t *testing.T) {
		fields := map[string]string{
			"jobId":        "e2e-job-build-only",
			"jobName":      "e2e-build",
			"organization": "test-org",
			"repository":   "test-repo",
			"result":       "SUCCESS",
		}
		resp := postTestResults(t, client.Endpoint, connID, fields, "")
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.Equal(t, float64(0), body["suitesSaved"])
		require.Equal(t, float64(0), body["casesSaved"])

		var jobs []trmodels.TestRegistryCIJob
		countRows(t, db, &jobs, dal.Where("connection_id = ? AND job_name = ?", connID, "e2e-build"))
		require.Len(t, jobs, 1)
	})

	t.Run("idempotent_on_repeat_post", func(t *testing.T) {
		resp1 := postTestResults(t, client.Endpoint, connID, baseFields, junitSingle)
		resp1.Body.Close()
		require.Equal(t, http.StatusOK, resp1.StatusCode)

		resp2 := postTestResults(t, client.Endpoint, connID, baseFields, junitSingle)
		resp2.Body.Close()
		require.Equal(t, http.StatusOK, resp2.StatusCode)

		var jobs []trmodels.TestRegistryCIJob
		countRows(t, db, &jobs, dal.Where("connection_id = ? AND job_name = ?", connID, "e2e-unit-tests"))
		require.Len(t, jobs, 1, "repeated POST must not duplicate ci_test_jobs rows")
	})

	t.Run("missing_required_field_no_db_write", func(t *testing.T) {
		var jobsBefore []trmodels.TestRegistryCIJob
		countRows(t, db, &jobsBefore, dal.Where("connection_id = ?", connID))

		incompleteFields := map[string]string{
			// missing jobId
			"jobName":      "should-not-be-saved",
			"organization": "test-org",
			"repository":   "test-repo",
			"result":       "SUCCESS",
		}
		resp := postTestResults(t, client.Endpoint, connID, incompleteFields, "")
		resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var jobsAfter []trmodels.TestRegistryCIJob
		countRows(t, db, &jobsAfter, dal.Where("connection_id = ?", connID))
		require.Equal(t, len(jobsBefore), len(jobsAfter), "failed request must not write any rows")
	})
}
