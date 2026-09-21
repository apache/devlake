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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

const (
	// atlassianOAuthEndpoint is the public Atlassian OAuth 2.0 client-credentials endpoint.
	atlassianOAuthEndpoint = "https://auth.atlassian.com/oauth/token"
	defaultTokenTimeout    = 10 * time.Second
	defaultTokenTTL        = 3600 * time.Second
	// Atlassian access tokens are JWTs; with many scopes the JSON easily
	// exceeds 4KiB. Keep a cap as a DoS guard, not a typical-token size.
	maxOAuthResponseBytes = 64 * 1024
)

// NewOAuthHTTPClient returns an HTTP client for Atlassian token requests.
// It must not share a transport with the Jira API client (which may wrap a refresh round tripper).
func NewOAuthHTTPClient(proxy string) (*http.Client, errors.Error) {
	transport := &http.Transport{}
	if proxy != "" {
		pu, err := url.Parse(proxy)
		if err != nil {
			return nil, errors.Convert(err)
		}
		transport.Proxy = http.ProxyURL(pu)
	}
	return &http.Client{
		Timeout:   defaultTokenTimeout,
		Transport: transport,
	}, nil
}

func (jc *JiraConn) tokenURL() string {
	if jc.OAuthTokenURL != "" {
		return jc.OAuthTokenURL
	}
	return atlassianOAuthEndpoint
}

// MintOAuthAccessToken exchanges client_id/client_secret for a Bearer access token.
func (jc *JiraConn) MintOAuthAccessToken(httpClient *http.Client) (string, time.Time, errors.Error) {
	if httpClient == nil {
		return "", time.Time{}, errors.Default.New("oauth http client is required")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", jc.ClientId)
	form.Set("client_secret", jc.ClientSecret)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTokenTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, jc.tokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, errors.Convert(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, errors.Default.Wrap(err, "failed to request oauth2 access token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return "", time.Time{}, errors.Convert(err)
	}
	if len(body) > maxOAuthResponseBytes {
		return "", time.Time{}, errors.Default.New("oauth2 token response exceeded size limit")
	}
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, errors.Default.New(fmt.Sprintf("failed to mint oauth2 access token: %d: %s", resp.StatusCode, oauthErrorDetail(body)))
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return "", time.Time{}, errors.Default.New("oauth2 token endpoint returned an empty body")
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", time.Time{}, errors.Default.Wrap(err, fmt.Sprintf("decoding oauth2 token response (%d bytes)", len(body)))
	}
	if result.AccessToken == "" {
		return "", time.Time{}, errors.Default.New("empty oauth2 access token returned")
	}

	ttl := time.Duration(result.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}
	return result.AccessToken, time.Now().Add(ttl), nil
}

// PrepareApiClient mints an OAuth 2.0 access token so subsequent requests can use Bearer auth.
func (jc *JiraConn) PrepareApiClient(_ plugin.ApiClient) errors.Error {
	if !jc.IsOAuth2() {
		return nil
	}
	jc.ApplyGatewayEndpoint()
	if jc.OAuthAccessToken() != "" {
		return nil
	}
	httpClient, err := NewOAuthHTTPClient(jc.GetProxy())
	if err != nil {
		return err
	}
	token, expiresAt, err := jc.MintOAuthAccessToken(httpClient)
	if err != nil {
		return err
	}
	jc.SetOAuthAccessToken(token, expiresAt)
	return nil
}

func oauthErrorDetail(body []byte) string {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &payload) == nil {
		switch {
		case payload.Error != "" && payload.ErrorDescription != "":
			return payload.Error + ": " + payload.ErrorDescription
		case payload.Error != "":
			return payload.Error
		case payload.ErrorDescription != "":
			return payload.ErrorDescription
		}
	}
	return "unexpected token endpoint response"
}
