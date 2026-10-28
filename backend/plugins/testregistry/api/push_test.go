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
	"bytes"
	"encoding/xml"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/testregistry/tasks"
)

func TestParseJUnitXML_TestSuites(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="step graph" tests="3" failures="1" skipped="0" time="456.5">
    <testcase name="pre phase" time="120.3"/>
    <testcase name="test phase" time="300.2">
      <failure message="test failed">Expected healthy cluster</failure>
    </testcase>
    <testcase name="post phase" time="36.0"/>
  </testsuite>
</testsuites>`)

	var suitesXml tasks.TestSuites
	if err := xml.Unmarshal(xmlContent, &suitesXml); err != nil {
		t.Fatalf("Failed to parse testsuites XML: %v", err)
	}

	if len(suitesXml.Suites) != 1 {
		t.Fatalf("Expected 1 suite, got %d", len(suitesXml.Suites))
	}

	suite := suitesXml.Suites[0]
	if suite.Name != "step graph" {
		t.Errorf("Suite name = %q, want %q", suite.Name, "step graph")
	}
	if suite.NumTests != 3 {
		t.Errorf("Suite NumTests = %d, want 3", suite.NumTests)
	}
	if suite.NumFailed != 1 {
		t.Errorf("Suite NumFailed = %d, want 1", suite.NumFailed)
	}
	if len(suite.TestCases) != 3 {
		t.Fatalf("Expected 3 test cases, got %d", len(suite.TestCases))
	}

	if suite.TestCases[0].Name != "pre phase" {
		t.Errorf("TestCase[0].Name = %q, want %q", suite.TestCases[0].Name, "pre phase")
	}
	if suite.TestCases[0].FailureOutput != nil {
		t.Error("TestCase[0] should not have failure output")
	}

	if suite.TestCases[1].FailureOutput == nil {
		t.Fatal("TestCase[1] should have failure output")
	}
	if suite.TestCases[1].FailureOutput.Message != "test failed" {
		t.Errorf("TestCase[1].FailureOutput.Message = %q, want %q", suite.TestCases[1].FailureOutput.Message, "test failed")
	}
}

func TestParseJUnitXML_BareTestSuite(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="e2e-console" tests="3" failures="0" skipped="1" time="60">
  <testcase name="render dashboard" classname="DashboardPage" time="12.3"/>
  <testcase name="create project" classname="ProjectPage" time="8.7"/>
  <testcase name="test GPU" classname="GPUPage" time="0">
    <skipped message="GPU not available"/>
  </testcase>
</testsuite>`)

	var suitesXml tasks.TestSuites
	err := xml.Unmarshal(xmlContent, &suitesXml)
	if err == nil && len(suitesXml.Suites) > 0 {
		t.Fatal("Expected TestSuites parse to fail or return empty suites for bare <testsuite>")
	}

	var singleSuite tasks.TestSuite
	if err := xml.Unmarshal(xmlContent, &singleSuite); err != nil {
		t.Fatalf("Failed to parse bare testsuite XML: %v", err)
	}

	if singleSuite.Name != "e2e-console" {
		t.Errorf("Suite name = %q, want %q", singleSuite.Name, "e2e-console")
	}
	if singleSuite.NumTests != 3 {
		t.Errorf("Suite NumTests = %d, want 3", singleSuite.NumTests)
	}
	if singleSuite.NumSkipped != 1 {
		t.Errorf("Suite NumSkipped = %d, want 1", singleSuite.NumSkipped)
	}
	if len(singleSuite.TestCases) != 3 {
		t.Fatalf("Expected 3 test cases, got %d", len(singleSuite.TestCases))
	}

	if singleSuite.TestCases[0].Classname != "DashboardPage" {
		t.Errorf("TestCase[0].Classname = %q, want %q", singleSuite.TestCases[0].Classname, "DashboardPage")
	}

	if singleSuite.TestCases[2].SkipMessage == nil {
		t.Fatal("TestCase[2] should have skip message")
	}
	if singleSuite.TestCases[2].SkipMessage.Message != "GPU not available" {
		t.Errorf("TestCase[2].SkipMessage = %q, want %q", singleSuite.TestCases[2].SkipMessage.Message, "GPU not available")
	}
}

