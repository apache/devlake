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
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

// tokenPrefix returns the first n characters of a token for safe logging.
func tokenPrefix(token string, n int) string {
	if len(token) <= n {
		return token
	}
	return token[:n] + "..."
}

func refreshGitHubAppInstallationToken(tp *TokenProvider) errors.Error {
	if tp == nil || tp.conn == nil {
		return errors.Default.New("missing github connection for app token refresh")
	}
	if tp.conn.AuthMethod != models.AppKey || tp.conn.InstallationID == 0 {
		return errors.Default.New("invalid github app connection for token refresh")
	}
	if tp.conn.Endpoint == "" {
		return errors.Default.New("missing github endpoint for token refresh")
	}

	oldToken := tp.conn.Token
	if tp.logger != nil {
		expiresStr := "unknown"
		if tp.conn.TokenExpiresAt != nil {
			expiresStr = tp.conn.TokenExpiresAt.Format(time.RFC3339)
		}
		tp.logger.Info(
			"Refreshing GitHub App installation token for connection %d (installation %d), old token=%s, expires_at=%s",
			tp.conn.ID, tp.conn.InstallationID,
			tokenPrefix(oldToken, 8),
			expiresStr,
		)
	}

	// Use baseTransport (the unwrapped transport) to avoid deadlock.
	// The httpClient's transport may be the RefreshRoundTripper which would
	// re-enter GetToken() and deadlock on the mutex.
	apiClient := newRefreshApiClientWithTransport(tp.conn.Endpoint, tp.baseTransport)
	installationToken, err := tp.conn.GithubAppKey.GetInstallationAccessToken(apiClient)
	if err != nil {
		if tp.logger != nil {
			tp.logger.Error(err, "Failed to refresh GitHub App installation token for connection %d", tp.conn.ID)
		}
		return err
	}

	var expiresAt *time.Time
	if !installationToken.ExpiresAt.IsZero() {
		expiresAt = &installationToken.ExpiresAt
	}
	tp.conn.UpdateToken(installationToken.Token, "", expiresAt, nil)

	if tp.logger != nil {
		tp.logger.Info(
			"Successfully refreshed GitHub App installation token for connection %d, new token=%s, new expires_at=%s",
			tp.conn.ID,
			tokenPrefix(installationToken.Token, 8),
			installationToken.ExpiresAt.Format(time.RFC3339),
		)
	}

	persistAppToken(tp.dal, tp.conn, tp.encryptionSecret, tp.logger)
	return nil
}

func persistAppToken(d dal.Dal, conn *models.GithubConnection, encryptionSecret string, logger log.Logger) {
	if d == nil || conn == nil {
		return
	}
	if err := PersistEncryptedTokenColumns(d, conn, encryptionSecret, logger, false); err != nil {
		if logger != nil {
			logger.Warn(err, "Failed to persist refreshed app installation token for connection %d", conn.ID)
		}
	} else if logger != nil {
		logger.Info("Persisted refreshed app installation token for connection %d", conn.ID)
	}
}

// PersistEncryptedTokenColumns manually encrypts token fields and writes them
// to the DB using UpdateColumns (map-based), which only touches the specified
// columns. This avoids two problems:
//   - dal.Update (GORM Save) writes ALL fields, including refresh_token_expires_at
//     which may have Go zero time that MySQL rejects as '0000-00-00'.
//   - dal.UpdateColumns with plaintext bypasses the GORM encdec serializer,
//     writing unencrypted tokens that corrupt subsequent reads.
//
// IMPORTANT: We pass the table name string (not the conn struct) to UpdateColumns
// so that GORM uses Table() instead of Model(). When Model(conn) is used, GORM
// processes the encdec serializer on the struct's Token field during statement
// preparation, which overwrites conn.Token in memory with the encrypted ciphertext.
// This corrupts the in-memory token causing immediate 401s on the next API call.
//
// If includeRefreshToken is true, refresh_token and refresh_token_expires_at
// are also written (used by the OAuth refresh path where these values are valid).
func PersistEncryptedTokenColumns(d dal.Dal, conn *models.GithubConnection, encryptionSecret string, logger log.Logger, includeRefreshToken bool) errors.Error {
	encToken, err := plugin.Encrypt(encryptionSecret, conn.Token)
	if err != nil {
		return errors.Default.Wrap(err, "failed to encrypt token for persistence")
	}

	sets := []dal.DalSet{
		{ColumnName: "token", Value: encToken},
		{ColumnName: "token_expires_at", Value: conn.TokenExpiresAt},
	}

	if includeRefreshToken {
		encRefreshToken, err := plugin.Encrypt(encryptionSecret, conn.RefreshToken)
		if err != nil {
			return errors.Default.Wrap(err, "failed to encrypt refresh_token for persistence")
		}
		sets = append(sets,
			dal.DalSet{ColumnName: "refresh_token", Value: encRefreshToken},
			dal.DalSet{ColumnName: "refresh_token_expires_at", Value: conn.RefreshTokenExpiresAt},
		)
	}

	// Use the table name string instead of the conn struct to prevent GORM from
	// running the encdec serializer on conn.Token during Model() processing.
	return d.UpdateColumns(
		conn.TableName(),
		sets,
		dal.Where("id = ?", conn.ID),
	)
}
