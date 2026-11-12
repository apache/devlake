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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
	"testing"
)

func TestOnPremisesScopeRequestsAcrossVersions(t *testing.T) {
	for _, tc := range []struct {
		version  string
		projects string
		repos    string
		service  string
		source   string
	}{
		{"6.0", "6.0", "6.0", "6.0-preview.4", "6.0-preview.1"},
		{"7.0", "7.0", "7.0", "7.0", "7.0"},
		{"7.1", "7.1", "7.1", "7.1", "7.1"},
		{"7.2", "7.2-preview.4", "7.2-preview.2", "7.2-preview.4", "7.2-preview.1"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			sourcePages := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/tfs/DefaultCollection/")
				query := r.URL.Query()
				version := query.Get("api-version")
				if user, token, ok := r.BasicAuth(); !ok || user != "" || token != "test-token" {
					t.Error("expected authenticated on-premises request")
				}
				if path == "_apis/projects" && query.Get("$top") == "1" && version != tc.projects {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, `{"typeKey":"VssVersionNotSupportedException"}`)
					return
				}
				switch path {
				case "_apis/projects":
					if version != tc.projects {
						t.Errorf("project API version = %s; want %s", version, tc.projects)
					}
					fmt.Fprint(w, `{"count":1,"value":[{"id":"project","name":"Project One"}]}`)
				case "Project One/_apis/git/repositories":
					if version != tc.repos {
						t.Errorf("repository API version = %s; want %s", version, tc.repos)
					}
					fmt.Fprint(w, `{"value":[{"id":"repo","name":"repo"}]}`)
				case "Project One/_apis/serviceendpoint/endpoints/":
					if version != tc.service {
						t.Errorf("service endpoint API version = %s; want %s", version, tc.service)
					}
					fmt.Fprint(w, `{"value":[{"id":"endpoint","type":"github"}]}`)
				case "Project One/_apis/sourceProviders/github/repositories/":
					sourcePages++
					if version != tc.source || query.Get("serviceEndpointId") != "endpoint" || query.Get("pageResults") != "true" {
						t.Errorf("unexpected source provider query: %v", query)
					}
					if sourcePages == 1 {
						if query.Get("continuationToken") != "" {
							t.Error("first page must not have a continuation token")
						}
						fmt.Fprint(w, `{"repositories":[{"id":"first"}],"continuationToken":"next-page"}`)
					} else {
						if sourcePages != 2 || query.Get("continuationToken") != "next-page" {
							t.Errorf("unexpected continuation request: page %d, query %v", sourcePages, query)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						fmt.Fprint(w, `{"repositories":[{"id":"second"}]}`)
					}
				default:
					t.Errorf("unexpected on-premises URL: %s", r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			conn := &models.AzuredevopsConnection{AzuredevopsConn: models.AzuredevopsConn{
				Endpoint: server.URL + "/tfs/DefaultCollection", Organization: "cloud-org",
				AzuredevopsAccessToken: models.AzuredevopsAccessToken{Token: "test-token"},
			}}
			apiClient := &api.ApiClient{}
			apiClient.Setup(conn.GetEndpoint(), nil, 0)
			if err := conn.PrepareApiClient(apiClient); err != nil {
				t.Fatal(err)
			}
			apiClient.SetAuthFunction(conn.SetupAuthentication)
			client := NewClient(conn, apiClient, conn.GetEndpoint())
			projects, err := client.GetProjects(GetProjectsArgs{OrgId: "cloud-org"})
			if err != nil || len(projects) != 1 {
				t.Fatalf("projects = %v, error = %v", projects, err)
			}
			repos, err := client.GetRepositories(GetRepositoriesArgs{OrgId: "cloud-org", ProjectId: "Project One"})
			if err != nil || len(repos) != 1 {
				t.Fatalf("repositories = %v, error = %v", repos, err)
			}
			endpoints, err := client.GetServiceEndpoints(GetServiceEndpointsArgs{OrgId: "cloud-org", ProjectId: "Project One"})
			if err != nil || len(endpoints) != 1 {
				t.Fatalf("service endpoints = %v, error = %v", endpoints, err)
			}
			remote, remoteErr := client.GetRemoteRepositories(GetRemoteRepositoriesArgs{
				OrgId: "cloud-org", ProjectId: "Project One", Provider: "github", ServiceEndpoint: "endpoint",
			})
			if remoteErr != nil || len(remote) != 2 || sourcePages != 2 {
				t.Fatalf("remote repositories = %v, pages = %d, error = %v", remote, sourcePages, remoteErr)
			}
		})
	}
}

func TestGetRemoteRepositoriesRejectsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"message":"API version not supported"}`)
	}))
	defer server.Close()
	apiClient := &api.ApiClient{}
	apiClient.Setup(server.URL, nil, 0)
	client := NewClient(&models.AzuredevopsConnection{}, apiClient, server.URL)
	repos, err := client.GetRemoteRepositories(GetRemoteRepositoriesArgs{
		OrgId: "org", ProjectId: "project", Provider: "github",
	})
	if err == nil || len(repos) != 0 {
		t.Fatalf("expected upstream error, got repositories = %v, error = %v", repos, err)
	}
}

func TestGetUserProfile_OnPremises(t *testing.T) {
	conn := &models.AzuredevopsConnection{
		BaseConnection: api.BaseConnection{},
		AzuredevopsConn: models.AzuredevopsConn{
			AzuredevopsAccessToken: models.AzuredevopsAccessToken{
				Token: "test-token",
			},
			Endpoint: "https://tfs.company.local/DefaultCollection/",
		},
	}

	client := NewClient(conn, nil, "https://tfs.company.local/DefaultCollection/")
	profile, err := client.GetUserProfile()
	if err != nil {
		t.Fatalf("GetUserProfile() returned unexpected error for On-Premises: %v", err)
	}
	if profile.DisplayName != "On-Premises User" {
		t.Errorf("GetUserProfile() DisplayName = %q; want %q", profile.DisplayName, "On-Premises User")
	}
}

func TestGetUserAccounts_OnPremises(t *testing.T) {
	conn := &models.AzuredevopsConnection{
		BaseConnection: api.BaseConnection{},
		AzuredevopsConn: models.AzuredevopsConn{
			AzuredevopsAccessToken: models.AzuredevopsAccessToken{
				Token: "test-token",
			},
			Endpoint: "https://tfs.company.local/DefaultCollection/",
		},
	}

	client := NewClient(conn, nil, "https://tfs.company.local/DefaultCollection/")
	accounts, err := client.GetUserAccounts("test-member-id")
	if err != nil {
		t.Fatalf("GetUserAccounts() returned unexpected error for On-Premises: %v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("GetUserAccounts() returned %d accounts; want 0 for On-Premises", len(accounts))
	}
}

func TestGetUserProfile_Cloud(t *testing.T) {
	// For Cloud connections (no Endpoint set), GetUserProfile should NOT return
	// the placeholder profile. It should attempt to call the VSSPS API.
	conn := &models.AzuredevopsConnection{
		BaseConnection: api.BaseConnection{},
		AzuredevopsConn: models.AzuredevopsConn{
			AzuredevopsAccessToken: models.AzuredevopsAccessToken{
				Token: "test-token",
			},
			// Endpoint is empty = Cloud mode
		},
	}

	// Without a mock server, this will fail with a connection error,
	// which is expected - the important thing is it does NOT bypass to placeholder
	client := NewClient(conn, nil, "http://localhost:0")
	_, err := client.GetUserProfile()
	if err == nil {
		t.Error("GetUserProfile() for Cloud should attempt real API call and fail without a server")
	}
}

func TestGetUserAccounts_Cloud(t *testing.T) {
	conn := &models.AzuredevopsConnection{
		BaseConnection: api.BaseConnection{},
		AzuredevopsConn: models.AzuredevopsConn{
			AzuredevopsAccessToken: models.AzuredevopsAccessToken{
				Token: "test-token",
			},
			// Endpoint is empty = Cloud mode
		},
	}

	// Without a mock server, this will fail with a connection error
	client := NewClient(conn, nil, "http://localhost:0")
	_, err := client.GetUserAccounts("test-member-id")
	if err == nil {
		t.Error("GetUserAccounts() for Cloud should attempt real API call and fail without a server")
	}
}
