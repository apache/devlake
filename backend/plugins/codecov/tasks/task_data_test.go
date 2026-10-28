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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTaskOptions(t *testing.T) {
	t.Run("valid options", func(t *testing.T) {
		err := ValidateTaskOptions(&CodecovOptions{
			ConnectionId: 1,
			FullName:     "org/repo",
		})
		assert.Nil(t, err)
	})

	t.Run("zero connectionId returns error", func(t *testing.T) {
		err := ValidateTaskOptions(&CodecovOptions{
			ConnectionId: 0,
			FullName:     "org/repo",
		})
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "connectionId")
	})

	t.Run("empty fullName returns error", func(t *testing.T) {
		err := ValidateTaskOptions(&CodecovOptions{
			ConnectionId: 1,
			FullName:     "",
		})
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "fullName")
	})

	t.Run("both invalid returns connectionId error first", func(t *testing.T) {
		err := ValidateTaskOptions(&CodecovOptions{
			ConnectionId: 0,
			FullName:     "",
		})
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "connectionId")
	})
}

func TestDecodeTaskOptions(t *testing.T) {
	t.Run("valid map decodes correctly", func(t *testing.T) {
		opts := map[string]any{
			"connectionId": uint64(42),
			"fullName":     "owner/repo",
		}
		result, err := DecodeTaskOptions(opts)
		assert.Nil(t, err)
		assert.Equal(t, uint64(42), result.ConnectionId)
		assert.Equal(t, "owner/repo", result.FullName)
	})

	t.Run("empty map produces zero-value options", func(t *testing.T) {
		result, err := DecodeTaskOptions(map[string]any{})
		assert.Nil(t, err)
		assert.Equal(t, uint64(0), result.ConnectionId)
		assert.Empty(t, result.FullName)
	})

	t.Run("scopeConfigId decoded", func(t *testing.T) {
		opts := map[string]any{
			"connectionId":  uint64(1),
			"fullName":      "org/repo",
			"scopeConfigId": uint64(99),
		}
		result, err := DecodeTaskOptions(opts)
		assert.Nil(t, err)
		assert.Equal(t, uint64(99), result.ScopeConfigId)
	})
}

func TestDecodeAndValidateTaskOptions(t *testing.T) {
	t.Run("valid input succeeds", func(t *testing.T) {
		opts := map[string]any{
			"connectionId": uint64(1),
			"fullName":     "owner/repo",
		}
		result, err := DecodeAndValidateTaskOptions(opts)
		assert.Nil(t, err)
		assert.Equal(t, uint64(1), result.ConnectionId)
		assert.Equal(t, "owner/repo", result.FullName)
	})

	t.Run("missing fullName fails validation", func(t *testing.T) {
		opts := map[string]any{
			"connectionId": uint64(1),
		}
		result, err := DecodeAndValidateTaskOptions(opts)
		assert.NotNil(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "fullName")
	})

	t.Run("decode error returns error", func(t *testing.T) {
		// Pass a value that mapstructure cannot decode into uint64
		opts := map[string]any{
			"connectionId": map[string]any{"nested": "object"},
			"fullName":     "owner/repo",
		}
		result, err := DecodeAndValidateTaskOptions(opts)
		assert.NotNil(t, err)
		assert.Nil(t, result)
	})
}

func TestDecodeTaskOptions_Error(t *testing.T) {
	// Pass a value that mapstructure cannot decode into uint64
	opts := map[string]any{
		"connectionId": map[string]any{"nested": "object"},
		"fullName":     "owner/repo",
	}
	result, err := DecodeTaskOptions(opts)
	assert.NotNil(t, err)
	assert.Nil(t, result)
}
