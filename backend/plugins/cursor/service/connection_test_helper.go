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
	stdctx "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	corectx "github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

const (
	membersEndpoint     = "teams/members"
	spendEndpoint       = "teams/spend"
	usageEventsEndpoint = "teams/filtered-usage-events"
)

// AdminApiPermissions reports which Cursor API endpoints the key can access.
type AdminApiPermissions struct {
	Members        bool `json:"members"`
	Spend          bool `json:"spend"`
	UsageEvents    bool `json:"usageEvents"`
	Analytics      bool `json:"analytics"`
	AiCodeTracking bool `json:"aiCodeTracking"`
	BugbotReviews  bool `json:"bugbotReviews"`
}

// TestConnectionResult represents the payload returned by the connection test endpoints.
type TestConnectionResult struct {
	Success     bool                `json:"success"`
	Message     string              `json:"message"`
	MemberCount int                 `json:"memberCount,omitempty"`
	Permissions AdminApiPermissions `json:"permissions,omitempty"`
	KeyTier     string              `json:"keyTier,omitempty"`
}

// TestConnection exercises the Cursor Admin API to validate credentials and permissions.
func TestConnection(ctx stdctx.Context, br corectx.BasicRes, connection *models.CursorConnection) (*TestConnectionResult, errors.Error) {
	if connection == nil {
		return nil, errors.BadInput.New("connection is required")
	}

	connection.Normalize()
	if strings.TrimSpace(connection.Token) == "" {
		return nil, errors.BadInput.New("token is required")
	}

	apiClient, err := helper.NewApiClientFromConnection(ctx, br, connection)
	if err != nil {
		return nil, err
	}
	apiClient.SetHeaders(map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})

	if userKeyErr := detectUserApiKey(apiClient); userKeyErr != nil {
		return &TestConnectionResult{
			Success: false,
			Message: userKeyErr.Error(),
			KeyTier: models.KeyTierPersonal,
		}, nil
	}

	permissions := AdminApiPermissions{}
	var failures []string

	memberCount, membersErr := probeMembers(apiClient)
	permissions.Members = membersErr == nil
	if membersErr != nil {
		failures = append(failures, membersErr.Error())
	}

	if spendErr := probeSpend(apiClient); spendErr != nil {
		permissions.Spend = false
		failures = append(failures, spendErr.Error())
	} else {
		permissions.Spend = true
	}

	if usageErr := probeUsageEvents(apiClient); usageErr != nil {
		permissions.UsageEvents = false
		failures = append(failures, usageErr.Error())
	} else {
		permissions.UsageEvents = true
	}

	if len(failures) > 0 {
		result := &TestConnectionResult{
			Success:     false,
			Message:     buildPermissionFailureMessage(failures),
			MemberCount: memberCount,
			Permissions: permissions,
		}
		if permissions.Members || permissions.Spend || permissions.UsageEvents {
			result.KeyTier = models.KeyTierTeam
		}
		return result, nil
	}

	permissions.Analytics = probeEndpoint(apiClient, "analytics/team/dau?startDate=7d&endDate=today")
	permissions.AiCodeTracking = probeEndpoint(apiClient, "analytics/ai-code/commits?page=1&pageSize=1")
	permissions.BugbotReviews = probeEndpoint(apiClient, "analytics/team/bugbot-reviews?page=1&pageSize=1")

	keyTier := models.KeyTierTeam
	if permissions.Analytics || permissions.AiCodeTracking {
		keyTier = models.KeyTierEnterprise
	}

	msg := "Team Admin API key validated. Members, spend, and usage events are accessible."
	if keyTier == models.KeyTierEnterprise {
		msg = "Enterprise Admin API key validated. Team data, analytics, and AI code tracking are accessible."
	}
	if permissions.BugbotReviews {
		msg += " BugBot review analytics are accessible."
	}

	return &TestConnectionResult{
		Success:     true,
		Message:     msg,
		MemberCount: memberCount,
		Permissions: permissions,
		KeyTier:     keyTier,
	}, nil
}

