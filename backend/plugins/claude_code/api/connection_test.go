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

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apache/incubator-devlake/plugins/claude_code/models"
)

const (
	testOrganization = "anthropic-labs"
	testToken        = "sk-ant-example"
)

func TestValidateConnectionSuccess(t *testing.T) {
	connection := &models.ClaudeCodeConnection{
		ClaudeCodeConn: models.ClaudeCodeConn{
			Organization: testOrganization,
			Token:        testToken,
		},
	}
	connection.Normalize()

	err := validateConnection(connection)
	assert.NoError(t, err)
}

func TestValidateConnectionNil(t *testing.T) {
	err := validateConnection(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection is required")
}

func TestValidateConnectionMissingOrganization(t *testing.T) {
	connection := &models.ClaudeCodeConnection{
		ClaudeCodeConn: models.ClaudeCodeConn{
			Token: testToken,
		},
	}

	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization is required")
}

func TestValidateConnectionMissingToken(t *testing.T) {
	connection := &models.ClaudeCodeConnection{
		ClaudeCodeConn: models.ClaudeCodeConn{
			Organization: testOrganization,
		},
	}

	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either token or at least one custom header is required")
}

func TestValidateConnectionCustomHeadersWithoutToken(t *testing.T) {
	connection := &models.ClaudeCodeConnection{
		ClaudeCodeConn: models.ClaudeCodeConn{
			Organization: testOrganization,
			CustomHeaders: []models.CustomHeader{
				{Key: "Ocp-Apim-Subscription-Key", Value: "secret-key"},
			},
		},
	}
	connection.Normalize()

	err := validateConnection(connection)
	assert.NoError(t, err)
}

func TestValidateConnectionRejectsBlankCustomHeaders(t *testing.T) {
	connection := &models.ClaudeCodeConnection{
		ClaudeCodeConn: models.ClaudeCodeConn{
			Organization: testOrganization,
			CustomHeaders: []models.CustomHeader{
				{Key: "", Value: ""},
			},
		},
	}
	connection.Normalize()

	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either token or at least one custom header is required")
}

func TestValidateConnectionRejectsIncompleteCustomHeaders(t *testing.T) {
	connection := &models.ClaudeCodeConnection{
		ClaudeCodeConn: models.ClaudeCodeConn{
			Organization: testOrganization,
			CustomHeaders: []models.CustomHeader{
				{Key: "Ocp-Apim-Subscription-Key", Value: ""},
			},
		},
	}
	connection.Normalize()

	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "custom headers must include both key and value")
}

func TestValidateConnectionInvalidRateLimit(t *testing.T) {
	connection := &models.ClaudeCodeConnection{
		ClaudeCodeConn: models.ClaudeCodeConn{
			Organization: testOrganization,
			Token:        testToken,
		},
	}
	connection.RateLimitPerHour = -1

	err := validateConnection(connection)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rateLimitPerHour must be non-negative")
}
