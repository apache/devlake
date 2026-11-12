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

package models

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/pluginhelper/api"
)

func TestGetEndpoint_CloudDefault(t *testing.T) {
	conn := &AzuredevopsConn{}
	endpoint := conn.GetEndpoint()
	if endpoint != "https://dev.azure.com/" {
		t.Errorf("GetEndpoint() = %q; want %q", endpoint, "https://dev.azure.com/")
	}
}

func TestGetEndpoint_OnPremises(t *testing.T) {
	conn := &AzuredevopsConn{
		Endpoint: "https://tfs.company.local/DefaultCollection/",
	}
	endpoint := conn.GetEndpoint()
	if endpoint != "https://tfs.company.local/DefaultCollection/" {
		t.Errorf("GetEndpoint() = %q; want %q", endpoint, "https://tfs.company.local/DefaultCollection/")
	}
}

func TestGetEndpoint_TrailingSlash(t *testing.T) {
	conn := &AzuredevopsConn{
		Endpoint: "https://tfs.company.local/DefaultCollection",
	}
	endpoint := conn.GetEndpoint()
	expected := "https://tfs.company.local/DefaultCollection/"
	if endpoint != expected {
		t.Errorf("GetEndpoint() = %q; want %q (trailing slash should be appended)", endpoint, expected)
	}
}

func TestGetEndpoint_ConnectionWrapper(t *testing.T) {
	conn := AzuredevopsConnection{
		AzuredevopsConn: AzuredevopsConn{
			Endpoint: "https://tfs.company.local/DefaultCollection/",
		},
	}
	endpoint := conn.GetEndpoint()
	if endpoint != "https://tfs.company.local/DefaultCollection/" {
		t.Errorf("AzuredevopsConnection.GetEndpoint() = %q; want %q", endpoint, "https://tfs.company.local/DefaultCollection/")
	}
}

func TestGetEndpoint_ConnectionWrapperCloud(t *testing.T) {
	conn := AzuredevopsConnection{}
	endpoint := conn.GetEndpoint()
	if endpoint != "https://dev.azure.com/" {
		t.Errorf("AzuredevopsConnection.GetEndpoint() = %q; want %q", endpoint, "https://dev.azure.com/")
	}
}

func TestSetupAuthentication_Cloud(t *testing.T) {
	conn := &AzuredevopsConn{
		AzuredevopsAccessToken: AzuredevopsAccessToken{
			Token: "test-pat-token",
		},
		Username: "testuser",
	}

	req, _ := http.NewRequest("GET", "https://dev.azure.com/org/_apis/projects", nil)
	err := conn.SetupAuthentication(req)
	if err != nil {
		t.Fatalf("SetupAuthentication() returned error: %v", err)
	}

	user, pass, ok := req.BasicAuth()
	if !ok {
		t.Fatal("SetupAuthentication() did not set Basic Auth header")
	}
	if user != "testuser" {
		t.Errorf("Basic Auth username = %q; want %q", user, "testuser")
	}
	if pass != "test-pat-token" {
		t.Errorf("Basic Auth password = %q; want %q", pass, "test-pat-token")
	}
}

func TestSetupAuthentication_OnPremises(t *testing.T) {
	conn := &AzuredevopsConn{
		AzuredevopsAccessToken: AzuredevopsAccessToken{
			Token: "on-prem-pat-token",
		},
		Username: "domain\\admin",
		Endpoint: "https://tfs.company.local/DefaultCollection/",
	}

	req, _ := http.NewRequest("GET", "https://tfs.company.local/DefaultCollection/_apis/projects", nil)
	err := conn.SetupAuthentication(req)
	if err != nil {
		t.Fatalf("SetupAuthentication() returned error: %v", err)
	}

	user, pass, ok := req.BasicAuth()
	if !ok {
		t.Fatal("SetupAuthentication() did not set Basic Auth header")
	}
	// On-Premises: username should be empty (PAT auth with empty username)
	if user != "" {
		t.Errorf("On-Premises Basic Auth username = %q; want empty string", user)
	}
	if pass != "on-prem-pat-token" {
		t.Errorf("On-Premises Basic Auth password = %q; want %q", pass, "on-prem-pat-token")
	}
}

