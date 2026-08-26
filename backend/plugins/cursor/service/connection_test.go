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

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/apache/incubator-devlake/plugins/cursor/models"
	"github.com/stretchr/testify/require"
)

func TestBuildPermissionFailureMessage(t *testing.T) {
	message := buildPermissionFailureMessage([]string{
		"Cannot read team billing spend (teams/spend): forbidden.",
		"Cannot read filtered usage events (teams/filtered-usage-events): unauthorized.",
	})
	require.Contains(t, message, "missing required permissions")
	require.Contains(t, message, "teams/spend")
	require.Contains(t, message, "filtered-usage-events")
}

func TestBuildAdminApiError_Unauthorized(t *testing.T) {
	res := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{"message":"invalid key"}`)),
	}
	err := buildAdminApiError(spendEndpoint, "read team billing spend", res)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Team Admin API key")
	require.Contains(t, err.Error(), "teams/spend")
}

func TestBuildAdminApiError_Forbidden(t *testing.T) {
	res := &http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(strings.NewReader(`{"message":"forbidden"}`)),
	}
	err := buildAdminApiError(usageEventsEndpoint, "read filtered usage events", res)
	require.Error(t, err)
	require.Contains(t, err.Error(), "forbidden")
	require.Contains(t, err.Error(), "filtered-usage-events")
}

func TestKeyTierConstants(t *testing.T) {
	require.Equal(t, "personal", models.KeyTierPersonal)
	require.Equal(t, "team", models.KeyTierTeam)
	require.Equal(t, "enterprise", models.KeyTierEnterprise)
}

func TestTestConnectionResult_KeyTierField(t *testing.T) {
	result := &TestConnectionResult{
		Success: true,
		KeyTier: models.KeyTierEnterprise,
		Permissions: AdminApiPermissions{
			Members:        true,
			Spend:          true,
			UsageEvents:    true,
			Analytics:      true,
			AiCodeTracking: true,
		},
	}
	require.Equal(t, models.KeyTierEnterprise, result.KeyTier)
	require.True(t, result.Permissions.Analytics)
	require.True(t, result.Permissions.AiCodeTracking)
}

func TestTestConnectionResult_TeamKeyTier(t *testing.T) {
	result := &TestConnectionResult{
		Success: true,
		KeyTier: models.KeyTierTeam,
		Permissions: AdminApiPermissions{
			Members:        true,
			Spend:          true,
			UsageEvents:    true,
			Analytics:      false,
			AiCodeTracking: false,
		},
	}
	require.Equal(t, models.KeyTierTeam, result.KeyTier)
	require.False(t, result.Permissions.Analytics)
	require.False(t, result.Permissions.AiCodeTracking)
}

func TestAdminApiPermissions_EnterpriseFields(t *testing.T) {
	perms := AdminApiPermissions{
		Members:        true,
		Spend:          true,
		UsageEvents:    true,
		Analytics:      true,
		AiCodeTracking: false,
	}
	require.True(t, perms.Analytics)
	require.False(t, perms.AiCodeTracking)
}

func TestApplyTestResultToConnection(t *testing.T) {
	conn := &models.CursorConnection{}
	ApplyTestResultToConnection(conn, &TestConnectionResult{KeyTier: models.KeyTierEnterprise})
	require.Equal(t, models.KeyTierEnterprise, conn.KeyTier)

	ApplyTestResultToConnection(conn, nil)
	require.Equal(t, models.KeyTierEnterprise, conn.KeyTier)

	ApplyTestResultToConnection(conn, &TestConnectionResult{})
	require.Equal(t, models.KeyTierEnterprise, conn.KeyTier)
}

func TestFailedConnectionResult_OmitsKeyTierWhenUnauthorized(t *testing.T) {
	permissions := AdminApiPermissions{}
	result := &TestConnectionResult{
		Success:     false,
		Message:     "Cannot list team members (teams/members): unauthorized.",
		Permissions: permissions,
	}
	if permissions.Members || permissions.Spend || permissions.UsageEvents {
		result.KeyTier = models.KeyTierTeam
	}
	require.Empty(t, result.KeyTier)
}

func TestFailedConnectionResult_SetsKeyTierOnPartialAccess(t *testing.T) {
	permissions := AdminApiPermissions{Members: true}
	result := &TestConnectionResult{
		Success:     false,
		Message:     "missing spend permission",
		Permissions: permissions,
	}
	if permissions.Members || permissions.Spend || permissions.UsageEvents {
		result.KeyTier = models.KeyTierTeam
	}
	require.Equal(t, models.KeyTierTeam, result.KeyTier)
}
