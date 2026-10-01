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
	"fmt"
	"io"
	"net/http"
)

// maxRetryBodyBytes caps how much of a request body is buffered so a 401
// retry can re-send it. Collection is almost entirely GET; this is a DoS guard.
const maxRetryBodyBytes = int64(1 << 20)

// RefreshRoundTripper automatically remints OAuth 2.0 client-credentials tokens.
// On 401 the round tripper will:
// - Force a refresh of the access token via the TokenProvider
// - Retry the original request with the new token
//
// When active, the RefreshRoundTripper overwrites the Authorization header
// on every request, superseding any header previously set by SetupAuthentication.
type RefreshRoundTripper struct {
	base          http.RoundTripper
	tokenProvider *TokenProvider
}

// NewRefreshRoundTripper wraps base with a round tripper that automatically remints the OAuth 2.0 token on 401 responses.
func NewRefreshRoundTripper(base http.RoundTripper, tp *TokenProvider) *RefreshRoundTripper {
	return &RefreshRoundTripper{
		base:          base,
		tokenProvider: tp,
	}
}

// RoundTrip implements http.RoundTripper and remints the access token once on 401.
// It may consume and close req.Body (per the RoundTripper contract) so a retry can
// re-send the payload; it does not otherwise mutate req.
func (rt *RefreshRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	replay, err := snapshotRequestBody(req)
	if err != nil {
		return nil, err
	}
	return rt.roundTripWithRetry(req, replay, false)
}

func (rt *RefreshRoundTripper) roundTripWithRetry(req *http.Request, replay bodyReplay, refreshAttempted bool) (*http.Response, error) {
	token, err := rt.tokenProvider.GetToken()
	if err != nil {
		return nil, err
	}

	reqClone, cloneErr := cloneRequestWithBearer(req, token, replay)
	if cloneErr != nil {
		return nil, cloneErr
	}

	resp, reqErr := rt.base.RoundTrip(reqClone)
	if reqErr != nil {
		return nil, reqErr
	}

	if resp.StatusCode == http.StatusUnauthorized && !refreshAttempted {
		resp.Body.Close()

		if err := rt.tokenProvider.ForceRefresh(token); err != nil {
			return nil, err
		}

		return rt.roundTripWithRetry(req, replay, true)
	}

	return resp, nil
}

type bodyReplay struct {
	getBody func() (io.ReadCloser, error)
	length  int64
}

func cloneRequestWithBearer(req *http.Request, token string, replay bodyReplay) (*http.Request, error) {
	reqClone := req.Clone(req.Context())
	if replay.getBody != nil {
		body, err := replay.getBody()
		if err != nil {
			return nil, err
		}
		reqClone.Body = body
		reqClone.GetBody = replay.getBody
		if replay.length >= 0 {
			reqClone.ContentLength = replay.length
		}
	}
	reqClone.Header.Set("Authorization", "Bearer "+token)
	return reqClone, nil
}

// snapshotRequestBody returns a replayable body for clones. When GetBody is
// already set, the original request is left untouched. Otherwise the original
// Body is consumed and closed (allowed by http.RoundTripper) and a GetBody is
// synthesized for clones only.
func snapshotRequestBody(req *http.Request) (bodyReplay, error) {
	if req.GetBody != nil {
		return bodyReplay{getBody: req.GetBody, length: req.ContentLength}, nil
	}
	if req.Body == nil || req.Body == http.NoBody {
		return bodyReplay{}, nil
	}
	buf, err := io.ReadAll(io.LimitReader(req.Body, maxRetryBodyBytes+1))
	_ = req.Body.Close()
	if err != nil {
		return bodyReplay{}, err
	}
	if int64(len(buf)) > maxRetryBodyBytes {
		return bodyReplay{}, fmt.Errorf("request body exceeded %d-byte retry snapshot limit", maxRetryBodyBytes)
	}
	getBody := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf)), nil
	}
	return bodyReplay{getBody: getBody, length: int64(len(buf))}, nil
}
