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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DecodeAndValidateTaskOptions
// ---------------------------------------------------------------------------

func TestDecodeAndValidateTaskOptions_Valid(t *testing.T) {
	opts, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"boardId":      uint64(42),
		"projectKeys":  []string{"KONFLUX", "HELM"},
	})
	require.Nil(t, err)
	assert.Equal(t, uint64(1), opts.ConnectionId)
	assert.Equal(t, uint64(42), opts.BoardId)
	assert.Equal(t, []string{"KONFLUX", "HELM"}, opts.ProjectKeys)
}

func TestDecodeAndValidateTaskOptions_MissingConnectionId(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"boardId":     uint64(42),
		"projectKeys": []string{"KONFLUX"},
	})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "connectionId")
}

func TestDecodeAndValidateTaskOptions_MissingBoardId(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"projectKeys":  []string{"KONFLUX"},
	})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "boardId")
}

func TestDecodeAndValidateTaskOptions_EmptyProjectKeys(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"boardId":      uint64(42),
		"projectKeys":  []string{},
	})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "projectKeys")
}

func TestDecodeAndValidateTaskOptions_MissingProjectKeys(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"boardId":      uint64(42),
	})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "projectKeys")
}

// ---------------------------------------------------------------------------
// parseRSAPrivateKey
// ---------------------------------------------------------------------------

func generateTestPKCS8PEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	return string(pem.EncodeToMemory(block))
}

func TestParseRSAPrivateKey_ValidPKCS8(t *testing.T) {
	pemStr := generateTestPKCS8PEM(t)
	key, err := parseRSAPrivateKey(pemStr)
	require.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, 2048, key.N.BitLen())
}

func TestParseRSAPrivateKey_EmptyString(t *testing.T) {
	_, err := parseRSAPrivateKey("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PEM")
}

func TestParseRSAPrivateKey_InvalidPEM(t *testing.T) {
	_, err := parseRSAPrivateKey("not a pem block")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PEM")
}

func TestParseRSAPrivateKey_WrongKeyType(t *testing.T) {
	// PKCS#1 format (BEGIN RSA PRIVATE KEY) is not PKCS#8, should fail ParsePKCS8PrivateKey
	key, genErr := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, genErr)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	pemStr := string(pem.EncodeToMemory(block))

	_, err := parseRSAPrivateKey(pemStr)
	assert.Error(t, err, "PKCS#1 key should be rejected; expected PKCS#8")
}

// ---------------------------------------------------------------------------
// stringVal
// ---------------------------------------------------------------------------

func TestStringVal(t *testing.T) {
	t.Run("nil pointer returns empty string", func(t *testing.T) {
		assert.Equal(t, "", stringVal(nil))
	})
	t.Run("non-nil pointer returns value", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, "hello", stringVal(&s))
	})
	t.Run("empty string pointer returns empty string", func(t *testing.T) {
		s := ""
		assert.Equal(t, "", stringVal(&s))
	})
}
