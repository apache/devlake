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
	"fmt"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// minRefreshInterval rate-limits provider re-discovery so a flood of bad
// tokens cannot stampede the IdP's well-known endpoint.
const minRefreshInterval = 30 * time.Second

// Provider lazily resolves the OIDC discovery document for one IdP and
// caches the resulting *oidc.Provider. Re-initializes on signature-
// verification failure to handle JWKS key rotation.
type Provider struct {
	cfg *ProviderConfig

	mu          sync.RWMutex
	provider    *oidc.Provider
	lastRefresh time.Time
}

func NewProvider(cfg *ProviderConfig) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string { return p.cfg.Name }

func (p *Provider) OIDC(ctx context.Context) (*oidc.Provider, error) {
	p.mu.RLock()
	cached := p.provider
	p.mu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	return p.refresh(ctx)
}

// refresh re-initializes the provider, but no more often than
// minRefreshInterval to prevent a stampede of failed verifications from
// hammering the IdP. Callers receive the cached provider when rate-limited.
func (p *Provider) refresh(ctx context.Context) (*oidc.Provider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.provider != nil && time.Since(p.lastRefresh) < minRefreshInterval {
		return p.provider, nil
	}
	prov, err := oidc.NewProvider(ctx, p.cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery (%s): %w", p.cfg.IssuerURL, err)
	}
	p.provider = prov
	p.lastRefresh = time.Now()
	return prov, nil
}

func (p *Provider) OAuth2Config(ctx context.Context) (*oauth2.Config, error) {
	prov, err := p.OIDC(ctx)
	if err != nil {
		return nil, err
	}
	return &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		RedirectURL:  p.cfg.RedirectURL,
		Endpoint:     prov.Endpoint(),
		Scopes:       p.cfg.Scopes,
	}, nil
}

// VerifyIDToken validates the raw ID token against the provider's JWKS.
// On signature-verification failure it forces a JWKS refresh (subject to
// minRefreshInterval) and retries once, transparently handling key rotation.
func (p *Provider) VerifyIDToken(ctx context.Context, raw string) (*oidc.IDToken, error) {
	prov, err := p.OIDC(ctx)
	if err != nil {
		return nil, err
	}
	verifier := prov.Verifier(&oidc.Config{ClientID: p.cfg.ClientID})
	tok, err := verifier.Verify(ctx, raw)
	if err == nil {
		return tok, nil
	}
	prov, refreshErr := p.refresh(ctx)
	if refreshErr != nil {
		return nil, fmt.Errorf("verify id_token: %w (refresh also failed: %v)", err, refreshErr)
	}
	verifier = prov.Verifier(&oidc.Config{ClientID: p.cfg.ClientID})
	return verifier.Verify(ctx, raw)
}

func (p *Provider) EndSessionURL(ctx context.Context) (string, error) {
	prov, err := p.OIDC(ctx)
	if err != nil {
		return "", err
	}
	var claims struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := prov.Claims(&claims); err != nil {
		return "", err
	}
	return claims.EndSessionEndpoint, nil
}
