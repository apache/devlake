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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
		wantErr  bool
	}{
		{
			name:     "InCloud URL with /youtrack prefix",
			endpoint: "https://example.myjetbrains.com/youtrack/api",
			want:     "https://example.myjetbrains.com/youtrack/api",
		},
		{
			name:     "youtrack.cloud URL",
			endpoint: "https://example.youtrack.cloud/api",
			want:     "https://example.youtrack.cloud/api",
		},
		{
			name:     "self-hosted without context path",
			endpoint: "https://youtrack.example.com/api",
			want:     "https://youtrack.example.com/api",
		},
		{
			name:     "plain http is accepted",
			endpoint: "http://localhost:8080/youtrack/api",
			want:     "http://localhost:8080/youtrack/api",
		},
		{
			name:     "trailing slash is normalized away",
			endpoint: "https://example.myjetbrains.com/youtrack/api/",
			want:     "https://example.myjetbrains.com/youtrack/api",
		},
		{
			name:     "missing /api suffix",
			endpoint: "https://example.myjetbrains.com/youtrack",
			wantErr:  true,
		},
		{
			name:     "host root without any path",
			endpoint: "https://example.myjetbrains.com",
			wantErr:  true,
		},
		{
			name:     "non-absolute URL",
			endpoint: "example.myjetbrains.com/youtrack/api",
			wantErr:  true,
		},
		{
			name:     "non-http(s) scheme",
			endpoint: "ftp://example.com/api",
			wantErr:  true,
		},
		{
			name:     "empty endpoint",
			endpoint: "",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateEndpoint(tt.endpoint)
			if tt.wantErr {
				assert.NotNil(t, err)
				// the rejection must tell the user exactly what shape is expected
				assert.Contains(t, err.Error(), "/api")
				assert.Contains(t, err.Error(), "https://example.myjetbrains.com/youtrack/api")
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckAuthenticated(t *testing.T) {
	// a bare 2xx proves nothing: YouTrack silently degrades to the guest user
	// when the token is missing or not accepted, so guest must fail loudly
	err := checkAuthenticated(youtrackUser{Id: "1-2", Login: "guest", Guest: true})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "authenticated as guest")
	assert.Contains(t, err.Error(), "permanent token")

	// a real user passes, and the login is what the success message names
	assert.Nil(t, checkAuthenticated(youtrackUser{Id: "1-2", Login: "jane.doe", Guest: false}))
}

func TestDiagnoseTestFailure(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		statusCode   int
		wantContains []string
	}{
		{
			name:         "401 means the token was rejected",
			endpoint:     "https://example.myjetbrains.com/youtrack/api",
			statusCode:   401,
			wantContains: []string{"authentication failed", "token"},
		},
		{
			name:         "404 on InCloud without /youtrack hints at the missing prefix",
			endpoint:     "https://example.myjetbrains.com/api",
			statusCode:   404,
			wantContains: []string{"/youtrack", "https://example.myjetbrains.com/youtrack/api"},
		},
		{
			name:         "404 on InCloud with /youtrack is a generic not-found",
			endpoint:     "https://example.myjetbrains.com/youtrack/api",
			statusCode:   404,
			wantContains: []string{"not found", "/api"},
		},
		{
			name:         "404 on InCloud with a /youtrack-looking segment still gets the prefix hint",
			endpoint:     "https://example.myjetbrains.com/youtrackfoo/api",
			statusCode:   404,
			wantContains: []string{"/youtrack", "https://example.myjetbrains.com/youtrack/api"},
		},
		{
			name:         "404 on youtrack.cloud is a generic not-found",
			endpoint:     "https://example.youtrack.cloud/api",
			statusCode:   404,
			wantContains: []string{"not found", "/api"},
		},
		{
			name:         "404 on self-hosted is a generic not-found",
			endpoint:     "https://youtrack.example.com/api",
			statusCode:   404,
			wantContains: []string{"not found", "/api"},
		},
		{
			name:         "5xx surfaces the status code",
			endpoint:     "https://example.myjetbrains.com/youtrack/api",
			statusCode:   504,
			wantContains: []string{"504"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := diagnoseTestFailure(tt.endpoint, tt.statusCode)
			assert.NotNil(t, err)
			for _, fragment := range tt.wantContains {
				assert.Contains(t, err.Error(), fragment)
			}
		})
	}
}

func TestSuccessMessage(t *testing.T) {
	// the happy path names the token's login
	msg := successMessage("jane.doe", "", "", "perm:am9obi5kb2U")
	assert.Contains(t, msg, "success")
	assert.Contains(t, msg, "jane.doe")
	assert.NotContains(t, msg, "permanent token")

	// the server version is reported when the best-effort probe succeeds
	msg = successMessage("jane.doe", "2025.3", "12345", "perm:am9obi5kb2U")
	assert.Contains(t, msg, "2025.3")
	assert.Contains(t, msg, "12345")

	// a non-perm: token shape earns a soft warning, not a failure
	msg = successMessage("jane.doe", "", "", "pasted-something-else")
	assert.Contains(t, msg, "success")
	assert.Contains(t, msg, "perm:")

	// the perm- prefix variant is also accepted silently
	msg = successMessage("jane.doe", "", "", "perm-am9obi5kb2U")
	assert.NotContains(t, msg, "perm:")
}

// fakeYoutrack is a minimal stand-in for a YouTrack instance: it answers
// users/me and config under {base}/api and records the Authorization header.
type fakeYoutrack struct {
	server    *httptest.Server
	user      map[string]interface{}
	userCode  int
	config    map[string]interface{}
	configHit bool
	lastAuth  string
}

func newFakeYoutrack(t *testing.T) *fakeYoutrack {
	f := &fakeYoutrack{
		user:     map[string]interface{}{"id": "1-2", "login": "jane.doe", "fullName": "Jane Doe", "guest": false},
		userCode: http.StatusOK,
		config:   map[string]interface{}{"version": "2025.3", "build": "12345"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users/me", func(w http.ResponseWriter, r *http.Request) {
		f.lastAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.userCode)
		if f.userCode == http.StatusOK {
			assert.NoError(t, json.NewEncoder(w).Encode(f.user))
		}
	})
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		f.configHit = true
		w.Header().Set("Content-Type", "application/json")
		if f.config == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(f.config))
	})
	// any other path answers 404, like a wrong base URL would
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeYoutrack) endpoint() string {
	return fmt.Sprintf("%s/api", f.server.URL)
}

