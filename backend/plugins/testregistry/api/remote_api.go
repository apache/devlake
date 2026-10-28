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
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/plugins/testregistry/models"
)

type QuayRepository struct {
	Name        string  `json:"name"`
	Namespace   string  `json:"namespace"`
	Description *string `json:"description"` // Can be null
	IsPublic    bool    `json:"is_public"`
	Kind        string  `json:"kind"`  // e.g., "image"
	State       string  `json:"state"` // e.g., "NORMAL"
	// quota_report is ignored as we don't need it
}

type QuayRepositoryResponse struct {
	Repositories []QuayRepository `json:"repositories"`
	NextPage     *string          `json:"next_page"`
}

type TestRegistryRemotePagination struct {
	PageToken string `json:"pageToken"`
}

// GitHubContentItem represents a file or directory in GitHub repository
type GitHubContentItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
	SHA         string `json:"sha"`
}

func listTestRegistryRemoteScopes(
	connection *models.TestRegistryConnection,
	apiClient plugin.ApiClient,
	_ string,
	pageToken string,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.TestRegistryScope],
	nextPageToken string,
	err errors.Error,
) {
	// Check CI tool type to determine which API to use
	if connection.CITool == models.CIToolOpenshiftCI {
		return listOpenshiftCIScopes(connection, apiClient, pageToken)
	} else if connection.CITool == models.CIToolTektonCI {
		return listTektonCIScopes(connection, apiClient, pageToken)
	}

	return nil, "", errors.BadInput.New("ciTool must be either 'Openshift CI' or 'Tekton CI'")
}

func listOpenshiftCIScopes(
	connection *models.TestRegistryConnection,
	apiClient plugin.ApiClient,
	_ string,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.TestRegistryScope],
	nextPageToken string,
	err errors.Error,
) {
	if connection.GitHubOrganization == "" {
		return nil, "", errors.BadInput.New("githubOrganization is required for Openshift CI")
	}

	// Fetch contents from GitHub: openshift/release repository, ci-operator/config/{org} path
	// Fixed repository: openshift/release
	// Path: ci-operator/config/{githubOrganization}
	repoPath := fmt.Sprintf("repos/openshift/release/contents/ci-operator/config/%s", connection.GitHubOrganization)
	// Use master branch (or main)
	branch := "master"
	apiURL := fmt.Sprintf("%s?ref=%s", repoPath, branch)

	resp, err := apiClient.Get(apiURL, nil, nil)
	if err != nil {
		return nil, "", errors.Default.Wrap(err, fmt.Sprintf("failed to fetch contents from GitHub repo openshift/release/ci-operator/config/%s", connection.GitHubOrganization))
	}

	// Check if response is successful
	if resp.StatusCode == http.StatusNotFound {
		// Try with 'main' branch if 'master' doesn't exist
		apiURL = fmt.Sprintf("%s?ref=%s", repoPath, "main")
		resp, err = apiClient.Get(apiURL, nil, nil)
		if err != nil {
			return nil, "", errors.Default.Wrap(err, fmt.Sprintf("failed to fetch contents from GitHub repo openshift/release/ci-operator/config/%s", connection.GitHubOrganization))
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.Default.New(fmt.Sprintf("GitHub API returned status %d for repo openshift/release/ci-operator/config/%s", resp.StatusCode, connection.GitHubOrganization))
	}

	var contents []GitHubContentItem
	if err = api.UnmarshalResponse(resp, &contents); err != nil {
		return nil, "", errors.Default.Wrap(err, "failed to parse GitHub API response")
	}

	// Log for debugging
	if basicRes != nil {
		logger := basicRes.GetLogger()
		logger.Info("GitHub API response: %d items found in ci-operator/config/%s for org %s", len(contents), connection.GitHubOrganization, connection.GitHubOrganization)
	}

	// Convert GitHub contents to scope entries
	// Each file/directory under ci-operator/config/{org} becomes a scope
	for _, item := range contents {
		// Include all directories (which typically contain CI configs)
		// Also include YAML files directly in the org directory
		isYAMLFile := item.Type == "file" && (len(item.Name) >= 5 && (item.Name[len(item.Name)-5:] == ".yaml" || item.Name[len(item.Name)-5:] == ".yml"))
		if item.Type == "dir" || isYAMLFile {
			// Name and FullName should only contain the repo name inside the org folder
			// e.g., if item.Name is "my-repo", then both Name and FullName are "my-repo"
			scopeName := item.Name
			scopeFullName := item.Name // Same as name, just the repo name without the path

			// Create scope data
			scopeData := &models.TestRegistryScope{
				Name:     scopeName,
				FullName: scopeFullName,
			}
			children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.TestRegistryScope]{
				Type:     api.RAS_ENTRY_TYPE_SCOPE,
				ParentId: nil,
				Id:       scopeData.ScopeId(),
				Name:     scopeData.ScopeName(),
				FullName: scopeData.ScopeFullName(),
				Data:     scopeData,
			})
		}
	}

	return children, "", nil // GitHub Contents API doesn't use pagination tokens in the same way
}

