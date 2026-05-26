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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/utils"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// TaigaConn holds the essential information to connect to the Taiga API
type TaigaConn struct {
	helper.RestConnection `mapstructure:",squash"`
	helper.BasicAuth      `mapstructure:",squash"`
	// Token is optional - can be provided directly or obtained via username/password auth
	Token string `mapstructure:"token" json:"token" gorm:"serializer:encdec"`
}

func (tc *TaigaConn) Sanitize() TaigaConn {
	tc.Password = ""
	tc.Token = utils.SanitizeString(tc.Token)
	return *tc
}

// SetupAuthentication sets up the HTTP request with authentication.
// If Token is set directly, use it. Otherwise exchange Username+Password for a token.
func (tc *TaigaConn) SetupAuthentication(req *http.Request) errors.Error {
	if tc.Token != "" {
		req.Header.Set("Authorization", "Bearer "+tc.Token)
		return nil
	}
	if tc.Username != "" && tc.Password != "" {
		token, err := tc.fetchToken()
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

// fetchToken exchanges username+password for a Taiga auth token via POST /auth
func (tc *TaigaConn) fetchToken() (string, errors.Error) {
	endpoint := strings.TrimSuffix(tc.Endpoint, "/")
	// strip /api/v1 suffix to get base, then re-add /api/v1/auth
	authURL := endpoint + "/auth"

	body, e := json.Marshal(map[string]string{
		"type":     "normal",
		"username": tc.Username,
		"password": tc.Password,
	})
	if e != nil {
		return "", errors.Default.WrapRaw(e)
	}

	resp, e := http.Post(authURL, "application/json", bytes.NewReader(body)) //nolint:noctx
	if e != nil {
		return "", errors.Default.WrapRaw(e)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.Default.New(fmt.Sprintf("taiga auth failed with status %d", resp.StatusCode))
	}

	var result map[string]interface{}
	if e := json.NewDecoder(resp.Body).Decode(&result); e != nil {
		return "", errors.Default.WrapRaw(e)
	}
	// Taiga returns auth_token (v5) or token (v6)
	for _, key := range []string{"auth_token", "token"} {
		if t, ok := result[key]; ok {
			if token, ok := t.(string); ok && token != "" {
				return token, nil
			}
		}
	}
	// fallback: read raw body hint
	raw, _ := io.ReadAll(bytes.NewReader(body))
	return "", errors.Default.New(fmt.Sprintf("taiga auth response missing token field, body: %s", string(raw)))
}

// TaigaConnection holds TaigaConn plus ID/Name for database storage
type TaigaConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	TaigaConn             `mapstructure:",squash"`
}

func (TaigaConnection) TableName() string {
	return "_tool_taiga_connections"
}

func (connection *TaigaConnection) MergeFromRequest(target *TaigaConnection, body map[string]interface{}) error {
	token := target.Token
	password := target.Password

	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}

	modifiedToken := target.Token
	modifiedPassword := target.Password

	// preserve existing token if not modified
	if modifiedToken == "" || modifiedToken == utils.SanitizeString(token) {
		target.Token = token
	}

	// preserve existing password if not modified
	if modifiedPassword == "" || modifiedPassword == password {
		target.Password = password
	}

	return nil
}

func (connection TaigaConnection) Sanitize() TaigaConnection {
	connection.TaigaConn = connection.TaigaConn.Sanitize()
	return connection
}
