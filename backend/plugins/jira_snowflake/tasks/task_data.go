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
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"

	"github.com/apache/devlake/core/errors"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
	sf "github.com/snowflakedb/gosnowflake"
)

// JiraSnowflakeOptions contains all per-pipeline task options.
type JiraSnowflakeOptions struct {
	ConnectionId uint64 `json:"connectionId"  mapstructure:"connectionId"`
	BoardId      uint64 `json:"boardId"       mapstructure:"boardId"`
	// ProjectKeys lists all Jira project keys that belong to this board,
	// e.g. ["KONFLUX", "HELM"]. Required because a board may span multiple projects.
	ProjectKeys   []string                    `json:"projectKeys"   mapstructure:"projectKeys"`
	ScopeConfigId uint64                      `json:"scopeConfigId" mapstructure:"scopeConfigId"`
	ScopeConfig   *jiramodels.JiraScopeConfig `json:"scopeConfig"   mapstructure:"scopeConfig"`
}

// JiraSnowflakeTaskData is passed to every subtask via taskCtx.GetData().
type JiraSnowflakeTaskData struct {
	Options     *JiraSnowflakeOptions
	SnowflakeDB *sql.DB
}

// JiraApiParams mirrors jira/models.JiraApiParams so that RawDataSubTaskArgs
// produces the same _raw_data_params format for state management.
type JiraApiParams struct {
	ConnectionId uint64
	BoardId      uint64
}

// DecodeAndValidateTaskOptions decodes and validates options for the task.
func DecodeAndValidateTaskOptions(options map[string]interface{}) (*JiraSnowflakeOptions, errors.Error) {
	var op JiraSnowflakeOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, err
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New(fmt.Sprintf("invalid connectionId: %d", op.ConnectionId))
	}
	if op.BoardId == 0 {
		return nil, errors.BadInput.New(fmt.Sprintf("invalid boardId: %d", op.BoardId))
	}
	if len(op.ProjectKeys) == 0 {
		return nil, errors.BadInput.New("projectKeys must not be empty")
	}
	return &op, nil
}

// OpenSnowflakeDB opens a database/sql connection to Snowflake.
//
// authType controls authentication:
//   - "keypair" (default): JWT key-pair auth using privateKeyPEM. Works in containers and CI.
//   - "externalbrowser": SSO via browser pop-up. Only works when DevLake runs on a desktop host
//     (i.e. via `make run`, not inside a Docker container).
func OpenSnowflakeDB(account, user, authType, privateKeyPEM, database, schema, warehouse, role string) (*sql.DB, errors.Error) {
	cfg := &sf.Config{
		Account:  account,
		User:     user,
		Database: database,
		Schema:   schema,
	}
	if warehouse != "" {
		cfg.Warehouse = warehouse
	}
	if role != "" {
		cfg.Role = role
	}

	if authType == "externalbrowser" {
		cfg.Authenticator = sf.AuthTypeExternalBrowser
	} else {
		privKey, err := parseRSAPrivateKey(privateKeyPEM)
		if err != nil {
			return nil, errors.Default.Wrap(err, "failed to parse Snowflake private key")
		}
		cfg.Authenticator = sf.AuthTypeJwt
		cfg.PrivateKey = privKey
	}

	dsn, goErr := sf.DSN(cfg)
	if goErr != nil {
		return nil, errors.Default.Wrap(goErr, "failed to build Snowflake DSN")
	}
	db, goErr := sql.Open("snowflake", dsn)
	if goErr != nil {
		return nil, errors.Default.Wrap(goErr, "failed to open Snowflake connection")
	}
	return db, nil
}

// parseRSAPrivateKey parses a PKCS#8 PEM-encoded RSA private key.
func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not an RSA key")
	}
	return rsaKey, nil
}