func listTektonCIScopes(
	connection *models.TestRegistryConnection,
	apiClient plugin.ApiClient,
	pageToken string,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.TestRegistryScope],
	nextPageToken string,
	err errors.Error,
) {
	if connection.QuayOrganization == "" {
		return nil, "", errors.BadInput.New("quayOrganization is required for Tekton CI")
	}

	// Quay.io API endpoint: /api/v1/repository?namespace=<org>
	// Note: apiClient endpoint is already set to "https://quay.io", so use relative path
	apiURL := "/api/v1/repository"
	queryParams := url.Values{}
	queryParams.Set("namespace", connection.QuayOrganization)
	queryParams.Set("public", "true") // Include public repositories

	if pageToken != "" {
		// Parse pageToken to extract page number if needed
		// For Quay.io, we can use next_page from the response
		queryParams.Set("next_page", pageToken)
	}

	resp, err := apiClient.Get(apiURL, queryParams, nil)
	if err != nil {
		return nil, "", errors.Default.Wrap(err, "failed to fetch repositories from Quay.io")
	}

	// Check if response is successful
	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.Default.New(fmt.Sprintf("Quay.io API returned status %d", resp.StatusCode))
	}

	var quayResponse QuayRepositoryResponse
	if err = api.UnmarshalResponse(resp, &quayResponse); err != nil {
		return nil, "", errors.Default.Wrap(err, "failed to parse Quay.io API response")
	}

	// Log for debugging
	if basicRes != nil {
		logger := basicRes.GetLogger()
		logger.Info("Quay.io API response: %d repositories found for organization %s", len(quayResponse.Repositories), connection.QuayOrganization)
	}

	// Convert Quay repositories to scope entries
	for _, repo := range quayResponse.Repositories {
		repoName := repo.Name
		repoFullName := fmt.Sprintf("%s/%s", repo.Namespace, repo.Name)
		// Create scope data - the embedded common.Scope fields will be set when saved
		scopeData := &models.TestRegistryScope{
			Name:     repoName,
			FullName: repoFullName,
			// common.Scope fields (ConnectionId, etc.) will be set when the scope is saved to DB
		}
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.TestRegistryScope]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			ParentId: nil,                       // No parent grouping
			Id:       scopeData.ScopeId(),       // Use scope method to get ID
			Name:     scopeData.ScopeName(),     // Use scope method to get name
			FullName: scopeData.ScopeFullName(), // Use scope method to get full name
			Data:     scopeData,
		})
	}

	// Set next page token if available
	if quayResponse.NextPage != nil && *quayResponse.NextPage != "" {
		nextPageToken = *quayResponse.NextPage
	}

	return children, nextPageToken, nil
}

// RemoteScopes fetches scopes based on CI tool type
// @Summary get testregistry remote scopes
// @Description Get scopes from Quay.io (Tekton CI) or GitHub (Openshift CI)
// @Tags plugins/testregistry
// @Param connectionId path int true "connection ID"
// @Param pageToken query string false "page token for pagination"
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.TestRegistryScope]
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/testregistry/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.ModelApiHelper.FindByPk(input)
	if err != nil {
		return nil, err
	}

	pageToken := input.Query.Get("pageToken")

	var apiClient plugin.ApiClient

	// Create API client based on CI tool type
	if connection.CITool == models.CIToolOpenshiftCI {
		// GitHub API client with authentication
		apiClient, err = api.NewApiClient(gocontext.TODO(), "https://api.github.com", nil, 0, "", basicRes)
		if err != nil {
			return nil, errors.Default.Wrap(err, "failed to create GitHub API client")
		}

		// Set authentication header with GitHub token
		if connection.GitHubToken != "" {
			apiClient.SetHeaders(map[string]string{
				"Authorization": fmt.Sprintf("Bearer %s", connection.GitHubToken),
			})
		}
	} else if connection.CITool == models.CIToolTektonCI {
		// Quay.io API client (no authentication needed for public repos)
		apiClient, err = api.NewApiClient(gocontext.TODO(), "https://quay.io", nil, 0, "", basicRes)
		if err != nil {
			return nil, errors.Default.Wrap(err, "failed to create Quay.io API client")
		}
	} else {
		return nil, errors.BadInput.New("ciTool must be either 'Openshift CI' or 'Tekton CI'")
	}

	children, nextPageToken, err := listTestRegistryRemoteScopes(connection, apiClient, "", pageToken)
	if err != nil {
		return nil, err
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"children":      children,
			"nextPageToken": nextPageToken,
		},
		Status: http.StatusOK,
	}, nil
}
