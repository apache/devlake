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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

const (
	DefaultRefreshBuffer = 5 * time.Minute
)

type TokenProvider struct {
	conn             *models.GithubConnection
	dal              dal.Dal
	encryptionSecret string
	httpClient       *http.Client
	baseTransport    http.RoundTripper // original transport, before RefreshRoundTripper wrapping
	logger           log.Logger
	mu               sync.Mutex
	refreshURL       string
	refreshFn        func(*TokenProvider) errors.Error
}

// NewTokenProvider creates a TokenProvider for the given GitHub connection using
// the provided DAL, HTTP client, and logger, and returns a pointer to it.
func NewTokenProvider(conn *models.GithubConnection, d dal.Dal, client *http.Client, logger log.Logger, encryptionSecret string) *TokenProvider {
	return &TokenProvider{
		conn:             conn,
		dal:              d,
		encryptionSecret: encryptionSecret,
		httpClient:       client,
		logger:           logger,
		refreshURL:       "https://github.com/login/oauth/access_token",
	}
}

// NewAppInstallationTokenProvider creates a TokenProvider that refreshes GitHub App installation tokens.
// IMPORTANT: Call this BEFORE wrapping the client's transport with RefreshRoundTripper,
// so that baseTransport captures the unwrapped transport and refresh calls don't deadlock.
func NewAppInstallationTokenProvider(conn *models.GithubConnection, d dal.Dal, client *http.Client, logger log.Logger, encryptionSecret string) *TokenProvider {
	if logger != nil {
		expiresStr := "unknown"
		if conn.TokenExpiresAt != nil {
			expiresStr = conn.TokenExpiresAt.Format(time.RFC3339)
		}
		logger.Info("Created AppInstallation token provider for connection %d (installation %d, token expires at %s)",
			conn.ID, conn.InstallationID, expiresStr)
	}
	// Capture the transport now, before the caller wraps it with RefreshRoundTripper.
	// This avoids a deadlock: refresh calls must bypass the RefreshRoundTripper that
	// holds the TokenProvider mutex during GetToken().
	baseTransport := client.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	return &TokenProvider{
		conn:             conn,
		dal:              d,
		encryptionSecret: encryptionSecret,
		httpClient:       client,
		baseTransport:    baseTransport,
		logger:           logger,
		refreshFn:        refreshGitHubAppInstallationToken,
	}
}

func (tp *TokenProvider) GetToken() (string, errors.Error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.needsRefresh() {
		if tp.logger != nil {
			expiresStr := "unknown"
			if tp.conn.TokenExpiresAt != nil {
				expiresStr = tp.conn.TokenExpiresAt.Format(time.RFC3339)
			}
			tp.logger.Info("Proactive token refresh triggered for connection %d (token expires at %s)",
				tp.conn.ID, expiresStr)
		}
		if err := tp.refreshToken(); err != nil {
			return "", err
		}
	}
	return tp.conn.Token, nil
}

func (tp *TokenProvider) needsRefresh() bool {
	buffer := DefaultRefreshBuffer
	if envBuffer := os.Getenv("GITHUB_TOKEN_REFRESH_BUFFER_MINUTES"); envBuffer != "" {
		if val, err := strconv.Atoi(envBuffer); err == nil {
			buffer = time.Duration(val) * time.Minute
		}
	}

	if tp.refreshFn != nil {
		if tp.conn.TokenExpiresAt == nil {
			return false
		}
		return time.Now().Add(buffer).After(*tp.conn.TokenExpiresAt)
	}

	if tp.conn.RefreshToken == "" {
		return false
	}
	if tp.conn.TokenExpiresAt == nil {
		return false
	}
	return time.Now().Add(buffer).After(*tp.conn.TokenExpiresAt)
}

func (tp *TokenProvider) refreshToken() errors.Error {
	if tp.refreshFn != nil {
		return tp.refreshFn(tp)
	}
	tp.logger.Info("Refreshing GitHub token for connection %d", tp.conn.ID)

	data := map[string]string{
		"refresh_token": tp.conn.RefreshToken,
		"grant_type":    "refresh_token",
		"client_id":     tp.conn.AppId,
		"client_secret": tp.conn.SecretKey,
	}
	jsonData, _ := json.Marshal(data)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", tp.refreshURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Convert(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := tp.httpClient.Do(req)
	if err != nil {
		return errors.Convert(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Convert(err)
	}

	if resp.StatusCode != http.StatusOK {
		// Log the response body to aid in debugging token refresh failures.
		if tp.logger != nil {
			tp.logger.Error(nil, "failed to refresh token from GitHub, status=%d, body=%s", resp.StatusCode, string(body))
		}
		return errors.Default.New(fmt.Sprintf("failed to refresh token: %d, body: %s", resp.StatusCode, string(body)))
	}
	var result struct {
		AccessToken           string `json:"access_token"`
		RefreshToken          string `json:"refresh_token"`
		ExpiresIn             int    `json:"expires_in"`
		RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return errors.Convert(err)
	}

	if result.AccessToken == "" {
		bodyStr := string(body)
		const maxBodySnippet = 512
		if len(bodyStr) > maxBodySnippet {
			bodyStr = bodyStr[:maxBodySnippet] + "…"
		}
		return errors.Default.New(fmt.Sprintf("empty access token returned; response body: %s", bodyStr))
	}

	tokenExpiredAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	refreshTokenExpiredAt := time.Now().Add(time.Duration(result.RefreshTokenExpiresIn) * time.Second)

	tp.conn.UpdateToken(
		result.AccessToken,
		result.RefreshToken,
		&tokenExpiredAt,
		&refreshTokenExpiredAt,
	)

	if tp.dal != nil {
		// Manually encrypt and use UpdateColumns to persist only the token-related
		// columns. We cannot use dal.Update (GORM Save) because it writes ALL fields
		// including refresh_token_expires_at which may have Go zero time that MySQL
		// rejects. We cannot use UpdateColumns with plaintext because it bypasses the
		// GORM encdec serializer. So we encrypt manually and write the ciphertext.
		if err := PersistEncryptedTokenColumns(tp.dal, tp.conn, tp.encryptionSecret, tp.logger, true); err != nil {
			tp.logger.Warn(err, "failed to persist refreshed token")
		}
	}

	return nil
}

// ForceRefresh refreshes the access token if the current token is still equal to oldToken.
// The oldToken parameter should be the token value observed by the caller when it determined
// that a refresh might be needed; if the token has changed since then, another goroutine has
// already refreshed it and this method returns without performing a redundant refresh.
func (tp *TokenProvider) ForceRefresh(oldToken string) errors.Error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// If the token has changed since the request was made, it means another thread
	// has already refreshed it.
	if tp.conn.Token != oldToken {
		if tp.logger != nil {
			tp.logger.Info("Skipping reactive token refresh for connection %d — token already changed by another goroutine", tp.conn.ID)
		}
		return nil
	}

	if tp.logger != nil {
		tp.logger.Info("Reactive token refresh triggered for connection %d (received 401)", tp.conn.ID)
	}
	return tp.refreshToken()
}