// setupTestGlobals points the package globals at throwaway instances, the way
// Init does in production.
func setupTestGlobals() {
	mockRes := unithelper.DummyBasicRes(func(mockDal *mockdal.Dal) {})
	mockRes.On("GetConfigReader").Return(config.GetConfig())
	basicRes = mockRes
	vld = validator.New()
}

func TestTestConnectionWithFakeServer(t *testing.T) {
	setupTestGlobals()

	t.Run("success: sends the Bearer header and names the token's login", func(t *testing.T) {
		fake := newFakeYoutrack(t)
		connection := models.YoutrackConn{}
		connection.Endpoint = fake.endpoint()
		connection.Token = "perm:am9obi5kb2U"

		result, err := testConnection(context.TODO(), connection)
		assert.Nil(t, err)
		assert.True(t, result.Success)
		assert.Contains(t, result.Message, "jane.doe")
		// the permanent token goes out verbatim as a Bearer credential
		assert.Equal(t, "Bearer perm:am9obi5kb2U", fake.lastAuth)
		// the version probe ran and fed the message
		assert.True(t, fake.configHit)
		assert.Contains(t, result.Message, "2025.3")
		// the echoed connection is sanitized
		assert.NotNil(t, result.Connection)
		assert.NotContains(t, result.Connection.Token, "am9obi5kb2U")
	})

	t.Run("guest fallback fails even though the server answered 200", func(t *testing.T) {
		fake := newFakeYoutrack(t)
		fake.user = map[string]interface{}{"id": "1-1", "login": "guest", "fullName": "guest", "guest": true}
		connection := models.YoutrackConn{}
		connection.Endpoint = fake.endpoint()
		connection.Token = "garbage-token"

		result, err := testConnection(context.TODO(), connection)
		assert.Nil(t, result)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "authenticated as guest")
	})

	t.Run("401 means the token was rejected", func(t *testing.T) {
		fake := newFakeYoutrack(t)
		fake.userCode = http.StatusUnauthorized
		connection := models.YoutrackConn{}
		connection.Endpoint = fake.endpoint()
		connection.Token = "perm:revoked"

		result, err := testConnection(context.TODO(), connection)
		assert.Nil(t, result)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "authentication failed")
	})

	t.Run("a failing version probe degrades silently", func(t *testing.T) {
		fake := newFakeYoutrack(t)
		fake.config = nil
		connection := models.YoutrackConn{}
		connection.Endpoint = fake.endpoint()
		connection.Token = "perm:am9obi5kb2U"

		result, err := testConnection(context.TODO(), connection)
		assert.Nil(t, err)
		assert.True(t, result.Success)
		assert.Contains(t, result.Message, "jane.doe")
	})

	t.Run("a malformed endpoint is rejected before any HTTP call", func(t *testing.T) {
		connection := models.YoutrackConn{}
		connection.Endpoint = "https://example.myjetbrains.com/youtrack"
		connection.Token = "perm:am9obi5kb2U"

		result, err := testConnection(context.TODO(), connection)
		assert.Nil(t, result)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "/api")
	})

	t.Run("an unreachable server fails with the transport error", func(t *testing.T) {
		fake := newFakeYoutrack(t)
		endpoint := fake.endpoint()
		fake.server.Close() // nothing listening anymore: connection refused

		connection := models.YoutrackConn{}
		connection.Endpoint = endpoint
		connection.Token = "perm:am9obi5kb2U"

		result, err := testConnection(context.TODO(), connection)
		assert.Nil(t, result)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "connect")
	})
}
