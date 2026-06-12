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

package oidchelper

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// StatePayload is encrypted into the state cookie. The Nonce field is also
// echoed as the OIDC `state` query parameter so we can verify the user's
// browser is the one that initiated the flow. Provider names which IdP
// minted this flow so the callback handler picks the right token endpoint.
type StatePayload struct {
	Provider     string    `json:"v"`
	Nonce        string    `json:"n"`
	ReturnURL    string    `json:"r"`
	PKCEVerifier string    `json:"p"`
	IssuedAt     time.Time `json:"t"`
}

// EncodeState seals a StatePayload with AES-GCM (key derived from
// SESSION_SECRET via SHA-256) and returns a URL-safe base64 string.
func EncodeState(secret []byte, p *StatePayload) (string, error) {
	gcm, err := newGCM(secret)
	if err != nil {
		return "", err
	}
	plaintext, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// DecodeState reverses EncodeState and rejects payloads older than StateCookieMaxAge.
func DecodeState(secret []byte, encoded string) (*StatePayload, error) {
	gcm, err := newGCM(secret)
	if err != nil {
		return nil, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("state decode: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("state too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("state decrypt: %w", err)
	}
	var p StatePayload
	if err := json.Unmarshal(plaintext, &p); err != nil {
		return nil, fmt.Errorf("state unmarshal: %w", err)
	}
	if time.Since(p.IssuedAt) > StateCookieMaxAge {
		return nil, fmt.Errorf("state expired")
	}
	return &p, nil
}

// NewNonce returns a 24-char base64 random string suitable for the OIDC
// `state` parameter (~144 bits of entropy).
func NewNonce() (string, error) {
	b := make([]byte, 18)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewCSRFToken returns a 256-bit random URL-safe string for double-submit CSRF.
func NewCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func newGCM(secret []byte) (cipher.AEAD, error) {
	// AES-256 needs a 32-byte key; SHA-256 of the secret gives one.
	sum := sha256.Sum256(secret)
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