// ApplyTestResultToConnection copies detected capabilities from a test result onto the connection.
func ApplyTestResultToConnection(connection *models.CursorConnection, result *TestConnectionResult) {
	if connection == nil || result == nil {
		return
	}
	if result.KeyTier != "" {
		connection.KeyTier = result.KeyTier
	}
	connection.HasBugbotReviews = result.Permissions.BugbotReviews
}

// PopulateKeyTier probes the Cursor API and sets connection.KeyTier from the result.
func PopulateKeyTier(ctx stdctx.Context, br corectx.BasicRes, connection *models.CursorConnection) errors.Error {
	result, err := TestConnection(ctx, br, connection)
	if err != nil {
		return err
	}
	ApplyTestResultToConnection(connection, result)
	return nil
}

func probeMembers(apiClient *helper.ApiClient) (int, errors.Error) {
	res, err := apiClient.Get(membersEndpoint, nil, nil)
	if err != nil {
		return 0, errors.Default.Wrap(err, "failed to reach Cursor Admin API")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return 0, buildAdminApiError(membersEndpoint, "list team members", res)
	}

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return 0, errors.Default.Wrap(readErr, "failed to read members response")
	}

	var response struct {
		TeamMembers []json.RawMessage `json:"teamMembers"`
	}
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return 0, errors.Default.Wrap(errors.Convert(jsonErr), "failed to parse members response")
	}

	return len(response.TeamMembers), nil
}

func probeSpend(apiClient *helper.ApiClient) errors.Error {
	res, err := apiClient.Post(spendEndpoint, nil, map[string]interface{}{
		"page":     1,
		"pageSize": 1,
	}, nil)
	if err != nil {
		return errors.Default.Wrap(err, "failed to reach Cursor spend API")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return buildAdminApiError(spendEndpoint, "read team billing spend", res)
	}
	return nil
}

func probeUsageEvents(apiClient *helper.ApiClient) errors.Error {
	endMs := time.Now().UTC().UnixMilli()
	startMs := endMs - int64(7*24*time.Hour/time.Millisecond)

	res, err := apiClient.Post(usageEventsEndpoint, nil, map[string]interface{}{
		"startDate": startMs,
		"endDate":   endMs,
		"page":      1,
		"pageSize":  1,
	}, nil)
	if err != nil {
		return errors.Default.Wrap(err, "failed to reach Cursor usage events API")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return buildAdminApiError(usageEventsEndpoint, "read filtered usage events", res)
	}
	return nil
}

func buildAdminApiError(endpoint, action string, res *http.Response) errors.Error {
	body, _ := io.ReadAll(res.Body)
	detail := strings.TrimSpace(string(body))
	if detail != "" && len(detail) > 200 {
		detail = detail[:200] + "..."
	}

	switch res.StatusCode {
	case http.StatusUnauthorized:
		return errors.BadInput.New(fmt.Sprintf(
			"Cannot %s (%s): unauthorized. Use a Team Admin API key from Dashboard → API Keys (admin:* scope), not a User API key from Settings → Integrations.",
			action, endpoint,
		))
	case http.StatusForbidden:
		msg := fmt.Sprintf("Cannot %s (%s): forbidden. The key may lack admin permissions for this team.", action, endpoint)
		if detail != "" {
			msg = fmt.Sprintf("%s Details: %s", msg, detail)
		}
		return errors.BadInput.New(msg)
	default:
		msg := fmt.Sprintf("Cannot %s (%s): unexpected status %d", action, endpoint, res.StatusCode)
		if detail != "" {
			msg = fmt.Sprintf("%s. Details: %s", msg, detail)
		}
		return errors.BadInput.New(msg)
	}
}

func buildPermissionFailureMessage(failures []string) string {
	if len(failures) == 1 {
		return failures[0]
	}
	return "Team Admin API key is missing required permissions:\n- " + strings.Join(failures, "\n- ")
}

func detectUserApiKey(apiClient *helper.ApiClient) errors.Error {
	res, err := apiClient.Get("v1/me", nil, nil)
	if err != nil || res == nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return errors.BadInput.New("this is a User API key. Create a Team Admin key in Dashboard → API Keys (admin:* scope)")
	}
	return nil
}

func probeEndpoint(apiClient *helper.ApiClient, path string) bool {
	res, err := apiClient.Get(path, nil, nil)
	if err != nil || res == nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}
