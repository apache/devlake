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
	"testing"
	"time"
)

func newTestCfg(secret string, ttl time.Duration) *Config {
	return &Config{SessionSecret: []byte(secret), SessionTTL: ttl}
}

func TestSessionRoundTrip(t *testing.T) {
	cfg := newTestCfg("session-test-secret-32-bytes!!", time.Hour)
	jwt, exp, err := IssueSession(cfg, "jti-1", "entra", "user-1", "u@example.com", "Alice")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if exp.IsZero() {
		t.Fatalf("expected non-zero expiry")
	}
	claims, err := ParseSession(cfg.SessionSecret, jwt)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Subject != "user-1" || claims.Email != "u@example.com" || claims.Name != "Alice" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.ID != "jti-1" {
		t.Fatalf("expected jti claim, got %q", claims.ID)
	}
	if claims.Provider != "entra" {
		t.Fatalf("expected provider claim, got %q", claims.Provider)
	}
}

func TestSessionRejectsExpired(t *testing.T) {
	cfg := newTestCfg("session-test-secret-32-bytes!!", -1*time.Second)
	jwt, _, err := IssueSession(cfg, "jti-1", "entra", "user-1", "u@example.com", "Alice")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := ParseSession(cfg.SessionSecret, jwt); err == nil {
		t.Fatal("expected expired token to fail validation")
	}
}

func TestSessionRejectsWrongSecret(t *testing.T) {
	a := newTestCfg("session-test-secret-32-bytes!a", time.Hour)
	b := newTestCfg("session-test-secret-32-bytes!b", time.Hour)
	jwt, _, err := IssueSession(a, "jti-1", "entra", "user-1", "", "")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := ParseSession(b.SessionSecret, jwt); err == nil {
		t.Fatal("expected wrong-secret to fail validation")
	}
}

func TestSessionRejectsTampered(t *testing.T) {
	cfg := newTestCfg("session-test-secret-32-bytes!!", time.Hour)
	jwt, _, err := IssueSession(cfg, "jti-1", "entra", "user-1", "", "")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	tampered := jwt[:len(jwt)-2] + "AA"
	if _, err := ParseSession(cfg.SessionSecret, tampered); err == nil {
		t.Fatal("expected tampered token to fail validation")
	}
}
