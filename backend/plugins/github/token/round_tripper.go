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
	"net/http"
)

// RefreshRoundTripper is an HTTP transport middleware that automatically manages OAuth token refreshes.
// It wraps an underlying http.RoundTripper and provides token refresh on auth failures.
// On 401's the round tripper will:
// - Force a refresh of the OAuth token via the TokenProvider
// - Retry the original request with the new token
//
// Note: When active, the RefreshRoundTripper overwrites the Authorization header
// on every request, superseding any header previously set by SetupAuthentication.
// This is intentional — for connections using token refresh (OAuth or GitHub App),
// the round tripper is the single source of truth for the current bearer token.
type RefreshRoundTripper struct {
	base          http.RoundTripper
	tokenProvider *TokenProvider
}

func NewRefreshRoundTripper(base http.RoundTripper, tp *TokenProvider) *RefreshRoundTripper {
	return &RefreshRoundTripper{
		base:          base,
		tokenProvider: tp,
	}
}

// RoundTrip implements the http.RoundTripper interface and handles automatic token refresh on 401 responses.
// It clones the request, adds the Authorization header, and retries once with a refreshed token if needed.
func (rt *RefreshRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.roundTripWithRetry(req, false)
}

// roundTripWithRetry performs the actual request with retry on 401.
// The refreshAttempted parameter tracks whether a refresh has already been tried for this request
// to prevent infinite retry loops if token refresh itself fails.
func (rt *RefreshRoundTripper) roundTripWithRetry(req *http.Request, refreshAttempted bool) (*http.Response, error) {
	// Get token
	token, err := rt.tokenProvider.GetToken()
	if err != nil {
		return nil, err
	}

	// Clone request before modifying
	reqClone := req.Clone(req.Context())
	reqClone.Header.Set("Authorization", "Bearer "+token)

	// Execute request
	resp, reqErr := rt.base.RoundTrip(reqClone)
	if reqErr != nil {
		return nil, reqErr
	}

	// Reactive refresh on 401
	if resp.StatusCode == http.StatusUnauthorized && !refreshAttempted {
		// Close previous response body
		resp.Body.Close()

		// Force refresh
		if err := rt.tokenProvider.ForceRefresh(token); err != nil {
			return nil, err
		}

		// Get new token
		newToken, err := rt.tokenProvider.GetToken()
		if err != nil {
			return nil, err
		}

		// Retry request with new token
		reqRetry := req.Clone(req.Context())
		reqRetry.Header.Set("Authorization", "Bearer "+newToken)
		return rt.roundTripWithRetry(reqRetry, true)
	}

	return resp, nil
}
