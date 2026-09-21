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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apache/devlake/plugins/jira/models"
)

func oauthConn(tokenURL string) *models.JiraConn {
	jc := &models.JiraConn{
		ClientId:      "cid",
		ClientSecret:  "csecret",
		CloudId:       "cloud-1",
		OAuthTokenURL: tokenURL,
	}
	jc.AuthMethod = models.AUTH_METHOD_OAUTH2
	return jc
}

func seedToken(tp *TokenProvider, token string, ttl time.Duration) {
	tp.cacheToken(token, time.Now().Add(ttl))
}

func TestNeedsRefresh(t *testing.T) {
	tp := &TokenProvider{conn: oauthConn("")}

	assert.True(t, tp.needsRefresh(), "empty token should refresh")

	seedToken(tp, "tok", 10*time.Minute)
	assert.False(t, tp.needsRefresh())

	seedToken(tp, "tok", 1*time.Minute)
	assert.True(t, tp.needsRefresh())

	seedToken(tp, "tok", -1*time.Minute)
	assert.True(t, tp.needsRefresh())
}

func TestGetTokenMintsWhenExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "fresh-token",
			"expires_in":   3600,
			"token_type":   "Bearer",
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)
	seedToken(tp, "stale", -time.Minute)

	token, err := tp.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "fresh-token", token)
}

func TestForceRefreshSkipsIfTokenChanged(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)
	seedToken(tp, "current", time.Hour)

	require.NoError(t, tp.ForceRefresh("stale-old-token"))
	assert.Equal(t, 0, calls)
	assert.Equal(t, "current", tp.token)

	require.NoError(t, tp.ForceRefresh("current"))
	assert.Equal(t, 1, calls)
	assert.Equal(t, "new-token", tp.token)
}

func TestGetTokenConcurrency(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "shared-token",
			"expires_in":   3600,
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, gerr := tp.GetToken()
			assert.NoError(t, gerr)
			assert.Equal(t, "shared-token", token)
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, calls)
}

func TestRoundTripper401Refresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)
	seedToken(tp, "old-token", 10*time.Minute)

	apiCalls := 0
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		apiCalls++
		auth := req.Header.Get("Authorization")
		if auth == "Bearer old-token" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		}
		if auth == "Bearer new-token" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		}
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(http.NoBody), Request: req}, nil
	})

	rt := NewRefreshRoundTripper(base, tp)
	req, reqErr := http.NewRequest(http.MethodGet, "https://api.atlassian.com/ex/jira/c/rest/api/2/serverInfo", nil)
	require.NoError(t, reqErr)

	resp, tripErr := rt.RoundTrip(req)
	require.NoError(t, tripErr)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, apiCalls)
	assert.Equal(t, "new-token", tp.token)
}

// TestRoundTripper401RetryRestoresBody: fake Jira 401s Bearer old-token, the
// round tripper remints via the token server, and retries with new-token.
// Both attempts must still send the original POST body.
func TestRoundTripper401RetryRestoresBody(t *testing.T) {
	// Token endpoint: ForceRefresh POSTs here and gets new-token.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)
	seedToken(tp, "old-token", 10*time.Minute)

	var bodies []string
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body, readErr := io.ReadAll(req.Body)
		require.NoError(t, readErr)
		bodies = append(bodies, string(body))
		if req.Header.Get("Authorization") == "Bearer old-token" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(http.NoBody),
			Request:    req,
		}, nil
	})

	rt := NewRefreshRoundTripper(base, tp)
	req, reqErr := http.NewRequest(http.MethodPost, "https://example.com/rest", strings.NewReader(`{"ok":true}`))
	require.NoError(t, reqErr)
	req.Header.Set("Content-Type", "application/json")

	resp, tripErr := rt.RoundTrip(req)
	require.NoError(t, tripErr)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// bodies[0] = first POST (old-token, 401); bodies[1] = retry (new-token, 200)
	require.Len(t, bodies, 2)
	assert.Equal(t, `{"ok":true}`, bodies[0])
	assert.Equal(t, `{"ok":true}`, bodies[1])
}

// TestRoundTripper401RetryRestoresBodyWithoutGetBody covers collectors that
// attach an io.Reader without http.NewRequest setting GetBody.
func TestRoundTripper401RetryRestoresBodyWithoutGetBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)
	seedToken(tp, "old-token", 10*time.Minute)

	var bodies []string
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body, readErr := io.ReadAll(req.Body)
		require.NoError(t, readErr)
		bodies = append(bodies, string(body))
		if req.Header.Get("Authorization") == "Bearer old-token" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(http.NoBody),
			Request:    req,
		}, nil
	})

	rt := NewRefreshRoundTripper(base, tp)
	req, reqErr := http.NewRequest(http.MethodPost, "https://example.com/rest", nil)
	require.NoError(t, reqErr)
	payload := `{"ok":true}`
	req.Body = io.NopCloser(strings.NewReader(payload))
	req.GetBody = nil
	req.ContentLength = int64(len(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, tripErr := rt.RoundTrip(req)
	require.NoError(t, tripErr)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, bodies, 2)
	assert.Equal(t, payload, bodies[0])
	assert.Equal(t, payload, bodies[1])
	assert.Nil(t, req.GetBody, "RoundTrip must not mutate the original request")
}

// TestRoundTripperPersistent401: fake Jira 401s even after remint. The round
// tripper must retry once, then return that 401 (no infinite loop).
func TestRoundTripperPersistent401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)
	seedToken(tp, "old-token", 10*time.Minute)

	apiCalls := 0
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		apiCalls++
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(http.NoBody),
			Request:    req,
		}, nil
	})

	rt := NewRefreshRoundTripper(base, tp)
	req, reqErr := http.NewRequest(http.MethodGet, "https://example.com/rest", nil)
	require.NoError(t, reqErr)

	resp, tripErr := rt.RoundTrip(req)
	require.NoError(t, tripErr)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, 2, apiCalls)
}

func TestRoundTripperRejectsOversizedBody(t *testing.T) {
	tp := &TokenProvider{conn: oauthConn("")}
	seedToken(tp, "tok", time.Hour)
	rt := NewRefreshRoundTripper(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("must not send an oversized body")
		return nil, nil
	}), tp)

	payload := strings.Repeat("a", int(maxRetryBodyBytes)+1)
	req, err := http.NewRequest(http.MethodPost, "https://example.com/rest", nil)
	require.NoError(t, err)
	req.Body = io.NopCloser(strings.NewReader(payload))
	req.GetBody = nil
	req.ContentLength = int64(len(payload))

	_, tripErr := rt.RoundTrip(req)
	require.Error(t, tripErr)
	assert.Contains(t, tripErr.Error(), "retry snapshot limit")
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
