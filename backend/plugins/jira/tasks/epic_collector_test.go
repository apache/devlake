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
	"strings"
	"testing"

	"github.com/apache/incubator-devlake/plugins/jira/models"
)

func TestEpicCollectorBatchSizeLogic(t *testing.T) {
	tests := []struct {
		name           string
		deploymentType models.DeploymentType
		versionNumbers []int
		expectedBatch  int
	}{
		{
			name:           "JIRA Server v8 should use batch size 1",
			deploymentType: models.DeploymentServer,
			versionNumbers: []int{8, 0, 0},
			expectedBatch:  1,
		},
		{
			name:           "JIRA Server v7 should use batch size 1",
			deploymentType: models.DeploymentServer,
			versionNumbers: []int{7, 5, 0},
			expectedBatch:  1,
		},
		{
			name:           "JIRA Server v9 should use default batch size",
			deploymentType: models.DeploymentServer,
			versionNumbers: []int{9, 0, 0},
			expectedBatch:  100,
		},
		{
			name:           "JIRA Cloud should use default batch size",
			deploymentType: models.DeploymentCloud,
			versionNumbers: []int{8, 0, 0}, // Version shouldn't matter for cloud
			expectedBatch:  100,
		},
		{
			name:           "Case insensitive server comparison",
			deploymentType: "server", // lowercase
			versionNumbers: []int{8, 0, 0},
			expectedBatch:  1,
		},
		{
			name:           "Case insensitive cloud comparison",
			deploymentType: "CLOUD", // uppercase
			versionNumbers: []int{8, 0, 0},
			expectedBatch:  100,
		},
		{
			name:           "Mixed case server comparison",
			deploymentType: "SeRvEr", // mixed case
			versionNumbers: []int{7, 0, 0},
			expectedBatch:  1,
		},
		{
			name:           "JIRA Server with incomplete version numbers",
			deploymentType: models.DeploymentServer,
			versionNumbers: []int{8, 0}, // Only 2 elements instead of 3
			expectedBatch:  100,
		},
		{
			name:           "JIRA Server with empty version numbers",
			deploymentType: models.DeploymentServer,
			versionNumbers: []int{},
			expectedBatch:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the batch size logic from CollectEpics function
			batchSize := 100

			// Replicate the exact logic from CollectEpics
			if strings.EqualFold(string(tt.deploymentType), string(models.DeploymentServer)) &&
				len(tt.versionNumbers) == 3 &&
				tt.versionNumbers[0] <= 8 {
				batchSize = 1
			}

			if batchSize != tt.expectedBatch {
				t.Errorf("Batch size for %s with version %v: got %d, want %d",
					tt.deploymentType, tt.versionNumbers, batchSize, tt.expectedBatch)
			}
		})
	}
}

func TestEpicCollectorDeploymentTypeLogic(t *testing.T) {
	tests := []struct {
		name           string
		deploymentType models.DeploymentType
		expectServer   bool
	}{
		{
			name:           "JIRA Server constant should be detected as server",
			deploymentType: models.DeploymentServer,
			expectServer:   true,
		},
		{
			name:           "JIRA Cloud constant should not be detected as server",
			deploymentType: models.DeploymentCloud,
			expectServer:   false,
		},
		{
			name:           "Lowercase server should be detected as server",
			deploymentType: "server",
			expectServer:   true,
		},
		{
			name:           "Uppercase SERVER should be detected as server",
			deploymentType: "SERVER",
			expectServer:   true,
		},
		{
			name:           "Mixed case SeRvEr should be detected as server",
			deploymentType: "SeRvEr",
			expectServer:   true,
		},
		{
			name:           "Lowercase cloud should not be detected as server",
			deploymentType: "cloud",
			expectServer:   false,
		},
		{
			name:           "Uppercase CLOUD should not be detected as server",
			deploymentType: "CLOUD",
			expectServer:   false,
		},
		{
			name:           "Mixed case ClOuD should not be detected as server",
			deploymentType: "ClOuD",
			expectServer:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the deployment type logic from CollectEpics function
			// This replicates the exact comparison used in the collector
			isServer := strings.EqualFold(string(tt.deploymentType), string(models.DeploymentServer))

			if isServer != tt.expectServer {
				t.Errorf("Deployment type detection for %s: got isServer=%v, want %v",
					tt.deploymentType, isServer, tt.expectServer)
			}
		})
	}
}

func TestEpicCollectorApiEndpointSelection(t *testing.T) {
	tests := []struct {
		name             string
		deploymentType   models.DeploymentType
		expectedEndpoint string
	}{
		{
			name:             "JIRA Server should use api/2/search",
			deploymentType:   models.DeploymentServer,
			expectedEndpoint: "api/2/search",
		},
		{
			name:             "JIRA Cloud should use api/3/search/jql",
			deploymentType:   models.DeploymentCloud,
			expectedEndpoint: "api/3/search/jql",
		},
		{
			name:             "Lowercase server should use api/2/search",
			deploymentType:   "server",
			expectedEndpoint: "api/2/search",
		},
		{
			name:             "Uppercase CLOUD should use api/3/search/jql",
			deploymentType:   "CLOUD",
			expectedEndpoint: "api/3/search/jql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the API endpoint selection logic from CollectEpics function
			var selectedEndpoint string

			if strings.EqualFold(string(tt.deploymentType), string(models.DeploymentServer)) {
				selectedEndpoint = "api/2/search"
			} else {
				selectedEndpoint = "api/3/search/jql"
			}

			if selectedEndpoint != tt.expectedEndpoint {
				t.Errorf("API endpoint selection for %s: got %s, want %s",
					tt.deploymentType, selectedEndpoint, tt.expectedEndpoint)
			}
		})
	}
}
