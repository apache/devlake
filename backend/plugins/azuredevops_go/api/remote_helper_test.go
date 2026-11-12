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
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	mockcontext "github.com/apache/devlake/mocks/core/context"
	mocklog "github.com/apache/devlake/mocks/core/log"
	"github.com/apache/devlake/plugins/azuredevops_go/api/azuredevops"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// fakeApiClient answers GET requests from a fixed map of "path?serviceEndpointId" to response bodies.
// A missing entry is returned as a request error.
type fakeApiClient struct {
	plugin.ApiClient
	responses map[string]string
}

func (f *fakeApiClient) Get(path string, query url.Values, _ http.Header) (*http.Response, errors.Error) {
	key := path
	if id := query.Get("serviceEndpointId"); id != "" {
		key += "?" + id
	}
	body, ok := f.responses[key]
	if !ok {
		return nil, errors.Unauthorized.New("request failed: " + key)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://dev.azure.com/"+path, nil)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func TestListRemoteReposSkipsFailingServiceConnection(t *testing.T) {
	// the failing service connection must be logged once and then skipped
	logger := new(mocklog.Logger)
	logger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Once()
	res := new(mockcontext.BasicRes)
	res.On("GetLogger").Return(logger)
	previous := basicRes
	basicRes = res
	defer func() { basicRes = previous }()

	client := &fakeApiClient{responses: map[string]string{
		"org/project/_apis/serviceendpoint/endpoints/": `{"value": [
			{"id": "broken-endpoint", "name": "broken", "type": "bitbucket"},
			{"id": "working-endpoint", "name": "working", "type": "github"}
		]}`,
		"org/project/_apis/sourceProviders/github/repositories/?working-endpoint": `{"repositories": [
			{"id": "octo/repo", "sourceProviderName": "GitHub", "name": "repo", "fullName": "octo/repo",
			 "properties": {"connectedServiceId": "working-endpoint", "externalId": "123"}}
		]}`,
		// no entry for the broken endpoint: its request fails
	}}
	vsc := azuredevops.NewClient(&models.AzuredevopsConnection{}, client, "")

	children, err := listRemoteRepos(vsc, "org", "project")

	assert.Nil(t, err)
	if assert.Len(t, children, 1) {
		assert.Equal(t, "octo/repo", children[0].FullName)
		assert.Equal(t, "working-endpoint-123", children[0].Data.Id)
		assert.Equal(t, "org/project", *children[0].ParentId)
	}
	logger.AssertExpectations(t)
}

func TestOnPremisesScopeDiscoveryIgnoresCloudOrganization(t *testing.T) {
	connection := &models.AzuredevopsConnection{AzuredevopsConn: models.AzuredevopsConn{
		Endpoint: "https://ado.example.com/tfs/DefaultCollection/", Organization: "cloud-org",
	}}
	client := &fakeApiClient{responses: map[string]string{
		"_apis/projects":                                 `{"count":1,"value":[{"id":"project","name":"Project One"}]}`,
		"Project%20One/_apis/git/repositories":           `{"value":[{"id":"native","name":"native"}]}`,
		"Project%20One/_apis/serviceendpoint/endpoints/": `{"value":[{"id":"service","type":"github"}]}`,
		"Project%20One/_apis/sourceProviders/github/repositories/?service": `{"repositories":[
			{"id":"external","name":"external","fullName":"org/external","sourceProviderName":"github",
			 "properties":{"connectedServiceId":"service","externalId":"external"}}
		]}`,
	}}
	groups, _, err := listAzuredevopsRemoteScopes(connection, client, "", AzuredevopsRemotePagination{})
	assert.Nil(t, err)
	if assert.Len(t, groups, 1) {
		assert.Equal(t, "Project One", groups[0].Id)
	}
	repos, _, err := listAzuredevopsRemoteScopes(connection, client, "Project One", AzuredevopsRemotePagination{})
	assert.Nil(t, err)
	if assert.Len(t, repos, 2) {
		for _, repo := range repos {
			assert.Equal(t, "Project One", *repo.ParentId)
			assert.Empty(t, repo.Data.OrganizationId)
			assert.Equal(t, "Project One", repo.Data.ProjectId)
		}
	}
}
