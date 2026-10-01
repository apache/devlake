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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apache/devlake/core/config"
	implcontext "github.com/apache/devlake/impls/context"
	"github.com/apache/devlake/impls/logruslog"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestDeps wires up the package-level basicRes and vld.
// DefaultBasicRes satisfies context.BasicRes and only needs config + logger;
// dal can be nil because NewApiClientFromConnection never calls GetDal.
func initTestDeps(t *testing.T) {
	t.Helper()
	basicRes = implcontext.NewDefaultBasicRes(config.GetConfig(), logruslog.Global, nil)
	vld = validator.New()
}

// serverInfoResponse is the minimal Jira serverInfo payload used in tests.
type serverInfoResponse struct {
	DeploymentType string `json:"deploymentType"`
	Version        string `json:"version"`
	VersionNumbers []int  `json:"versionNumbers"`
}

// newJiraTestServer creates an httptest.Server that responds to Jira REST paths.
// It records the Authorization header sent by the client for later assertion.
func newJiraTestServer(t *testing.T, authHeaderOut *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHeaderOut != nil {
			*authHeaderOut = r.Header.Get("Authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/2/serverInfo"):
			json.NewEncoder(w).Encode(serverInfoResponse{
				DeploymentType: "Cloud",
				Version:        "1001.0.0",
				VersionNumbers: []int{1001, 0, 0},
			})
		case strings.Contains(r.URL.Path, "/agile/1.0/board"):
			json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
		default:
			http.NotFound(w, r)
		}
	}))
}

// TestTestConnection_Gateway verifies that a gateway-style endpoint with
// authMethod=AccessToken sends "Authorization: Bearer <token>" and succeeds.
func TestTestConnection_Gateway(t *testing.T) {
	initTestDeps(t)

	var capturedAuth string
	srv := newJiraTestServer(t, &capturedAuth)
	defer srv.Close()

	conn := models.JiraConn{}
	conn.Endpoint = srv.URL + "/rest/"
	conn.AuthMethod = "AccessToken"
	conn.Token = "my-scoped-gateway-token"

	resp, err := testConnection(context.Background(), conn)
	require.Nil(t, err, "testConnection should succeed for gateway endpoint")
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "Bearer my-scoped-gateway-token", capturedAuth,
		"gateway mode must send Bearer token, not Basic credentials")
}

// TestTestConnection_StandardCloud verifies backward compatibility:
// a standard atlassian.net endpoint with BasicAuth still works.
func TestTestConnection_StandardCloud(t *testing.T) {
	initTestDeps(t)

	var capturedAuth string
	srv := newJiraTestServer(t, &capturedAuth)
	defer srv.Close()

	conn := models.JiraConn{}
	conn.Endpoint = srv.URL + "/rest/"
	conn.AuthMethod = "BasicAuth"
	conn.Username = "user@example.com"
	conn.Password = "api-token-value"

	resp, err := testConnection(context.Background(), conn)
	require.Nil(t, err, "testConnection should succeed for standard cloud endpoint")
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.True(t, strings.HasPrefix(capturedAuth, "Basic "),
		"standard cloud mode must send Basic auth header")
}

// TestTestConnection_NonGateway404ShowsHint verifies that when a non-gateway endpoint
// is missing /rest/ and returns 404, the helpful hint message is included in the error.
func TestTestConnection_NonGateway404ShowsHint(t *testing.T) {
	initTestDeps(t)

	// Server that always returns 404
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// URL intentionally missing /rest/ — should trigger the "please try" hint
	connMissingRest := models.JiraConn{}
	connMissingRest.Endpoint = srv.URL + "/"
	connMissingRest.AuthMethod = "BasicAuth"
	connMissingRest.Username = "u"
	connMissingRest.Password = "p"

	_, errMissingRest := testConnection(context.Background(), connMissingRest)
	require.NotNil(t, errMissingRest)
	assert.Contains(t, errMissingRest.Error(), "please try",
		"non-gateway 404 should include the /rest/ hint")
}

func TestTestConnection_OAuth2RequiresCredentials(t *testing.T) {
	initTestDeps(t)

	conn := models.JiraConn{}
	conn.AuthMethod = models.AUTH_METHOD_OAUTH2
	conn.CloudId = "cloud-1"

	_, err := testConnection(context.Background(), conn)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "clientId, clientSecret and cloudId are required")
}

func TestJiraHTTPErrorDetail(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		header string
		want   string
	}{
		{
			name: "classic jira errorMessages",
			body: `{"errorMessages":["Client must be authenticated to access this resource."],"errors":{}}`,
			want: "Client must be authenticated to access this resource.",
		},
		{
			name: "atlassian gateway message",
			body: `{"code":401,"message":"Unauthorized; scope does not match"}`,
			want: "Unauthorized; scope does not match",
		},
		{
			name:   "www-authenticate when body empty",
			header: `Bearer error="insufficient_scope"`,
			want:   `Bearer error="insufficient_scope"`,
		},
		{
			name: "html body ignored",
			body: `<html><body>Unauthorized</body></html>`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}
			if tt.header != "" {
				res.Header.Set("WWW-Authenticate", tt.header)
			}
			if got := jiraHTTPErrorDetail(res); got != tt.want {
				t.Errorf("jiraHTTPErrorDetail() = %q, want %q", got, tt.want)
			}
		})
	}
}
