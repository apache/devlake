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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	implcontext "github.com/apache/devlake/impls/context"
	"github.com/apache/devlake/impls/logruslog"
	"github.com/apache/devlake/plugins/grafana_irm/models"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestDeps(t *testing.T) {
	t.Helper()
	basicRes = implcontext.NewDefaultBasicRes(config.GetConfig(), logruslog.Global, nil)
	vld = validator.New()
}

func TestTestConnection_DnsFailure(t *testing.T) {
	initTestDeps(t)
	conn := models.GrafanaIrmConn{
		RestConnection: api.RestConnection{
			Endpoint: "https://maroonmacaron2191.grafannet/",
		},
		GrafanaIrmAccessToken: models.GrafanaIrmAccessToken{
			Token: "dummy-token",
		},
	}
	_, err := testConnection(context.Background(), conn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to resolve hostname for 'https://maroonmacaron2191.grafannet/'")
	assert.Contains(t, err.Error(), "Please check your Grafana Cloud URL for typos")
	// Verify internal cockroachdb wrap dumps are not present
	assert.NotContains(t, err.Error(), "Wraps:")
	assert.NotContains(t, err.Error(), "*hintdetail.withDetail")
}

func TestTestConnection_Success(t *testing.T) {
	initTestDeps(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/plugins/grafana-irm-app/resources/api/v1/IncidentsService.QueryIncidents", r.URL.Path)
		assert.Equal(t, "Bearer valid-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"incidents":[]}`))
	}))
	defer ts.Close()

	conn := models.GrafanaIrmConn{
		RestConnection: api.RestConnection{
			Endpoint: ts.URL + "/",
		},
		GrafanaIrmAccessToken: models.GrafanaIrmAccessToken{
			Token: "valid-token",
		},
	}
	out, err := testConnection(context.Background(), conn)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, out.Status)
}

func TestTestConnection_Unauthorized(t *testing.T) {
	initTestDeps(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	conn := models.GrafanaIrmConn{
		RestConnection: api.RestConnection{
			Endpoint: ts.URL + "/",
		},
		GrafanaIrmAccessToken: models.GrafanaIrmAccessToken{
			Token: "invalid-token",
		},
	}
	_, err := testConnection(context.Background(), conn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Authentication failed: invalid Service Account token")
}

func TestTestConnection_NotFound(t *testing.T) {
	initTestDeps(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	conn := models.GrafanaIrmConn{
		RestConnection: api.RestConnection{
			Endpoint: ts.URL + "/",
		},
		GrafanaIrmAccessToken: models.GrafanaIrmAccessToken{
			Token: "valid-token",
		},
	}
	_, err := testConnection(context.Background(), conn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Grafana IRM app endpoint not found (HTTP 404)")
}

func TestTestConnection_ConnectRefused(t *testing.T) {
	initTestDeps(t)
	// Connect to an unreachable local port to trigger connection refused immediately
	conn := models.GrafanaIrmConn{
		RestConnection: api.RestConnection{
			Endpoint: "http://127.0.0.1:54321/",
		},
		GrafanaIrmAccessToken: models.GrafanaIrmAccessToken{
			Token: "valid-token",
		},
	}
	_, err := testConnection(context.Background(), conn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to connect to 'http://127.0.0.1:54321/'")
	assert.Contains(t, err.Error(), "Please check that the URL is spelled correctly")
	assert.NotContains(t, err.Error(), "Wraps:")
	assert.NotContains(t, err.Error(), "*hintdetail.withDetail")
}
