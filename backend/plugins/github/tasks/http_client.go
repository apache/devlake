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

package tasks

import (
	"net/http"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/github/models"
	"github.com/apache/incubator-devlake/plugins/github/token"
)

func CreateAuthenticatedHttpClient(
	taskCtx plugin.TaskContext,
	connection *models.GithubConnection,
	baseClient *http.Client,
) (*http.Client, errors.Error) {

	logger := taskCtx.GetLogger()
	db := taskCtx.GetDal()
	encryptionSecret := taskCtx.GetConfig(plugin.EncodeKeyEnvStr)

	if baseClient == nil {
		baseClient = &http.Client{}
	}

	// Inject TokenProvider for OAuth refresh or GitHub App installation tokens.
	var tp *token.TokenProvider
	if connection.RefreshToken != "" {
		tp = token.NewTokenProvider(connection, db, baseClient, logger, encryptionSecret)
	} else if connection.AuthMethod == models.AppKey && connection.InstallationID != 0 {
		tp = token.NewAppInstallationTokenProvider(connection, db, baseClient, logger, encryptionSecret)
	}

	baseTransport := baseClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	if tp != nil {
		baseClient.Transport = token.NewRefreshRoundTripper(baseTransport, tp)
		logger.Info(
			"Installed token refresh round tripper for connection %d (authMethod=%s)",
			connection.ID,
			connection.AuthMethod,
		)

	} else if connection.Token != "" {
		baseClient.Transport = token.NewStaticRoundTripper(
			baseTransport,
			connection.Token,
		)
		logger.Info(
			"Installed static token round tripper for connection %d",
			connection.ID,
		)
	}

	// Persist the freshly minted token so the DB has a correctly encrypted value.
	// PrepareApiClient (called by NewApiClientFromConnection) mints the token
	// in-memory but does not persist it; without this, the DB may contain a stale
	// or corrupted token that breaks GET /connections.
	if connection.AuthMethod == models.AppKey && connection.Token != "" {
		if err := token.PersistEncryptedTokenColumns(db, connection, encryptionSecret, logger, false); err != nil {
			logger.Warn(err, "Failed to persist initial token for connection %d", connection.ID)
		} else {
			logger.Info("Persisted initial token for connection %d", connection.ID)
		}
	}

	return baseClient, nil
}