func TestSanitize(t *testing.T) {
	conn := &AzuredevopsConn{
		AzuredevopsAccessToken: AzuredevopsAccessToken{
			Token: "secret-token",
		},
		Username: "testuser",
		Endpoint: "https://tfs.company.local/",
	}

	sanitized := conn.Sanitize()
	if sanitized.Endpoint != "https://tfs.company.local/" {
		t.Errorf("Sanitize() should preserve Endpoint, got %q", sanitized.Endpoint)
	}
	if sanitized.Username != "testuser" {
		t.Errorf("Sanitize() should preserve Username, got %q", sanitized.Username)
	}
}

func TestPrepareApiClientNegotiatesVersion(t *testing.T) {
	operations := []struct {
		path     string
		versions [4]string // 7.2, 7.1, 7.0, 6.0
		query    url.Values
	}{
		{"_apis/projects", [4]string{"7.2-preview.4", "7.1", "7.0", "6.0"}, url.Values{"$top": {"100"}, "$skip": {"100"}}},
		{"project/_apis/git/repositories", [4]string{"7.2-preview.2", "7.1", "7.0", "6.0"}, nil},
		{"project/_apis/git/repositories/repo/commits", [4]string{"7.2-preview.2", "7.1", "7.0", "6.0"}, url.Values{"$top": {"100"}, "$skip": {"200"}}},
		{"project/_apis/build/builds", [4]string{"7.2-preview.8", "7.1", "7.0", "6.0"}, url.Values{
			"repositoryId": {"repo"}, "repositoryType": {"TfsGit"}, "$top": {"100"},
			"queryOrder": {"queueTimeDescending"}, "minTime": {"2026-01-01T00:00:00Z"}, "continuationToken": {"next"},
		}},
		{"project/_apis/build/builds/1/Timeline", [4]string{"7.2-preview.3", "7.1", "7.0", "6.0"}, nil},
		{"project/_apis/git/repositories/repo/pullrequests", [4]string{"7.2-preview.2", "7.1", "7.0", "6.0"}, url.Values{
			"searchCriteria.status": {"all"}, "$skip": {"100"}, "$top": {"100"},
			"searchCriteria.queryTimeRangeType": {"created"}, "searchCriteria.minTime": {"2026-01-01T00:00:00Z"},
			"searchCriteria.maxTime": {"2026-02-01T00:00:00Z"},
		}},
		{"project/_apis/git/repositories/repo/pullRequests/1/commits", [4]string{"7.2-preview.1", "7.1", "7.0", "6.0"}, url.Values{"$top": {"100"}, "continuationToken": {"next"}}},
		{"project/_apis/serviceendpoint/endpoints/", [4]string{"7.2-preview.4", "7.1", "7.0", "6.0-preview.4"}, nil},
		{"project/_apis/sourceProviders/github/repositories/", [4]string{"7.2-preview.1", "7.1", "7.0", "6.0-preview.1"}, url.Values{
			"serviceEndpointId": {"endpoint"}, "pageResults": {"true"}, "continuationToken": {"next"},
		}},
	}
	for i, version := range []string{"7.2", "7.1", "7.0", "6.0"} {
		t.Run(version, func(t *testing.T) {
			var probes []string
			expectedQueries := make(map[string]url.Values)
			for _, operation := range operations {
				query := url.Values{"api-version": {operation.versions[i]}}
				for key, values := range operation.query {
					query[key] = values
				}
				if i >= 2 && strings.HasSuffix(operation.path, "/pullrequests") {
					query.Del("searchCriteria.minTime")
					query.Del("searchCriteria.maxTime")
					query.Del("searchCriteria.queryTimeRangeType")
				}
				expectedQueries["/tfs/DefaultCollection/"+operation.path] = query
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, token, ok := r.BasicAuth()
				if !ok || user != "" || token != "test-token" {
					t.Error("version detection and subsequent requests must authenticate")
				}
				if r.Header.Get("X-Existing") != "preserved" {
					t.Error("existing before-request hook was lost")
				}
				got := r.URL.Query().Get("api-version")
				if r.URL.Query().Get("$top") == "1" {
					if r.URL.Path != "/tfs/DefaultCollection/_apis/projects" {
						t.Errorf("unexpected probe path: %s", r.URL.Path)
					}
					probes = append(probes, got)
					if got != operations[0].versions[i] {
						w.WriteHeader(http.StatusBadRequest)
						if version == "6.0" {
							fmt.Fprint(w, `{"typeKey":"VssVersionOutOfRangeException"}`)
						} else {
							fmt.Fprint(w, `{"typeKey":"VssVersionNotSupportedException"}`)
						}
						return
					}
				} else if want, ok := expectedQueries[r.URL.Path]; !ok {
					t.Errorf("unexpected request URL: %s", r.URL)
				} else if !reflect.DeepEqual(r.URL.Query(), want) {
					t.Errorf("%s query = %v; want %v", r.URL.Path, r.URL.Query(), want)
				}
				fmt.Fprint(w, `{"count":0,"value":[]}`)
			}))
			defer server.Close()
			conn := AzuredevopsConn{
				Endpoint:               server.URL + "/tfs/DefaultCollection/",
				Organization:           "cloud-only-organization",
				AzuredevopsAccessToken: AzuredevopsAccessToken{Token: "test-token"},
			}
			client := &api.ApiClient{}
			client.Setup(conn.GetEndpoint(), nil, 0)
			client.SetBeforeFunction(func(req *http.Request) errors.Error {
				req.Header.Set("X-Existing", "preserved")
				return nil
			})
			if err := conn.PrepareApiClient(client); err != nil {
				t.Fatal(err)
			}
			client.SetAuthFunction(conn.SetupAuthentication)
			for _, operation := range operations {
				res, err := client.Get(operation.path+"?api-version=7.1", operation.query, nil)
				if err != nil {
					t.Fatal(err)
				}
				res.Body.Close()
			}
			want := []string{"7.2-preview.4", "7.1", "7.0", "6.0"}[:i+1]
			if !reflect.DeepEqual(probes, want) {
				t.Errorf("probes = %v; want %v", probes, want)
			}
		})
	}
}

