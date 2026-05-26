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

// testRSAKey is a throwaway 2048-bit RSA private key used only in tests.
// It is NOT a real credential.
const testRSAKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA1xJuX407giVTO/FY2pbp6bdB/XxaiPqAuvWIcEqabzq+d3ft
O7fGtbXSQrCdtxEQt5dHFKdJofHcGlKPDnq1BNjWM3/xLWsQWQPSwUZ9H1qy/nDI
GX+ciXmP8hbzoe5B1OXidAdrJUGWH3ox8Yp8OVd/yK9p34teCbzPnqVEc9tkgUT1
94gHKLmvP28VefybFyGbYB3ujVIuA8Z26c4gQsyFzR2v1fVeDIu1e1afyH5WTFgr
EWaztOo6pI5stzH00U7fNMzULBuYQ+ufQ4iQ7Ewt7fK5yyQNkx3pX0o1OQ/aYxQr
hyBrHakxHBe78eq8rSR2KwSr5nDuYzIAOUhtnwIDAQABAoH/EknhZ6EkvRPR7G5e
bq4/NduuSriDcITwbWuuuFPufzgdIZVzlu8xHfPOT9152qE2aD7XahGUk85fwbXh
EeP64x/cA3FnmrAxPYfuUFnB0+i7KHGpzW2Wa/LqXlz3vS/UQePoDwl+xDYeUt2u
xALHoSxZdlTWv6HLhLmw7ge9QJLc/xQO9dC678c4Y4JTCRrEvhE+eZiDEXb6HL2D
uNMEwFqMLTxOurYKXE+iyzKg0e4D6oDkw6BC+vOUBnuH+9iKV6wnal24E1WnyhCa
vy7iHBoc8npeKYAHjU5wKDQiWPMT9DjYBRucHvSyEYTeS5eyQHed42PGv+Ss2IL4
w0JZAoGBAPjxiEJwck29NyfElw5+/P0IKIE8BkuIOSC8JoGfo+JpKYWFIXaLRbK8
qvOiLDotHgC5IxFZyN8pejVe7Zvif5PepR9qMcV3e6Uz2nmJqtp5gFlzjNUSzbeM
J6gqkejtwn7wJY3dwZhfbTTDDY7cZ9f3Cydarx+iu07unGCRT9DDAoGBAN0rG/F9
tsOSwrgsnPwNeVPfJn2Zw6mvb1xuOoJl11tDnpS6tH50FBJ/TkERZak9+VZ761pq
CvuMjLtePgC31rsAcUEEt1OwYCxne8gNxuzl2mWKD98ActbFf96VM4Q68Jvntxl1
eOMMnSA+/yjvhpxdVabm3180mRu9eBv6SaH1AoGBAMpwL6xHoMwS6L1QIr7JCZYC
gl3FoCDgIAS8vFuApFbDyd4oSvQJgZ49yo7g/DI66kEQTLIZXz4KjrTEA1lWsQRg
c8q+Isc/yK6pIirfhq6vS25yhr3m0p9GPCGGrKzMW/O5+fAJuxrbzwSu8WGRXmjD
HrDcD7kcLlGbvFLTGCLdAoGBALNID7W5Z16v6AItv++d6Hzxhh0IeRBi8s2lWO59
KY6EiNcdZdSfuemoosGiHZuMbkMJ3qWDEnYI38e+xFoGrB0YZbYD4awIbF1yYWew
q1E7ncbznJvznCO3I0lF/uWwdXyb39PWYvECN5h9GI+RYrf7/MN3oRhm5boT43oi
cG/FAoGBALD1zSG9qrysLg8fw4dc4cs6dAHZhAszh47zw2WiMmUVWQZCbB+uhs44
qGCAaer5KdnTqy9NdwW6rlcr4Y8jUFnMlMFA5HdmHkIfgTi+zi4Qf1mb9yzpJqnU
jKflsh1Lyyqv2KsoIMz4vjew+lCVn80FZaEEQ1q9tAkyu5m53w65
-----END RSA PRIVATE KEY-----`

func TestRoundTripper401Refresh(t *testing.T) {
	mockRT := new(MockRoundTripper)
	client := &http.Client{Transport: mockRT}

	expiry := time.Now().Add(10 * time.Minute) // Not expired
	conn := &models.GithubConnection{
		GithubConn: models.GithubConn{
			RefreshToken: "refresh_token",
			GithubAccessToken: models.GithubAccessToken{
				AccessToken: api.AccessToken{
					Token: "old_token",
				},
			},
			TokenExpiresAt: &expiry,
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
	rt := NewRefreshRoundTripper(mockRT, tp)

	// Request
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)

	// 1. First call returns 401
	resp401 := &http.Response{
		StatusCode: 401,
		Body:       io.NopCloser(bytes.NewBufferString("Unauthorized")),
	}
	mockRT.On("RoundTrip", mock.MatchedBy(func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer old_token" && r.URL.String() == "https://api.github.com/user"
	})).Return(resp401, nil).Once()

	// 2. Refresh call (triggered by 401)
	respRefresh := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"access_token":"new_token","refresh_token":"new_refresh_token","expires_in":3600,"refresh_token_expires_in":3600}`)),
	}
	// The refresh call uses the same client, so it goes through mockRT too!
	mockRT.On("RoundTrip", mock.MatchedBy(func(r *http.Request) bool {
		return r.URL.String() == "https://github.com/login/oauth/access_token"
	})).Return(respRefresh, nil).Once()

	// 3. Retry call with new token
	resp200 := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString("Success")),
	}
	mockRT.On("RoundTrip", mock.MatchedBy(func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer new_token" && r.URL.String() == "https://api.github.com/user"
	})).Return(resp200, nil).Once()

	// Execute
	resp, err := rt.RoundTrip(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "Success", string(body))

	mockRT.AssertExpectations(t)
}

