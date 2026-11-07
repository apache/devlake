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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// discoveryServer serves a minimal OIDC discovery document whose
// end_session_endpoint is built from endSessionPath ("" omits it).
func discoveryServer(t *testing.T, endSessionPath string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		doc := map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
		}
		if endSessionPath != "" {
			doc["end_session_endpoint"] = srv.URL + endSessionPath
		}
		_ = json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestEndSessionURLAddsLogoutParams(t *testing.T) {
	srv := discoveryServer(t, "/logout?ui_locales=en")
	p := NewProvider(&ProviderConfig{
		IssuerURL:             srv.URL,
		ClientID:              "devlake",
		PostLogoutRedirectURL: "https://devlake.example.com/",
	})

	raw, err := p.EndSessionURL(context.Background(), "the-id-token")
	if err != nil {
		t.Fatalf("EndSessionURL: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	if u.Path != "/logout" {
		t.Fatalf("path = %q", u.Path)
	}
	q := u.Query()
	want := map[string]string{
		"ui_locales":               "en",
		"client_id":                "devlake",
		"post_logout_redirect_uri": "https://devlake.example.com/",
		"id_token_hint":            "the-id-token",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q (url=%s)", k, q.Get(k), v, raw)
		}
	}
}

func TestEndSessionURLOmitsEmptyHint(t *testing.T) {
	srv := discoveryServer(t, "/logout")
	p := NewProvider(&ProviderConfig{IssuerURL: srv.URL, ClientID: "devlake"})

	raw, err := p.EndSessionURL(context.Background(), "")
	if err != nil {
		t.Fatalf("EndSessionURL: %v", err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	if q.Has("id_token_hint") || q.Has("post_logout_redirect_uri") {
		t.Fatalf("unexpected optional params in %s", raw)
	}
	if q.Get("client_id") != "devlake" {
		t.Fatalf("client_id missing in %s", raw)
	}
}

func TestEndSessionURLEmptyWithoutEndpoint(t *testing.T) {
	srv := discoveryServer(t, "")
	p := NewProvider(&ProviderConfig{IssuerURL: srv.URL, ClientID: "devlake"})

	raw, err := p.EndSessionURL(context.Background(), "hint")
	if err != nil {
		t.Fatalf("EndSessionURL: %v", err)
	}
	if raw != "" {
		t.Fatalf("expected empty URL, got %q", raw)
	}
}
