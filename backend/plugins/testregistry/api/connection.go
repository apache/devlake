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
	gocontext "context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/testregistry/models"
)

// PostConnections
// @Summary create testregistry connection
// @Description Create testregistry connection
// @Tags plugins/testregistry
// @Param body body models.TestRegistryConnection true "json body"
// @Success 200  {object} models.TestRegistryConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/testregistry/connections [POST]
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Post(input)
}

// PatchConnection
// @Summary patch testregistry connection
// @Description Patch testregistry connection
// @Tags plugins/testregistry
// @Param body body models.TestRegistryConnection true "json body"
// @Success 200  {object} models.TestRegistryConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/testregistry/connections/{connectionId} [PATCH]
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Patch(input)
}

// DeleteConnection
// @Summary delete a testregistry connection
// @Description Delete a testregistry connection
// @Tags plugins/testregistry
// @Success 200  {object} models.TestRegistryConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/testregistry/connections/{connectionId} [DELETE]
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

// ListConnections
// @Summary get all testregistry connections
// @Description Get all testregistry connections
// @Tags plugins/testregistry
// @Success 200  {object} []models.TestRegistryConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/testregistry/connections [GET]
func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

// GetConnection
// @Summary get testregistry connection detail
// @Description Get testregistry connection detail
// @Tags plugins/testregistry
// @Success 200  {object} models.TestRegistryConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/testregistry/connections/{connectionId} [GET]
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}

// TestConnection test testregistry connection
// @Summary test testregistry connection
// @Description Test testregistry connection by pinging Quay.io API (Tekton CI) or GitHub API (Openshift CI)
// @Tags plugins/testregistry
// @Param body body models.TestRegistryConnection true "json body"
// @Success 200  {object} shared.ApiBody
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/testregistry/test [POST]
func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// input.Body is already decoded as map[string]interface{} by the framework
	// However, frontend pick() might exclude custom fields, so we check both input.Body and try struct decode
	bodyMap := input.Body
	if bodyMap == nil {
		bodyMap = make(map[string]interface{})
	}

	// Also try struct decode to see if we can get fields that way
	var conn models.TestRegistryConnection
	if err := api.Decode(input.Body, &conn, nil); err == nil {
		// Merge struct values into bodyMap (only if not already present)
		if ciToolVal, ok := bodyMap["ciTool"].(string); !ok || ciToolVal == "" {
			if conn.CITool != "" {
				bodyMap["ciTool"] = conn.CITool
			}
		}
		if quayOrgVal, ok := bodyMap["quayOrganization"].(string); !ok || quayOrgVal == "" {
			if conn.QuayOrganization != "" {
				bodyMap["quayOrganization"] = conn.QuayOrganization
			}
		}
		if githubOrgVal, ok := bodyMap["githubOrganization"].(string); !ok || githubOrgVal == "" {
			if conn.GitHubOrganization != "" {
				bodyMap["githubOrganization"] = conn.GitHubOrganization
			}
		}
		if githubTokenVal, ok := bodyMap["githubToken"].(string); !ok || githubTokenVal == "" {
			if conn.GitHubToken != "" {
				bodyMap["githubToken"] = conn.GitHubToken
			}
		}
	}

	// Log what we received for debugging
	if basicRes != nil {
		logger := basicRes.GetLogger()
		logger.Info("TestConnection received body fields: %v", bodyMap)
	}

	// Extract fields with type assertions
	ciTool, _ := bodyMap["ciTool"].(string)
	quayOrg, _ := bodyMap["quayOrganization"].(string)
	githubOrg, _ := bodyMap["githubOrganization"].(string)
	githubToken, _ := bodyMap["githubToken"].(string)

	// Provide helpful error if ciTool is missing
	if ciTool == "" {
		// Check if any fields are present to provide better error message
		hasAnyField := len(bodyMap) > 0
		if hasAnyField {
			return nil, errors.BadInput.New("ciTool field is missing from request. The frontend may not be sending custom fields. Please try saving the connection first, then test it using the 'Test' button from the connection list.")
		}
		return nil, errors.BadInput.New("ciTool field is required. Please ensure all form fields are filled correctly.")
	}

	var testErr errors.Error
	var successMsg string

	// Test based on CI tool type
	switch ciTool {
	case models.CIToolTektonCI:
		if quayOrg == "" {
			return nil, errors.BadInput.New("quayOrganization is required for Tekton CI")
		}
		testErr = testQuayConnection(gocontext.TODO(), quayOrg)
		if testErr == nil {
			successMsg = fmt.Sprintf("Successfully connected to Quay.io organization: %s", quayOrg)
		}
	case models.CIToolOpenshiftCI:
		if githubOrg == "" {
			return nil, errors.BadInput.New("githubOrganization is required for Openshift CI")
		}
		if githubToken == "" {
			return nil, errors.BadInput.New("githubToken is required for Openshift CI")
		}
		testErr = testGitHubConnection(gocontext.TODO(), githubOrg, githubToken)
		if testErr == nil {
			successMsg = fmt.Sprintf("Successfully connected to GitHub organization: %s", githubOrg)
		}
	default:
		return nil, errors.BadInput.New(fmt.Sprintf("invalid ciTool: %s. Must be 'Openshift CI' or 'Tekton CI'", ciTool))
	}

	if testErr != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, testErr)
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"success": true,
			"message": successMsg,
		},
		Status: http.StatusOK,
	}, nil
}

