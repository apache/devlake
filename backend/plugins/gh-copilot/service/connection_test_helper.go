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
	"strconv"
	"strings"
	"time"

	corectx "github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/gh-copilot/models"
)

// TestConnectionResult represents the payload returned by the connection test endpoints.
type TestConnectionResult struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	Enterprise   string `json:"enterprise,omitempty"`
	Organization string `json:"organization,omitempty"`
	PlanType     string `json:"planType,omitempty"`
	TotalSeats   int    `json:"totalSeats,omitempty"`
	ActiveSeats  int    `json:"activeSeats,omitempty"`
}

type copilotBillingSummary struct {
	Organization    string `json:"organization"`
	PlanType        string `json:"plan_type"`
	TotalSeats      int    `json:"total_seats"`
	ActiveSeats     int    `json:"active_seats"`
	ActiveThisCycle int    `json:"active_this_cycle"`
}

// TestConnection exercises the GitHub Copilot billing endpoint to validate credentials.
func TestConnection(ctx stdctx.Context, br corectx.BasicRes, connection *models.GhCopilotConnection) (*TestConnectionResult, errors.Error) {
	if connection == nil {
		return nil, errors.BadInput.New("connection is required")
	}

	connection.Normalize()

	hasEnterprise := connection.HasEnterprise()
	hasOrg := strings.TrimSpace(connection.Organization) != ""

	if !hasEnterprise && !hasOrg {
		return nil, errors.BadInput.New("either enterprise or organization must be specified")
	}

	apiClient, err := helper.NewApiClientFromConnection(ctx, br, connection)
	if err != nil {
		return nil, err
	}
	apiClient.SetHeaders(map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2026-03-10",
	})

	result := &TestConnectionResult{
		Success: true,
		Message: "Successfully connected to GitHub Copilot",
	}

	// Test enterprise endpoint first when configured.
	// Note: /enterprises/{ent}/copilot/billing does not exist — use /billing/seats instead.
	if hasEnterprise {
		entSlug := strings.TrimSpace(connection.Enterprise)
		seatsPath := fmt.Sprintf("enterprises/%s/copilot/billing/seats", entSlug)
		entSummary, entErr := fetchSeatsSummary(apiClient, seatsPath)
		if entErr != nil {
			return nil, entErr
		}
		result.Enterprise = entSlug
		result.PlanType = "enterprise"
		result.TotalSeats = entSummary.TotalSeats
	}

	// Test org endpoint when configured.
	if hasOrg {
		orgSummary, orgErr := fetchBillingSummary(apiClient, fmt.Sprintf("orgs/%s/copilot/billing", connection.Organization))
		if orgErr != nil {
			return nil, orgErr
		}
		organization := orgSummary.Organization
		if organization == "" {
			organization = connection.Organization
		}
		result.Organization = organization

		// When enterprise is not set, use org-level data for the result.
		if !hasEnterprise {
			result.PlanType = orgSummary.PlanType
			result.TotalSeats = orgSummary.TotalSeats
			activeSeats := orgSummary.ActiveSeats
			if activeSeats == 0 && orgSummary.ActiveThisCycle > 0 {
				activeSeats = orgSummary.ActiveThisCycle
			}
			result.ActiveSeats = activeSeats
		}
	}

	return result, nil
}

// fetchBillingSummary calls a billing endpoint and returns the parsed summary or error.
func fetchBillingSummary(apiClient *helper.ApiClient, path string) (*copilotBillingSummary, errors.Error) {
	res, err := apiClient.Get(path, nil, nil)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		body, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, errors.Convert(readErr)
		}
		return nil, buildGitHubApiError(res.StatusCode, path, body, res.Header.Get("Retry-After"))
	}

	summary := &copilotBillingSummary{}
	if err := helper.UnmarshalResponse(res, summary); err != nil {
		return nil, err
	}
	return summary, nil
}

// enterpriseSeatsSummary represents the top-level response from /copilot/billing/seats.
type enterpriseSeatsSummary struct {
	TotalSeats int `json:"total_seats"`
}

// fetchSeatsSummary calls the seats endpoint and returns the total seat count.
func fetchSeatsSummary(apiClient *helper.ApiClient, path string) (*enterpriseSeatsSummary, errors.Error) {
	res, err := apiClient.Get(path, nil, nil)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		body, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, errors.Convert(readErr)
		}
		return nil, buildGitHubApiError(res.StatusCode, path, body, res.Header.Get("Retry-After"))
	}

	summary := &enterpriseSeatsSummary{}
	if err := helper.UnmarshalResponse(res, summary); err != nil {
		return nil, err
	}
	return summary, nil
}

func buildGitHubApiError(status int, resource string, body []byte, retryAfter string) errors.Error {
	type githubError struct {
		Message string `json:"message"`
	}

	msg := strings.TrimSpace(string(body))
	if len(body) > 0 {
		errPayload := &githubError{}
		if jsonErr := json.Unmarshal(body, errPayload); jsonErr == nil && errPayload.Message != "" {
			msg = errPayload.Message
		}
	}

	var prefix string
	switch status {
	case http.StatusForbidden:
		prefix = "GitHub returned 403 Forbidden. Ensure the PAT includes manage_billing:copilot and the resource has Copilot access."
	case http.StatusNotFound:
		prefix = fmt.Sprintf("GitHub returned 404 Not Found for '%s'. Verify the organization/enterprise slug and Copilot availability.", resource)
	case http.StatusUnprocessableEntity:
		prefix = "GitHub returned 422 Unprocessable Entity. Enable Copilot metrics before testing."
	case http.StatusTooManyRequests:
		prefix = "GitHub rate limited the request (429). Respect Retry-After guidance before retrying."
	default:
		prefix = fmt.Sprintf("GitHub API request failed with status %d.", status)
	}

	if retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			prefix = fmt.Sprintf("%s Retry after %d seconds.", prefix, seconds)
		} else if delay, err := http.ParseTime(retryAfter); err == nil {
			seconds := int(time.Until(delay).Seconds())
			if seconds > 0 {
				prefix = fmt.Sprintf("%s Retry after %d seconds.", prefix, seconds)
			}
		} else {
			prefix = fmt.Sprintf("%s Retry-After: %s.", prefix, retryAfter)
		}
	}

	if msg != "" {
		prefix = fmt.Sprintf("%s Details: %s", prefix, msg)
	}

	return errors.HttpStatus(status).New(strings.TrimSpace(prefix))
}
