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
	"bytes"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/impls/logruslog"
	"github.com/apache/incubator-devlake/plugins/github/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRoundTripper struct {
	mock.Mock
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestNeedsRefresh(t *testing.T) {
	tp := &TokenProvider{
		conn: &models.GithubConnection{
			GithubConn: models.GithubConn{
				RefreshToken: "refresh_token",
			},
		},
	}

	// Not expired, outside buffer
	expiry1 := time.Now().Add(10 * time.Minute)
	tp.conn.TokenExpiresAt = &expiry1
	assert.False(t, tp.needsRefresh())

	// Inside buffer
	expiry2 := time.Now().Add(1 * time.Minute)
	tp.conn.TokenExpiresAt = &expiry2
	assert.True(t, tp.needsRefresh())

	// Expired
	expiry3 := time.Now().Add(-1 * time.Minute)
	tp.conn.TokenExpiresAt = &expiry3
	assert.True(t, tp.needsRefresh())

	// No refresh token
	tp.conn.RefreshToken = ""
	assert.False(t, tp.needsRefresh())
}

func TestTokenProviderConcurrency(t *testing.T) {
	mockRT := new(MockRoundTripper)
	client := &http.Client{Transport: mockRT}

	expired := time.Now().Add(-1 * time.Minute) // Expired
	conn := &models.GithubConnection{
		GithubConn: models.GithubConn{
			RefreshToken:   "refresh_token",
			TokenExpiresAt: &expired,
			GithubAppKey: models.GithubAppKey{
				AppKey: api.AppKey{
					AppId:     "123",
					SecretKey: "secret",
				},
			},
		},
	}

	logger, _ := logruslog.NewDefaultLogger(logrus.New())
	tp := NewTokenProvider(conn, nil, client, logger, "")

	// Mock response for refresh
	respBody := `{"access_token":"new_token","refresh_token":"new_refresh_token","expires_in":3600,"refresh_token_expires_in":3600}`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(respBody)),
	}

	// Expect exactly one call
	mockRT.On("RoundTrip", mock.Anything).Return(resp, nil).Once()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := tp.GetToken()
			assert.NoError(t, err)
			assert.Equal(t, "new_token", token)
		}()
	}
	wg.Wait()

	mockRT.AssertExpectations(t)
}

func TestConfigurableBuffer(t *testing.T) {
	os.Setenv("GITHUB_TOKEN_REFRESH_BUFFER_MINUTES", "10")
	defer os.Unsetenv("GITHUB_TOKEN_REFRESH_BUFFER_MINUTES")

	tp := &TokenProvider{
		conn: &models.GithubConnection{
			GithubConn: models.GithubConn{
				RefreshToken: "refresh_token",
			},
		},
	}

	// 9 minutes remaining (inside 10m buffer)
	expiry9 := time.Now().Add(9 * time.Minute)
	tp.conn.TokenExpiresAt = &expiry9
	assert.True(t, tp.needsRefresh())

	// 11 minutes remaining (outside 10m buffer)
	expiry11 := time.Now().Add(11 * time.Minute)
	tp.conn.TokenExpiresAt = &expiry11
	assert.False(t, tp.needsRefresh())
}

// fakeRefreshFn returns a refreshFn that updates the connection token to newToken
// and increments the call counter pointed to by count.
func fakeRefreshFn(newToken string, count *int) func(*TokenProvider) errors.Error {
	return func(tp *TokenProvider) errors.Error {
		*count++
		newExpiry := time.Now().Add(1 * time.Hour)
		tp.conn.UpdateToken(newToken, "", &newExpiry, nil)
		return nil
	}
}

func TestNeedsRefreshWithRefreshFn(t *testing.T) {
	callCount := 0
	tp := &TokenProvider{
		conn:      &models.GithubConnection{},
		refreshFn: fakeRefreshFn("unused", &callCount),
	}

	// Token not expired — outside default 5m buffer
	expiry1 := time.Now().Add(10 * time.Minute)
	tp.conn.TokenExpiresAt = &expiry1
	assert.False(t, tp.needsRefresh(), "should not refresh when token is 10m from expiry")

	// Token inside 5m buffer
	expiry2 := time.Now().Add(2 * time.Minute)
	tp.conn.TokenExpiresAt = &expiry2
	assert.True(t, tp.needsRefresh(), "should refresh when token is 2m from expiry")

	// Token already expired
	expiry3 := time.Now().Add(-1 * time.Minute)
	tp.conn.TokenExpiresAt = &expiry3
	assert.True(t, tp.needsRefresh(), "should refresh when token is expired")

	// TokenExpiresAt is nil — can't determine expiry, don't refresh (401 fallback covers this)
	tp.conn.TokenExpiresAt = nil
	assert.False(t, tp.needsRefresh(), "should not refresh when TokenExpiresAt is nil")

	// Provider with neither refreshFn nor RefreshToken — should never refresh
	tp2 := &TokenProvider{
		conn: &models.GithubConnection{},
	}
	expiry4 := time.Now().Add(-1 * time.Minute)
	tp2.conn.TokenExpiresAt = &expiry4
	assert.False(t, tp2.needsRefresh(), "should not refresh without refreshFn or RefreshToken")
}

func TestAppKeyGetTokenTriggersRefresh(t *testing.T) {
	callCount := 0
	expired := time.Now().Add(-1 * time.Minute)
	conn := &models.GithubConnection{
		GithubConn: models.GithubConn{
			GithubAccessToken: models.GithubAccessToken{
				AccessToken: api.AccessToken{
					Token: "expired_token",
				},
			},
			TokenExpiresAt: &expired,
		},
	}

	tp := &TokenProvider{
		conn:      conn,
		refreshFn: fakeRefreshFn("refreshed_app_token", &callCount),
	}

	token, err := tp.GetToken()
	assert.NoError(t, err)
	assert.Equal(t, "refreshed_app_token", token)
	assert.Equal(t, 1, callCount, "refreshFn should have been called exactly once")

	// Second call — token is now fresh, should not trigger refresh
	token2, err := tp.GetToken()
	assert.NoError(t, err)
	assert.Equal(t, "refreshed_app_token", token2)
	assert.Equal(t, 1, callCount, "refreshFn should not be called again for a fresh token")
}

func TestAppKeyForceRefresh(t *testing.T) {
	callCount := 0
	conn := &models.GithubConnection{
		GithubConn: models.GithubConn{
			GithubAccessToken: models.GithubAccessToken{
				AccessToken: api.AccessToken{
					Token: "old_app_token",
				},
			},
		},
	}

	tp := &TokenProvider{
		conn:      conn,
		refreshFn: fakeRefreshFn("new_app_token", &callCount),
	}

	// ForceRefresh with matching old token — should trigger refresh
	err := tp.ForceRefresh("old_app_token")
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, "new_app_token", conn.Token)

	// ForceRefresh with stale old token — token has already changed, should be a no-op
	err = tp.ForceRefresh("old_app_token")
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "should not refresh when token has already changed")
}
