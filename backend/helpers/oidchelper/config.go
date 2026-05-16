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
	"sort"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/config"
	"github.com/apache/incubator-devlake/core/context"
)

const (
	SessionCookieName = "devlake_session"
	StateCookieName   = "devlake_oauth_state"
	// CSRFCookieName is intentionally readable by JS so axios can echo it
	// back as the X-CSRF-Token header (double-submit cookie pattern).
	CSRFCookieName = "devlake_csrf"
	CSRFHeaderName = "X-CSRF-Token"

	// State cookie max-age. Covers a slow user typing 2FA at the IdP.
	StateCookieMaxAge = 10 * time.Minute

	defaultSessionTTL = 8 * time.Hour
	defaultScopes     = "openid,profile,email"
)

// ProviderConfig holds everything specific to one OIDC IdP. Multiple
// instances live in Config.Providers, keyed by Name.
type ProviderConfig struct {
	Name         string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	DisplayName  string

	// UseWorkloadIdentity authenticates the code exchange with an Azure
	// Workload Identity federated assertion (read from the SA token file)
	// instead of ClientSecret. Entra-specific.
	UseWorkloadIdentity bool
}

// Config is the typed view of the auth-related env vars. Build it once at boot.
type Config struct {
	AuthEnabled bool

	OIDCEnabled    bool
	Providers      map[string]*ProviderConfig
	LogoutRedirect bool

	SessionSecret []byte
	SessionTTL    time.Duration

	CookieDomain string
	CookieSecure bool
}

// ProviderNames returns the configured provider names in stable order.
func (c *Config) ProviderNames() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Providers))
	for name := range c.Providers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// LoadConfig reads auth env vars via Viper and validates required fields.
// AUTH_ENABLED defaults to true unless it is explicitly set to false.
func LoadConfig(basicRes context.BasicRes) (*Config, error) {
	cfg := basicRes.GetConfigReader()

	authEnabled := true
	if cfg.IsSet("AUTH_ENABLED") {
		authEnabled = cfg.GetBool("AUTH_ENABLED")
	}
	if !authEnabled {
		return &Config{AuthEnabled: false}, nil
	}

	oidcEnabled := cfg.GetBool("OIDC_ENABLED")
	sessionSecret := strings.TrimSpace(cfg.GetString("SESSION_SECRET"))
	if oidcEnabled {
		if sessionSecret == "" {
			return nil, fmt.Errorf("OIDC_ENABLED=true but SESSION_SECRET is not set")
		}
		if len(sessionSecret) < 32 {
			return nil, fmt.Errorf("SESSION_SECRET must be at least 32 bytes")
		}
	} else if sessionSecret != "" && len(sessionSecret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 bytes")
	}

	ttl := defaultSessionTTL
	if v := strings.TrimSpace(cfg.GetString("SESSION_TTL")); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SESSION_TTL %q: %w", v, err)
		}
		ttl = parsed
	}

	cookieSecure := true
	if cfg.IsSet("COOKIE_SECURE") {
		cookieSecure = cfg.GetBool("COOKIE_SECURE")
	}

	out := &Config{
		AuthEnabled:    true,
		OIDCEnabled:    oidcEnabled,
		Providers:      map[string]*ProviderConfig{},
		LogoutRedirect: cfg.GetBool("OIDC_LOGOUT_REDIRECT"),
		SessionSecret:  []byte(sessionSecret),
		SessionTTL:     ttl,
		CookieDomain:   strings.TrimSpace(cfg.GetString("COOKIE_DOMAIN")),
		CookieSecure:   cookieSecure,
	}

	if !out.OIDCEnabled {
		return out, nil
	}

	names := parseProviderNames(cfg.GetString("OIDC_PROVIDERS"))
	if len(names) == 0 {
		return nil, fmt.Errorf("OIDC_ENABLED=true but OIDC_PROVIDERS is empty")
	}
	for _, name := range names {
		prefix := "OIDC_" + strings.ToUpper(name) + "_"
		p, err := loadProviderConfig(cfg, name, prefix)
		if err != nil {
			return nil, err
		}
		out.Providers[name] = p
	}
	return out, nil
}

func loadProviderConfig(cfg config.ConfigReader, name, prefix string) (*ProviderConfig, error) {
	p := &ProviderConfig{
		Name:                name,
		IssuerURL:           strings.TrimSpace(cfg.GetString(prefix + "ISSUER_URL")),
		ClientID:            strings.TrimSpace(cfg.GetString(prefix + "CLIENT_ID")),
		ClientSecret:        strings.TrimSpace(cfg.GetString(prefix + "CLIENT_SECRET")),
		RedirectURL:         strings.TrimSpace(cfg.GetString(prefix + "REDIRECT_URL")),
		Scopes:              parseScopes(cfg.GetString(prefix + "SCOPES")),
		DisplayName:         valueOr(cfg.GetString(prefix+"DISPLAY_NAME"), name),
		UseWorkloadIdentity: cfg.GetBool(prefix + "USE_WORKLOAD_IDENTITY"),
	}

	missing := []string{}
	if p.IssuerURL == "" {
		missing = append(missing, prefix+"ISSUER_URL")
	}
	if p.ClientID == "" {
		missing = append(missing, prefix+"CLIENT_ID")
	}
	// Workload Identity replaces the static client secret with a federated
	// assertion read at exchange time, so the secret is optional in that mode.
	if p.ClientSecret == "" && !p.UseWorkloadIdentity {
		missing = append(missing, prefix+"CLIENT_SECRET")
	}
	if p.RedirectURL == "" {
		missing = append(missing, prefix+"REDIRECT_URL")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("provider %q missing required env vars: %s", name, strings.Join(missing, ", "))
	}
	return p, nil
}

func parseProviderNames(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		n := strings.ToLower(strings.TrimSpace(p))
		if n == "" {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func parseScopes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		raw = defaultScopes
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func valueOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
