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
	"testing"

	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/argocd/models"
	"github.com/stretchr/testify/assert"
)

func TestDetectEnvironment_DefaultsToTestingWhenConfigMissing(t *testing.T) {
	env := detectEnvironment(
		&models.ArgocdSyncOperation{ApplicationName: "staging-app"},
		nil,
		nil,
		nil,
	)
	assert.Equal(t, devops.TESTING, env)
}

func TestDetectEnvironment_UsesEnvPatternFallback(t *testing.T) {
	config := &models.ArgocdScopeConfig{
		EnvNamePattern: "",
	}
	env := detectEnvironment(
		&models.ArgocdSyncOperation{ApplicationName: "prod-app"},
		nil,
		config,
		nil,
	)
	assert.Equal(t, devops.PRODUCTION, env)
}

func TestDetectEnvironment_UsesRegexEnricherPriorities(t *testing.T) {
	enricher := api.NewRegexEnricher()
	assert.NoError(t, enricher.TryAdd(devops.PRODUCTION, "(?i)critical"))
	assert.NoError(t, enricher.TryAdd(devops.ENV_NAME_PATTERN, "(?i)prod"))

	config := &models.ArgocdScopeConfig{
		ProductionPattern: "(?i)critical",
		EnvNamePattern:    "(?i)prod",
	}

	env := detectEnvironment(
		&models.ArgocdSyncOperation{ApplicationName: "critical-app"},
		&models.ArgocdApplication{DestNamespace: "prod-east"},
		config,
		enricher,
	)
	assert.Equal(t, devops.PRODUCTION, env)
}

func TestIncludeSyncOperation(t *testing.T) {
	assert.True(t, includeSyncOperation(&models.ArgocdSyncOperation{ApplicationName: "any"}, nil, nil))

	config := &models.ArgocdScopeConfig{
		DeploymentPattern: "^prod",
	}
	assert.True(t, includeSyncOperation(&models.ArgocdSyncOperation{ApplicationName: "prod-app"}, config, nil))
	assert.False(t, includeSyncOperation(&models.ArgocdSyncOperation{ApplicationName: "dev-app"}, config, nil))

	config.DeploymentPattern = "(" // invalid regex should default to include
	assert.True(t, includeSyncOperation(&models.ArgocdSyncOperation{ApplicationName: "dev-app"}, config, nil))

	enricher := api.NewRegexEnricher()
	assert.NoError(t, enricher.TryAdd(devops.DEPLOYMENT, "(?i)release"))
	assert.True(t, includeSyncOperation(&models.ArgocdSyncOperation{ApplicationName: "release-app"}, config, enricher))
	assert.False(t, includeSyncOperation(&models.ArgocdSyncOperation{ApplicationName: "feature-app"}, config, enricher))
}

func TestConvertPhaseToResult(t *testing.T) {
	assert.Equal(t, devops.RESULT_SUCCESS, convertPhaseToResult("Succeeded"))
	assert.Equal(t, devops.RESULT_FAILURE, convertPhaseToResult("Failed"))
	assert.Equal(t, devops.RESULT_FAILURE, convertPhaseToResult("Error"))
	assert.Equal(t, devops.RESULT_FAILURE, convertPhaseToResult("Terminating"))
	assert.Equal(t, devops.RESULT_DEFAULT, convertPhaseToResult("Unknown"))
}

func TestConvertSyncOperationResult(t *testing.T) {
	cases := []struct {
		phase          string
		health         string
		result         string
		originalResult string
	}{
		{"Succeeded", "Degraded", devops.RESULT_FAILURE, "Degraded"},
		{"Succeeded", "Healthy", devops.RESULT_SUCCESS, "Succeeded"},
		{"Succeeded", "Progressing", devops.RESULT_SUCCESS, "Succeeded"},
		{"Succeeded", "", devops.RESULT_SUCCESS, "Succeeded"},
		{"Failed", "Degraded", devops.RESULT_FAILURE, "Failed"},
		{"Running", "Degraded", devops.RESULT_DEFAULT, "Running"},
	}
	for _, c := range cases {
		result, originalResult := convertSyncOperationResult(&models.ArgocdSyncOperation{Phase: c.phase, HealthStatus: c.health})
		assert.Equal(t, c.result, result, "%s/%s", c.phase, c.health)
		assert.Equal(t, c.originalResult, originalResult, "%s/%s", c.phase, c.health)
	}
}

func TestConvertPhaseToStatus(t *testing.T) {
	assert.Equal(t, devops.STATUS_DONE, convertPhaseToStatus("Succeeded"))
	assert.Equal(t, devops.STATUS_DONE, convertPhaseToStatus("Failed"))
	assert.Equal(t, devops.STATUS_IN_PROGRESS, convertPhaseToStatus("Running"))
	assert.Equal(t, devops.STATUS_IN_PROGRESS, convertPhaseToStatus("Terminating"))
	assert.Equal(t, devops.STATUS_OTHER, convertPhaseToStatus("Unknown"))
}
