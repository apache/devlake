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
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/sonarqube/models"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestConvertTimeToMinutes(t *testing.T) {
	testCases := []struct {
		timeStr     string
		expectedMin int
	}{
		//{"1min", 1},
		//{"30min", 30},
		//{"1h30min", 90},
		{"1d1h30min", 570},
		{"3d5h10min", 1750},
	}

	for _, tc := range testCases {
		actualMin := convertTimeToMinutes(tc.timeStr)
		if actualMin != tc.expectedMin {
			t.Errorf("convertTimeToMinutes(%v) = %v; expected %v", tc.timeStr, actualMin, tc.expectedMin)
		}
	}
}

func TestGenerateId(t *testing.T) {
	entity := &models.SonarqubeIssueCodeBlock{
		IssueKey:    "ISSUE-123",
		Component:   "com.example:example-project",
		StartLine:   10,
		EndLine:     20,
		StartOffset: 5,
		EndOffset:   10,
		Msg:         "Example message",
	}
	hashCodeBlock := sha256.New()
	generateId(hashCodeBlock, entity)

	expectedId := "c590f554324b82421b898723e51b4aa9217dc897aa79e5d55b1716df88a5af1e"
	if entity.Id != expectedId {
		t.Errorf("generateId did not generate the expected ID. Got %v, expected %v", entity.Id, expectedId)
	}
}

func TestGenerateIdWithReusedHash(t *testing.T) {
	// the issues extractor shares one hash across all code blocks of the subtask
	hashCodeBlock := sha256.New()
	other := &models.SonarqubeIssueCodeBlock{
		IssueKey:  "ISSUE-456",
		Component: "com.example:other-project",
		StartLine: 1,
		EndLine:   2,
		Msg:       "Other message",
	}
	generateId(hashCodeBlock, other)

	entity := &models.SonarqubeIssueCodeBlock{
		IssueKey:    "ISSUE-123",
		Component:   "com.example:example-project",
		StartLine:   10,
		EndLine:     20,
		StartOffset: 5,
		EndOffset:   10,
		Msg:         "Example message",
	}
	generateId(hashCodeBlock, entity)

	// same ID as TestGenerateId, whatever was hashed before
	expectedId := "c590f554324b82421b898723e51b4aa9217dc897aa79e5d55b1716df88a5af1e"
	if entity.Id != expectedId {
		t.Errorf("generateId depends on the previous code block. Got %v, expected %v", entity.Id, expectedId)
	}

	sameAsOther := *other
	sameAsOther.Id = ""
	generateId(hashCodeBlock, &sameAsOther)
	if sameAsOther.Id != other.Id {
		t.Errorf("generateId gave the same code block two IDs: %v and %v", other.Id, sameAsOther.Id)
	}
}

func TestGetTotalPagesFromResponse(t *testing.T) {
	// mock response body
	responseBody := `{
		"paging": {
			"pageIndex": 1,
			"pageSize": 10,
			"total": 20
		},
		"results": [
			{"id": 1, "name": "project 1"},
			{"id": 2, "name": "project 2"}
		]
	}`

	// create a mock HTTP response with the above body
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader(responseBody)),
	}

	// create mock ApiCollectorArgs
	args := &api.ApiCollectorArgs{
		PageSize: 10,
	}

	// call the function to get the total number of pages
	totalPages, err := GetTotalPagesFromResponse(response, args)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	// verify the result
	expectedPages := 2
	if totalPages != expectedPages {
		t.Fatalf("Expected %v pages, but got %v", expectedPages, totalPages)
	}
}

func TestSetMetrics(t *testing.T) {
	fileMetrics := &models.SonarqubeFileMetrics{}
	metricsList := []Measure{
		{Metric: "sqale_index", Value: "100"},
		{Metric: "sqale_rating", Value: "1.0"},
		{Metric: "reliability_rating", Value: "1.0"},
		{Metric: "security_rating", Value: "2.0"},
		{Metric: "security_review_rating", Value: "3.0"},
		{Metric: "ncloc", Value: "500"},
		{Metric: "code_smells", Value: "10"},
	}

	err := setMetrics(fileMetrics, metricsList)
	if err != nil {
		t.Errorf("setMetrics returned an error: %s", err)
	}

	if fileMetrics.SqaleIndex != 100 {
		t.Errorf("SqaleIndex was not set properly")
	}

	if fileMetrics.SqaleRating != 1.0 {
		t.Errorf("SqaleRating was not set properly")
	}

	if fileMetrics.ReliabilityRating != "A" {
		t.Errorf("ReliabilityRating was not set properly")
	}

	if fileMetrics.SecurityRating != "B" {
		t.Errorf("SecurityRating was not set properly")
	}

	if fileMetrics.SecurityReviewRating != "C" {
		t.Errorf("SecurityReviewRating was not set properly")
	}

	if fileMetrics.Ncloc != 500 {
		t.Errorf("Ncloc was not set properly")
	}

	if fileMetrics.CodeSmells != 10 {
		t.Errorf("CodeSmells was not set properly")
	}
}