// TestExistingConnection test existing testregistry connection
// @Summary test existing testregistry connection
// @Description Test existing testregistry connection by pinging Quay.io API (Tekton CI) or GitHub API (Openshift CI)
// @Tags plugins/testregistry
// @Param connectionId path int true "connection ID"
// @Success 200  {object} shared.ApiBody
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/testregistry/connections/{connectionId}/test [POST]
func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.Convert(err)
	}

	var testErr errors.Error
	var successMsg string

	// Test based on CI tool type
	switch connection.CITool {
	case models.CIToolTektonCI:
		if connection.QuayOrganization == "" {
			return nil, errors.BadInput.New("quayOrganization is required for Tekton CI")
		}
		testErr = testQuayConnection(gocontext.TODO(), connection.QuayOrganization)
		if testErr == nil {
			successMsg = fmt.Sprintf("Successfully connected to Quay.io organization: %s", connection.QuayOrganization)
		}
	case models.CIToolOpenshiftCI:
		if connection.GitHubOrganization == "" {
			return nil, errors.BadInput.New("githubOrganization is required for Openshift CI")
		}
		if connection.GitHubToken == "" {
			return nil, errors.BadInput.New("githubToken is required for Openshift CI")
		}
		testErr = testGitHubConnection(gocontext.TODO(), connection.GitHubOrganization, connection.GitHubToken)
		if testErr == nil {
			successMsg = fmt.Sprintf("Successfully connected to GitHub organization: %s", connection.GitHubOrganization)
		}
	default:
		return nil, errors.BadInput.New(fmt.Sprintf("invalid ciTool: %s. Must be 'Openshift CI' or 'Tekton CI'", connection.CITool))
	}

	if testErr != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, testErr)
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"success": true,
			"message": successMsg,
		},
		Status: http.StatusOK,
	}, nil
}

// testQuayConnection pings Quay.io API to verify the organization is accessible
func testQuayConnection(ctx gocontext.Context, quayOrganization string) errors.Error {
	// Create API client for Quay.io
	apiClient, err := api.NewApiClient(ctx, "https://quay.io", nil, 0, "", basicRes)
	if err != nil {
		return errors.Default.Wrap(err, "failed to create API client")
	}

	// Ping Quay.io by trying to list repositories for the organization
	apiURL := "/api/v1/repository"
	queryParams := url.Values{}
	queryParams.Set("namespace", quayOrganization)
	queryParams.Set("public", "true")
	queryParams.Set("limit", "1") // Only need to check if the request succeeds

	resp, err := apiClient.Get(apiURL, queryParams, nil)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("failed to connect to Quay.io organization '%s'", quayOrganization))
	}

	// Check response status
	if resp.StatusCode == http.StatusNotFound {
		return errors.BadInput.New(fmt.Sprintf("Quay.io organization '%s' not found or not accessible", quayOrganization))
	}

	if resp.StatusCode != http.StatusOK {
		return errors.Default.New(fmt.Sprintf("Quay.io API returned status %d for organization '%s'", resp.StatusCode, quayOrganization))
	}

	return nil
}

// testGitHubConnection pings GitHub API to verify the organization and token are valid
func testGitHubConnection(ctx gocontext.Context, githubOrganization, githubToken string) errors.Error {
	// Create API client for GitHub
	apiClient, err := api.NewApiClient(ctx, "https://api.github.com", nil, 0, "", basicRes)
	if err != nil {
		return errors.Default.Wrap(err, "failed to create API client")
	}

	// Set authentication header
	apiClient.SetHeaders(map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", githubToken),
	})

	// Test token validity by getting authenticated user
	userResp, err := apiClient.Get("user", nil, nil)
	if err != nil {
		return errors.Default.Wrap(err, "failed to authenticate with GitHub token")
	}

	if userResp.StatusCode == http.StatusUnauthorized {
		return errors.BadInput.New("GitHub token is invalid or expired")
	}

	if userResp.StatusCode != http.StatusOK {
		return errors.Default.New(fmt.Sprintf("GitHub API returned status %d while testing token", userResp.StatusCode))
	}

	// Verify organization access by trying to get organization info
	orgURL := fmt.Sprintf("orgs/%s", githubOrganization)
	orgResp, err := apiClient.Get(orgURL, nil, nil)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("failed to access GitHub organization '%s'", githubOrganization))
	}

	if orgResp.StatusCode == http.StatusNotFound {
		return errors.BadInput.New(fmt.Sprintf("GitHub organization '%s' not found", githubOrganization))
	}

	if orgResp.StatusCode == http.StatusForbidden {
		return errors.BadInput.New(fmt.Sprintf("GitHub token does not have access to organization '%s'", githubOrganization))
	}

	if orgResp.StatusCode != http.StatusOK {
		return errors.Default.New(fmt.Sprintf("GitHub API returned status %d for organization '%s'", orgResp.StatusCode, githubOrganization))
	}

	return nil
}