func TestPrepareApiClientRejectsFailedDetection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		attempts int
	}{
		{"unauthorized", 401, `{"value":[]}`, 1},
		{"forbidden", 403, `{"value":[]}`, 1},
		{"login page", 203, `<html>Sign in</html>`, 1},
		{"html success", 200, `<html>Sign in</html>`, 1},
		{"wrong endpoint", 200, `{}`, 1},
		{"not found", 404, `{"typeKey":"VssVersionNotSupportedException"}`, 1},
		{"out of range not found", 404, `{"typeKey":"VssVersionOutOfRangeException"}`, 1},
		{"unrelated bad request", 400, `{"typeKey":"ArgumentException"}`, 1},
		{"server error", 500, `{"typeKey":"VssVersionNotSupportedException"}`, 1},
		{"out of range server error", 500, `{"typeKey":"VssVersionOutOfRangeException"}`, 1},
		{"unsupported server", 400, `{"typeKey":"VssVersionNotSupportedException"}`, 4},
		{"out of range server", 400, `{"typeKey":"VssVersionOutOfRangeException"}`, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			conn := AzuredevopsConn{Endpoint: server.URL}
			client := &api.ApiClient{}
			client.Setup(conn.GetEndpoint(), nil, 0)
			if err := conn.PrepareApiClient(client); err == nil {
				t.Fatal("expected a detection error")
			}
			if calls != tc.attempts {
				t.Errorf("probe count = %d; want %d", calls, tc.attempts)
			}
			if client.GetBeforeFunction() != nil {
				t.Error("temporary authentication hook was not restored")
			}
		})
	}
}

func TestPrepareApiClientCloudPreservesPreviewVersion(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		want := DefaultAPIVersion
		if r.URL.Path == "/_apis/graph/users" {
			want = "7.0-preview.1"
		}
		if got := r.URL.Query().Get("api-version"); got != want {
			t.Errorf("api-version = %q; want %q", got, want)
		}
		fmt.Fprint(w, `{"value":[]}`)
	}))
	defer server.Close()
	conn := AzuredevopsConn{}
	client := &api.ApiClient{}
	client.Setup(server.URL, nil, 0)
	if err := conn.PrepareApiClient(client); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("Cloud must not probe for a server version")
	}
	for _, path := range []string{"_apis/projects", "_apis/graph/users?api-version=7.0-preview.1"} {
		res, err := client.Get(path, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
}
