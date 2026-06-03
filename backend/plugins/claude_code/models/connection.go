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
	"net/http"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/utils"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const (
	// DefaultEndpoint is the Anthropic API base URL.
	DefaultEndpoint = "https://api.anthropic.com"
	// DefaultRateLimitPerHour is a conservative default for Admin API calls.
	DefaultRateLimitPerHour = 1000
	// AnthropicVersion is the required API version header value.
	AnthropicVersion = "2023-06-01"
)

// CustomHeader represents a single HTTP header key-value pair for middleware authentication.
type CustomHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ClaudeCodeConn stores Anthropic Claude Code connection settings.
type ClaudeCodeConn struct {
	helper.RestConnection `mapstructure:",squash"`

	Token         string         `mapstructure:"token" json:"token"`
	Organization  string         `mapstructure:"organization" json:"organization" gorm:"type:varchar(255)"`
	CustomHeaders []CustomHeader `mapstructure:"customHeaders" json:"customHeaders" gorm:"type:json;serializer:json"`
}

func (conn *ClaudeCodeConn) HasUsableCustomHeaders() bool {
	if conn == nil {
		return false
	}
	for _, header := range conn.CustomHeaders {
		if strings.TrimSpace(header.Key) != "" && strings.TrimSpace(header.Value) != "" {
			return true
		}
	}
	return false
}

func (conn *ClaudeCodeConn) HasIncompleteCustomHeaders() bool {
	if conn == nil {
		return false
	}
	for _, header := range conn.CustomHeaders {
		hasKey := strings.TrimSpace(header.Key) != ""
		hasValue := strings.TrimSpace(header.Value) != ""
		if hasKey != hasValue {
			return true
		}
	}
	return false
}

// SetupAuthentication implements plugin.ApiAuthenticator so helper.NewApiClientFromConnection
// can attach the required headers for Anthropic Admin API requests.
//
// When Token is set, the standard Anthropic auth headers (x-api-key, anthropic-version) are
// added. When connecting through a middleware, leave Token empty and supply CustomHeaders instead.
func (conn *ClaudeCodeConn) SetupAuthentication(request *http.Request) errors.Error {
	if conn == nil {
		return errors.BadInput.New("connection is required")
	}

	request.Header.Set("User-Agent", "DevLake/1.0.0")

	if key := strings.TrimSpace(conn.Token); key != "" {
		request.Header.Set("x-api-key", key)
		request.Header.Set("anthropic-version", AnthropicVersion)
	}

	for _, h := range conn.CustomHeaders {
		if strings.TrimSpace(h.Key) != "" && strings.TrimSpace(h.Value) != "" {
			request.Header.Set(h.Key, h.Value)
		}
	}

	return nil
}

// Sanitize returns a copy of the conn with sensitive fields masked.
func (conn *ClaudeCodeConn) Sanitize() ClaudeCodeConn {
	if conn == nil {
		return ClaudeCodeConn{}
	}
	clone := *conn
	clone.Token = utils.SanitizeString(clone.Token)
	sanitizedHeaders := make([]CustomHeader, len(clone.CustomHeaders))
	for i, h := range clone.CustomHeaders {
		sanitizedHeaders[i] = CustomHeader{Key: h.Key, Value: utils.SanitizeString(h.Value)}
	}
	clone.CustomHeaders = sanitizedHeaders
	return clone
}

// ClaudeCodeConnection persists connection details with metadata required by DevLake.
type ClaudeCodeConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	ClaudeCodeConn        `mapstructure:",squash"`
}

func (ClaudeCodeConnection) TableName() string {
	return "_tool_claude_code_connections"
}

func (connection ClaudeCodeConnection) Sanitize() ClaudeCodeConnection {
	connection.ClaudeCodeConn = connection.ClaudeCodeConn.Sanitize()
	return connection
}

// Normalize applies default connection values where necessary.
func (connection *ClaudeCodeConnection) Normalize() {
	if connection == nil {
		return
	}
	if connection.Endpoint == "" {
		connection.Endpoint = DefaultEndpoint
	}
	if connection.RateLimitPerHour <= 0 {
		connection.RateLimitPerHour = DefaultRateLimitPerHour
	}
	normalizedHeaders := make([]CustomHeader, 0, len(connection.CustomHeaders))
	for _, header := range connection.CustomHeaders {
		key := strings.TrimSpace(header.Key)
		value := strings.TrimSpace(header.Value)
		if key == "" && value == "" {
			continue
		}
		normalizedHeaders = append(normalizedHeaders, CustomHeader{Key: key, Value: header.Value})
	}
	connection.CustomHeaders = normalizedHeaders
}

// MergeFromRequest applies a partial update from an HTTP PATCH body,
// preserving original secret values (Token and custom header values) when
// the caller sends back sanitized placeholders.
func (connection *ClaudeCodeConnection) MergeFromRequest(target *ClaudeCodeConnection, body map[string]interface{}) error {
	if target == nil {
		return nil
	}
	originalKey := target.Token
	originalHeaders := append([]CustomHeader(nil), target.CustomHeaders...)
	originalHeaderValues := make(map[string]string, len(originalHeaders))
	for _, h := range originalHeaders {
		originalHeaderValues[h.Key] = h.Value
	}

	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}

	sanitizedOriginal := utils.SanitizeString(originalKey)
	if target.Token == "" || target.Token == sanitizedOriginal {
		target.Token = originalKey
	}

	for i, h := range target.CustomHeaders {
		if orig, ok := originalHeaderValues[h.Key]; ok {
			if h.Value == "" || h.Value == utils.SanitizeString(orig) {
				target.CustomHeaders[i].Value = orig
				continue
			}
		}

		if i < len(originalHeaders) && h.Value != "" && h.Value == utils.SanitizeString(originalHeaders[i].Value) {
			target.CustomHeaders[i].Value = originalHeaders[i].Value
		}
	}

	return nil
}