func TestRoundTripper401WithAppKeyRefresh(t *testing.T) {
	mockRT := new(MockRoundTripper)

	expiry := time.Now().Add(10 * time.Minute) // Not expired (proactive refresh won't trigger)
	conn := &models.GithubConnection{
		GithubConn: models.GithubConn{
			GithubAccessToken: models.GithubAccessToken{
				AccessToken: api.AccessToken{
					Token: "old_app_token",
				},
			},
			TokenExpiresAt: &expiry,
		},
	}
	// Use tokens slice so GetToken returns the current token
	conn.UpdateToken("old_app_token", "", &expiry, nil)

	// refreshFn simulates minting a new installation token
	refreshCalled := 0
	tp := &TokenProvider{
		conn: conn,
		refreshFn: func(tp *TokenProvider) errors.Error {
			refreshCalled++
			newExpiry := time.Now().Add(1 * time.Hour)
			tp.conn.UpdateToken("new_app_token", "", &newExpiry, nil)
			return nil
		},
	}

	rt := NewRefreshRoundTripper(mockRT, tp)

	req, _ := http.NewRequest("GET", "https://api.github.com/repos/test/test", nil)

	// 1. First call returns 401
	resp401 := &http.Response{
		StatusCode: 401,
		Body:       io.NopCloser(bytes.NewBufferString("Bad credentials")),
	}
	mockRT.On("RoundTrip", mock.MatchedBy(func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer old_app_token"
	})).Return(resp401, nil).Once()

	// 2. Retry call with new token (after refreshFn runs)
	resp200 := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"full_name":"test/test"}`)),
	}
	mockRT.On("RoundTrip", mock.MatchedBy(func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer new_app_token"
	})).Return(resp200, nil).Once()

	// Execute
	resp, err := rt.RoundTrip(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, refreshCalled, "refreshFn should have been called exactly once")

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, `{"full_name":"test/test"}`, string(body))

	mockRT.AssertExpectations(t)
}

// TestProactiveRefreshNoDeadlock verifies that when the RefreshRoundTripper wraps
// the same http.Client's transport, a proactive token refresh does not deadlock.
// This reproduces the real-world scenario: GetToken() holds the mutex, calls
// refreshGitHubAppInstallationToken, which makes an HTTP request. If that request
// goes through the RefreshRoundTripper (re-entering GetToken), it would deadlock.
// The fix is that the refresh uses baseTransport directly.
func TestProactiveRefreshNoDeadlock(t *testing.T) {
	// Set up a mock transport that will serve both:
	// 1. The installation token refresh POST
	// 2. The actual API GET after refresh
	mockRT := new(MockRoundTripper)
	client := &http.Client{Transport: mockRT}

	// Token is expired — proactive refresh WILL trigger on GetToken()
	expired := time.Now().Add(-1 * time.Minute)
	conn := &models.GithubConnection{
		GithubConn: models.GithubConn{
			RestConnection: api.RestConnection{
				Endpoint: "https://api.github.com/",
			},
			MultiAuth: api.MultiAuth{
				AuthMethod: models.AppKey,
			},
			GithubAccessToken: models.GithubAccessToken{
				AccessToken: api.AccessToken{
					Token: "expired_ghs_token",
				},
			},
			GithubAppKey: models.GithubAppKey{
				AppKey: api.AppKey{
					AppId:     "12345",
					SecretKey: testRSAKey,
				},
				InstallationID: 99999,
			},
			TokenExpiresAt: &expired,
		},
	}
	conn.UpdateToken("expired_ghs_token", "", &expired, nil)

	// Create the TokenProvider with baseTransport = mockRT (the unwrapped transport).
	// This is what NewAppInstallationTokenProvider does: it captures client.Transport
	// BEFORE the caller wraps it with RefreshRoundTripper.
	tp := NewAppInstallationTokenProvider(conn, nil, client, nil, "")

	// Now wrap the client's transport with RefreshRoundTripper (simulating what
	// CreateApiClient does). After this, client.Transport = RefreshRoundTripper,
	// but tp.baseTransport still points to mockRT.
	rt := NewRefreshRoundTripper(mockRT, tp)
	client.Transport = rt

	// Mock: installation token refresh POST (goes through baseTransport, not RT)
	newExpiry := time.Now().Add(1 * time.Hour)
	installTokenBody := `{"token":"new_ghs_token","expires_at":"` + newExpiry.Format(time.RFC3339) + `"}`
	mockRT.On("RoundTrip", mock.MatchedBy(func(r *http.Request) bool {
		return r.Method == "POST" && r.URL.Path == "/app/installations/99999/access_tokens"
	})).Return(&http.Response{
		StatusCode: 201,
		Body:       io.NopCloser(bytes.NewBufferString(installTokenBody)),
	}, nil).Once()

	// Mock: the actual API request (goes through RefreshRoundTripper → GetToken → mockRT)
	mockRT.On("RoundTrip", mock.MatchedBy(func(r *http.Request) bool {
		return r.Method == "GET" && r.URL.Path == "/repos/test/test"
	})).Return(&http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"full_name":"test/test"}`)),
	}, nil).Once()

	// Execute through the RefreshRoundTripper with a timeout to detect deadlocks.
	done := make(chan struct{})
	var resp *http.Response
	var reqErr error
	go func() {
		req, _ := http.NewRequest("GET", "https://api.github.com/repos/test/test", nil)
		resp, reqErr = rt.RoundTrip(req)
		close(done)
	}()

	select {
	case <-done:
		// Success — no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("DEADLOCK: RoundTrip did not complete within 5 seconds — " +
			"the refresh call is likely going through RefreshRoundTripper instead of baseTransport")
	}

	assert.NoError(t, reqErr)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "new_ghs_token", conn.Token, "token should have been refreshed")

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, `{"full_name":"test/test"}`, string(body))

	mockRT.AssertExpectations(t)
}
