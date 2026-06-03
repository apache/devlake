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

	corectx "github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/claude_code/models"
)

// TestConnectionResult represents the payload returned by the connection test endpoints.
type TestConnectionResult struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	OrganizationId string `json:"organizationId,omitempty"`
}

// TestConnection exercises the Claude Code Analytics API to validate credentials.
// It makes a minimal request with limit=1 to confirm the connection is valid.
func TestConnection(ctx stdctx.Context, br corectx.BasicRes, connection *models.ClaudeCodeConnection) (*TestConnectionResult, errors.Error) {
	if connection == nil {
		return nil, errors.BadInput.New("connection is required")
	}

	connection.Normalize()

	hasToken := strings.TrimSpace(connection.Token) != ""
	if connection.HasIncompleteCustomHeaders() {
		return nil, errors.BadInput.New("custom headers must include both key and value")
	}
	hasCustomHeaders := connection.HasUsableCustomHeaders()
	if !hasToken && !hasCustomHeaders {
		return nil, errors.BadInput.New("either token or at least one custom header is required")
	}
	if strings.TrimSpace(connection.Organization) == "" {
		return nil, errors.BadInput.New("organization is required")
	}

	apiClient, err := helper.NewApiClientFromConnection(ctx, br, connection)
	if err != nil {
		return nil, err
	}

	// Use a date safely outside the analytics availability lag window for the test request.
	testDate := time.Now().UTC().AddDate(0, 0, -7).Format("2006-01-02")
	endpoint := fmt.Sprintf("v1/organizations/usage_report/claude_code?starting_at=%s&limit=1", testDate)

	res, err := apiClient.Get(endpoint, nil, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to reach Claude Code Analytics API")
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return &TestConnectionResult{
			Success: false,
			Message: fmt.Sprintf("authentication failed (HTTP %d): verify your API credentials", res.StatusCode),
		}, nil
	}

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return &TestConnectionResult{
			Success: false,
			Message: fmt.Sprintf("unexpected status %d: %s", res.StatusCode, string(body)),
		}, nil
	}

	// Parse the response to confirm it's a valid analytics payload.
	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return nil, errors.Default.Wrap(readErr, "failed to read response body")
	}

	var response struct {
		Data    json.RawMessage `json:"data"`
		HasMore bool            `json:"has_more"`
	}
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return &TestConnectionResult{
			Success: false,
			Message: fmt.Sprintf("failed to parse response: %v", jsonErr),
		}, nil
	}

	return &TestConnectionResult{
		Success:        true,
		Message:        "Connection validated successfully",
		OrganizationId: connection.Organization,
	}, nil
}
