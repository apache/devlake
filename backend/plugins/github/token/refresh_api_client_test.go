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

package token

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRefreshApiClientPost(t *testing.T) {
	var receivedMethod string
	var receivedPath string
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"token":"ghs_test123","expires_at":"2026-03-02T12:00:00Z"}`))
	}))
	defer server.Close()

	client := newRefreshApiClientWithTransport(server.URL, server.Client().Transport)

	headers := http.Header{
		"Authorization": []string{"Bearer jwt_token_here"},
	}
	resp, err := client.Post("/app/installations/123/access_tokens", nil, nil, headers)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "POST", receivedMethod)
	assert.Equal(t, "/app/installations/123/access_tokens", receivedPath)
	assert.Equal(t, "Bearer jwt_token_here", receivedAuth)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Contains(t, string(body), "ghs_test123")
}

func TestRefreshApiClientGet(t *testing.T) {
	var receivedMethod string
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedQuery = r.URL.Query().Get("page")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := newRefreshApiClientWithTransport(server.URL, server.Client().Transport)

	resp, err := client.Get("/test", map[string][]string{"page": {"2"}}, nil)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "GET", receivedMethod)
	assert.Equal(t, "2", receivedQuery)
	resp.Body.Close()
}

func TestRefreshApiClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the timeout
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Use a client without its own timeout so the context timeout is the only constraint
	client := newRefreshApiClientWithTransport(server.URL, http.DefaultTransport)
	// Override the timeout to something short for the test
	client.(*refreshApiClient).timeout = 100 * time.Millisecond

	resp, err := client.Post("/slow", nil, nil, nil)
	assert.NotNil(t, err)
	if resp != nil {
		resp.Body.Close()
	}
	// Verify the error is a deadline/context error
	assert.True(t, strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "context"),
		"expected deadline/context error, got: %s", err.Error())
}
