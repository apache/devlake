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

package azuredevops

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
)

const apiVersion = "7.1"
const maxPageSize = 100

type Client struct {
	c http.Client

	apiClient  plugin.ApiClient
	connection *models.AzuredevopsConnection
	url        string
}

func NewClient(con *models.AzuredevopsConnection, apiClient plugin.ApiClient, url string) Client {
	return Client{
		c: http.Client{
			Timeout: 10 * time.Second,
		},
		connection: con,
		url:        url,
		apiClient:  apiClient,
	}
}

func (c *Client) GetUserProfile() (Profile, errors.Error) {
	// On-Premises Azure DevOps Server does not have the global VSSPS profile API.
	// Return a placeholder profile to avoid blocking the connection test flow.
	if c.connection != nil && c.connection.Endpoint != "" {
		return Profile{
			DisplayName: "On-Premises User",
		}, nil
	}

	var p Profile
	baseUrl := strings.TrimRight(c.url, "/")
	if baseUrl == "" {
		baseUrl = "https://app.vssps.visualstudio.com"
	}
	endpoint := fmt.Sprintf("%s/_apis/profile/profiles/me?api-version=7.1-preview.1", baseUrl)

	res, err := c.doGet(endpoint)
	if err != nil {
		return Profile{}, errors.Internal.Wrap(err, "failed to read user accounts")
	}

	if res.StatusCode == 203 || res.StatusCode == 302 || res.StatusCode == 401 {
		return Profile{}, errors.Unauthorized.New("failed to read user profile")
	}

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return Profile{}, errors.Internal.Wrap(err, "failed to read response body")
	}

	if err := json.Unmarshal(resBody, &p); err != nil {
		return Profile{}, errors.Internal.Wrap(err, "failed to unmarshal user profile")
	}
	return p, nil
}

func (c *Client) GetUserAccounts(memberId string) (AccountResponse, errors.Error) {
	// On-Premises installations do not have the global accounts API, return empty list.
	if c.connection != nil && c.connection.Endpoint != "" {
		return AccountResponse{}, nil
	}

	var a AccountResponse
	endpoint := fmt.Sprintf("%s/_apis/accounts?memberId=%s&api-version=7.1-preview.1", strings.TrimRight(c.url, "/"), memberId)
	res, err := c.doGet(endpoint)
	if err != nil {
		return nil, errors.Internal.Wrap(err, "failed to read user accounts")
	}

	if res.StatusCode == 302 || res.StatusCode == 401 {
		return nil, errors.Unauthorized.New("failed to read user accounts")
	}

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Internal.Wrap(err, "failed to read response body")
	}

	if err := json.Unmarshal(resBody, &a); err != nil {
		return nil, errors.Internal.Wrap(err, "failed to read unmarshal response body")
	}
	return a, nil
}

func (c *Client) doGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if err = c.connection.GetAccessTokenAuthenticator().SetupAuthentication(req); err != nil {
		return nil, errors.Internal.Wrap(err, "failed to authorize the request using the plugin connection")
	}
	return c.c.Do(req)
}

type GetProjectsArgs struct {
	// (optional) Pagination
	*OffsetPagination

	OrgId string
}

func (c *Client) GetProjects(args GetProjectsArgs) ([]Project, errors.Error) {
	query := url.Values{}
	query.Set("api-version", apiVersion)

	var top, skip int
	top = maxPageSize
	skip = 0
	if args.OffsetPagination != nil {
		top = args.Top
		skip = args.Skip
	}

	var data struct {
		Count    int       `json:"count"`
		Projects []Project `json:"value"`
	}

	var projects []Project

	for {
		query.Set("$top", strconv.Itoa(top))
		query.Set("$skip", strconv.Itoa(skip))

		path := "_apis/projects"
		if args.OrgId != "" {
			path = fmt.Sprintf("%s/_apis/projects", args.OrgId)
		}
		res, err := c.apiClient.Get(path, query, nil)
		if err != nil {
			return nil, err
		}

		if res.StatusCode == 203 || res.StatusCode == 401 {
			return nil, errors.Unauthorized.New("failed to read projects")
		}

		if res.StatusCode != 200 {
			return nil, errors.Internal.New(fmt.Sprintf("failed to read projects, upstream api call failed with (%v)", res.StatusCode))
		}

		err = api.UnmarshalResponse(res, &data)
		if err != nil {
			return nil, err
		}

		projects = append(projects, data.Projects...)

		if data.Count < top {
			return projects, nil
		}

		skip += top
	}
}

type GetRepositoriesArgs struct {
	OrgId     string
	ProjectId string
}

func (c *Client) GetRepositories(args GetRepositoriesArgs) ([]Repository, errors.Error) {
	query := url.Values{}
	query.Set("api-version", apiVersion)

	var data struct {
		Repos []Repository `json:"value"`
	}

	path := fmt.Sprintf("%s/%s/_apis/git/repositories", args.OrgId, args.ProjectId)
	if args.OrgId == "" {
		path = fmt.Sprintf("%s/_apis/git/repositories", args.ProjectId)
	}
	res, err := c.apiClient.Get(path, query, nil)
	if err != nil {
		return nil, err
	}

	switch res.StatusCode {
	case 401:
		fallthrough
	case 403:
		return nil, errors.Unauthorized.New("failed to authorize the '.../_apis/git/repositories' request using the plugin connection")
	case 404:
		return nil, errors.NotFound.New("failed to find requested resource on '.../_apis/git/repositories'")
	default:
	}

	err = api.UnmarshalResponse(res, &data)
	if err != nil {
		return nil, err
	}

	return data.Repos, nil
}