func TestParseJUnitXML_EmptyTestSuites(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
</testsuites>`)

	var suitesXml tasks.TestSuites
	if err := xml.Unmarshal(xmlContent, &suitesXml); err != nil {
		t.Fatalf("Failed to parse empty testsuites XML: %v", err)
	}

	if len(suitesXml.Suites) != 0 {
		t.Errorf("Expected 0 suites, got %d", len(suitesXml.Suites))
	}
}

func TestParseJUnitXML_InvalidXML(t *testing.T) {
	xmlContent := []byte(`this is not XML at all`)

	var suitesXml tasks.TestSuites
	err := xml.Unmarshal(xmlContent, &suitesXml)
	if err == nil {
		t.Error("Expected error parsing invalid XML")
	}
}

func TestParseJUnitXML_MultipleSuites(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="unit" tests="2" failures="0" time="5">
    <testcase name="TestAdd" time="0.1"/>
    <testcase name="TestSub" time="0.2"/>
  </testsuite>
  <testsuite name="integration" tests="1" failures="1" time="300">
    <testcase name="TestE2E" time="300">
      <failure message="timeout">Connection refused</failure>
    </testcase>
  </testsuite>
</testsuites>`)

	var suitesXml tasks.TestSuites
	if err := xml.Unmarshal(xmlContent, &suitesXml); err != nil {
		t.Fatalf("Failed to parse multi-suite XML: %v", err)
	}

	if len(suitesXml.Suites) != 2 {
		t.Fatalf("Expected 2 suites, got %d", len(suitesXml.Suites))
	}

	if suitesXml.Suites[0].Name != "unit" {
		t.Errorf("Suite[0].Name = %q, want %q", suitesXml.Suites[0].Name, "unit")
	}
	if suitesXml.Suites[1].Name != "integration" {
		t.Errorf("Suite[1].Name = %q, want %q", suitesXml.Suites[1].Name, "integration")
	}
	if suitesXml.Suites[1].TestCases[0].FailureOutput == nil {
		t.Error("Suite[1].TestCase[0] should have failure output")
	}
}

func TestTestCaseStatusDetection(t *testing.T) {
	xmlContent := []byte(`<testsuites>
  <testsuite name="status-test" tests="3" failures="1" skipped="1">
    <testcase name="passed-test" time="1.0"/>
    <testcase name="failed-test" time="2.0">
      <failure message="assertion failed">expected true, got false</failure>
    </testcase>
    <testcase name="skipped-test">
      <skipped message="not ready"/>
    </testcase>
  </testsuite>
</testsuites>`)

	var suitesXml tasks.TestSuites
	if err := xml.Unmarshal(xmlContent, &suitesXml); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	cases := suitesXml.Suites[0].TestCases
	if len(cases) != 3 {
		t.Fatalf("Expected 3 cases, got %d", len(cases))
	}

	if cases[0].FailureOutput != nil || cases[0].SkipMessage != nil {
		t.Error("passed-test should have no failure/skip")
	}

	if cases[1].FailureOutput == nil {
		t.Fatal("failed-test should have failure output")
	}
	if cases[1].FailureOutput.Message != "assertion failed" {
		t.Errorf("failure message = %q, want %q", cases[1].FailureOutput.Message, "assertion failed")
	}
	if cases[1].FailureOutput.Output != "expected true, got false" {
		t.Errorf("failure output = %q, want %q", cases[1].FailureOutput.Output, "expected true, got false")
	}

	if cases[2].SkipMessage == nil {
		t.Fatal("skipped-test should have skip message")
	}
	if cases[2].SkipMessage.Message != "not ready" {
		t.Errorf("skip message = %q, want %q", cases[2].SkipMessage.Message, "not ready")
	}
}

func TestMaxJUnitFilesConstant(t *testing.T) {
	if maxJUnitFilesPerRequest != 100 {
		t.Errorf("maxJUnitFilesPerRequest = %d, want 100", maxJUnitFilesPerRequest)
	}
}

type errorReader struct{}

func (errorReader) Read(_ []byte) (int, error) {
	return 0, fmt.Errorf("simulated rand failure")
}

func TestGenerateUID_ReturnsErrorOnReadFailure(t *testing.T) {
	_, err := generateUIDFrom(errorReader{})
	if err == nil {
		t.Fatal("generateUIDFrom should return error when reader fails")
	}
	if !strings.Contains(err.Error(), "failed to generate random UID") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerateUID_Success(t *testing.T) {
	id, err := generateUID()
	if err != nil {
		t.Fatalf("generateUID() returned unexpected error: %v", err)
	}
	if len(id) != 16 {
		t.Errorf("generateUID() length = %d, want 16", len(id))
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateUID() contains non-hex character: %c in %s", c, id)
		}
	}
}

// makeMultipartRequest builds a POST request with the given form fields.
func makeMultipartRequest(t *testing.T, fields map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField(%q): %v", k, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("multipart.Writer.Close: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/test_results", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestPostTestResultsImpl_NilRequest(t *testing.T) {
	_, err := postTestResultsImpl(&plugin.ApiResourceInput{Request: nil}, 1)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
}

func TestPostTestResultsImpl_MissingRequiredFields(t *testing.T) {
	allFields := map[string]string{
		"jobId":        "job-1",
		"jobName":      "build",
		"organization": "my-org",
		"repository":   "my-repo",
		"result":       "SUCCESS",
	}

	for _, missing := range []string{"jobId", "jobName", "organization", "repository", "result"} {
		t.Run("missing_"+missing, func(t *testing.T) {
			fields := make(map[string]string, len(allFields))
			for k, v := range allFields {
				fields[k] = v
			}
			delete(fields, missing)

			input := &plugin.ApiResourceInput{
				Request: makeMultipartRequest(t, fields),
			}
			_, err := postTestResultsImpl(input, 1)
			if err == nil {
				t.Errorf("expected error when %q is missing, got nil", missing)
			}
		})
	}
}
