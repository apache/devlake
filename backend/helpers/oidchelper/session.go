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
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const sessionIssuer = "devlake"

// sessionParser is reused across the per-request session-validation path to
// avoid re-allocating the validator options on every call.
var sessionParser = jwt.NewParser(
	jwt.WithIssuer(sessionIssuer),
	jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
)

type SessionClaims struct {
	Provider string `json:"prv,omitempty"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
	jwt.RegisteredClaims
}

// IssueSession signs a session JWT carrying the jti, the provider name (so
// /auth/logout can find the right end_session_endpoint), and the user-facing
// claims. The jti lets the server-side revocation table address one session.
func IssueSession(cfg *Config, jti, provider, sub, email, name string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(cfg.SessionTTL)
	claims := SessionClaims{
		Provider: provider,
		Email:    email,
		Name:     name,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    sessionIssuer,
			Subject:   sub,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(cfg.SessionSecret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func ParseSession(secret []byte, raw string) (*SessionClaims, error) {
	parsed, err := sessionParser.ParseWithClaims(raw, &SessionClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*SessionClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid session token")
	}
	return claims, nil
}