type GetServiceEndpointsArgs struct {
	ProjectId string
	OrgId     string
}

func (c *Client) GetServiceEndpoints(args GetServiceEndpointsArgs) ([]ServiceEndpoint, errors.Error) {
	query := url.Values{}
	query.Set("api-version", apiVersion)

	path := fmt.Sprintf("%s/%s/_apis/serviceendpoint/endpoints/", args.OrgId, args.ProjectId)
	res, err := c.apiClient.Get(path, query, nil)
	if err != nil {
		return nil, err
	}

	switch res.StatusCode {
	case 401:
		fallthrough
	case 403:
		return nil, errors.Unauthorized.New("failed to authorize the '.../serviceendpoint/endpoints' request using the plugin connection")
	case 404:
		return nil, errors.NotFound.New("failed to find requested resource on '.../serviceendpoint/endpoints'")
	default:
	}

	var data struct {
		ServiceEndpoints []ServiceEndpoint `json:"value"`
	}

	err = api.UnmarshalResponse(res, &data)
	if err != nil {
		return nil, err
	}
	return data.ServiceEndpoints, nil
}

type GetRemoteRepositoriesArgs struct {
	ProjectId string
	OrgId     string
	Provider  string
	// (optional) Service Endpoint to filter for
	ServiceEndpoint string
}

func (c *Client) GetRemoteRepositories(args GetRemoteRepositoriesArgs) ([]RemoteRepository, error) {
	query := url.Values{}
	query.Set("api-version", apiVersion)
	if args.ServiceEndpoint != "" {
		query.Set("serviceEndpointId", args.ServiceEndpoint)
	}

	var repos []RemoteRepository
	var response struct {
		Repository []RemoteRepository `json:"repositories"`
	}

	for {
		path := fmt.Sprintf("%s/%s/_apis/sourceProviders/%s/repositories/", args.OrgId, args.ProjectId, args.Provider)
		res, err := c.apiClient.Get(path, query, nil)
		if err != nil {
			return nil, errors.Internal.Wrap(err, "failed to read remote repositories")
		}
		err = api.UnmarshalResponse(res, &response)
		if err != nil {
			return nil, errors.Internal.Wrap(err, "failed to unmarshal remote repositories response")
		}

		repos = append(repos, response.Repository...)
		contToken := res.Header.Get("X-Ms-Continuationtoken")
		if contToken == "" {
			return repos, nil
		}

		query.Set("continuationToken", contToken)
	}
}

const reposFeatureID = "ms.vss-code.version-control"

func (c *Client) GitReposEnabled(orgId, projectName string) (bool, errors.Error) {
	project, err := c.getProject(orgId, projectName)
	if err != nil {
		return false, err
	}
	query := url.Values{}
	query.Set("api-version", "4.1-preview.1")
	path := fmt.Sprintf("_apis/FeatureManagement/FeatureStatesQuery/host/project/%s", url.PathEscape(project.Id))
	if orgId != "" {
		path = url.PathEscape(orgId) + "/" + path
	}
	res, err := c.apiClient.Post(path, query, map[string]interface{}{
		"featureIds":  []string{reposFeatureID},
		"scopeValues": map[string]string{"project": project.Id},
	}, nil)
	if err != nil {
		return false, err
	}
	if res.StatusCode == http.StatusNonAuthoritativeInfo || res.StatusCode == http.StatusUnauthorized || res.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return false, errors.Default.New(fmt.Sprintf("failed to read Azure Repos service state (%d)", res.StatusCode))
	}
	var body struct {
		FeatureStates map[string]struct {
			State json.RawMessage `json:"state"`
		} `json:"featureStates"`
	}
	if err = api.UnmarshalResponse(res, &body); err != nil {
		return false, err
	}
	feature, ok := body.FeatureStates[reposFeatureID]
	if !ok {
		return true, nil
	}
	return !featureStateDisabled(feature.State), nil
}

func (c *Client) getProject(orgId, projectName string) (Project, errors.Error) {
	query := url.Values{}
	query.Set("api-version", apiVersion)
	path := fmt.Sprintf("_apis/projects/%s", url.PathEscape(projectName))
	if orgId != "" {
		path = url.PathEscape(orgId) + "/" + path
	}
	res, err := c.apiClient.Get(path, query, nil)
	if err != nil {
		return Project{}, err
	}
	if res.StatusCode == http.StatusNonAuthoritativeInfo || res.StatusCode == http.StatusUnauthorized || res.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return Project{}, errors.Default.New(fmt.Sprintf("failed to read Azure DevOps project %s (%d)", projectName, res.StatusCode))
	}
	var project Project
	if err = api.UnmarshalResponse(res, &project); err != nil {
		return Project{}, err
	}
	if project.Id == "" {
		return Project{}, errors.Default.New(fmt.Sprintf("Azure DevOps project %s has no id", projectName))
	}
	return project, nil
}

func featureStateDisabled(raw json.RawMessage) bool {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.EqualFold(text, "disabled") || text == "0"
	}
	var number float64
	if json.Unmarshal(raw, &number) == nil {
		return number == 0
	}
	return false
}
