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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apache/incubator-devlake/core/utils"
)

const (
	testHeaderKey   = "Ocp-Apim-Subscription-Key"
	testHeaderValue = "secret-key"
)

func TestMergeFromRequestPreservesSanitizedSecrets(t *testing.T) {
	connection := &ClaudeCodeConnection{
		ClaudeCodeConn: ClaudeCodeConn{
			Organization: "anthropic-labs",
			Token:        "sk-ant-example",
			CustomHeaders: []CustomHeader{
				{Key: testHeaderKey, Value: testHeaderValue},
			},
		},
	}

	body := map[string]interface{}{
		"token": utils.SanitizeString(connection.Token),
		"customHeaders": []map[string]interface{}{
			{
				"key":   connection.CustomHeaders[0].Key,
				"value": utils.SanitizeString(connection.CustomHeaders[0].Value),
			},
		},
	}

	err := (&ClaudeCodeConnection{}).MergeFromRequest(connection, body)

	assert.NoError(t, err)
	assert.Equal(t, "sk-ant-example", connection.Token)
	assert.Equal(t, []CustomHeader{{Key: testHeaderKey, Value: testHeaderValue}}, connection.CustomHeaders)
}

func TestMergeFromRequestPreservesSanitizedHeaderValueWhenKeyChanges(t *testing.T) {
	connection := &ClaudeCodeConnection{
		ClaudeCodeConn: ClaudeCodeConn{
			Organization: "anthropic-labs",
			CustomHeaders: []CustomHeader{
				{Key: testHeaderKey, Value: testHeaderValue},
			},
		},
	}

	body := map[string]interface{}{
		"customHeaders": []map[string]interface{}{
			{
				"key":   "X-Subscription-Key",
				"value": utils.SanitizeString(connection.CustomHeaders[0].Value),
			},
		},
	}

	err := (&ClaudeCodeConnection{}).MergeFromRequest(connection, body)

	assert.NoError(t, err)
	assert.Equal(t, []CustomHeader{{Key: "X-Subscription-Key", Value: testHeaderValue}}, connection.CustomHeaders)
}
