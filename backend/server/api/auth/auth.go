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

// Package auth implements user-facing OIDC login for DevLake. Supports any
// number of OIDC providers configured via OIDC_PROVIDERS + per-provider env
// vars; routes /auth/login?provider=<name> to the corresponding IdP.
package auth

import (
	stdctx "context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	corectx "github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/helpers/oidchelper"
	"github.com/apache/incubator-devlake/server/api/shared"
)

// Auth-related route paths, defined in one place so router registration and
// the middleware whitelist cannot drift.
const (
	PathMethods  = "/auth/methods"
	PathLogin    = "/auth/login"
	PathCallback = "/auth/callback"
	PathLogout   = "/auth/logout"
	PathUserInfo = "/auth/userinfo"
)

// lastSeenThrottle bounds DB writes to one per-jti per window. Tracking
// "last activity" doesn't need per-request precision.
const lastSeenThrottle = 5 * time.Minute

// Service holds everything the auth handlers and middlewares need. It is the
// unit of testability for this package: tests build one with stub deps and
// drive the gin handlers directly.
type Service struct {
	cfg       *oidchelper.Config
	providers map[string]*oidchelper.Provider
	logger    log.Logger
	db        dal.Dal
	revoked   *revocationCache

	lastSeenMu sync.Mutex
	lastSeen   map[string]time.Time
}

// defaultService is populated by Init and backs the package-level handler /
// middleware wrappers. Tests that need isolation should construct their own
// *Service via NewService.
var (
	defaultService *Service
	initOnce       sync.Once
)

// Init builds the default Service from env config and starts the background
// loops. Panics if auth is enabled but the config is incomplete.
func Init(basicRes corectx.BasicRes) {
	initOnce.Do(func() {
		s, err := NewService(stdctx.Background(), basicRes)
		if err != nil {
			panic(fmt.Errorf("auth init: %w", err))
		}
		defaultService = s
	})
}

// NewService is the testable constructor. ctx governs the lifetime of the
// revocation refresher and session cleanup goroutines.
func NewService(ctx stdctx.Context, basicRes corectx.BasicRes) (*Service, error) {
	cfg, err := oidchelper.LoadConfig(basicRes)
	if err != nil {
		return nil, err
	}
	s := &Service{
		cfg:       cfg,
		providers: map[string]*oidchelper.Provider{},
		logger:    basicRes.GetLogger(),
		db:        basicRes.GetDal(),
		revoked:   newRevocationCache(),
		lastSeen:  map[string]time.Time{},
	}
	if cfg.AuthEnabled {
		startRefresher(ctx, s.revoked, s.db, s.logger)
		startSessionCleanup(ctx, s.db, s.logger)
	}
	if cfg.OIDCEnabled {
		for name, pc := range cfg.Providers {
			s.providers[name] = oidchelper.NewProvider(pc)
			s.logger.Info("OIDC provider %q enabled (issuer=%s, client=%s)", name, pc.IssuerURL, pc.ClientID)
		}
	} else if cfg.AuthEnabled {
		s.logger.Info("AUTH_ENABLED but OIDC_ENABLED=false: only API-key/proxy auth will work")
	}
	return s, nil
}

func (s *Service) Config() *oidchelper.Config { return s.cfg }

// Config returns the default service's config. Nil before Init has run.
func Config() *oidchelper.Config {
	if defaultService == nil {
		return nil
	}
	return defaultService.cfg
}

type ProviderInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	LoginURL    string `json:"loginUrl"`
}

type Methods struct {
	Providers []ProviderInfo `json:"providers,omitempty"`
	APIKey    *APIKey        `json:"apiKey,omitempty"`
}

type APIKey struct {
	Enabled bool `json:"enabled"`
}

func GetMethods(c *gin.Context) { defaultService.GetMethods(c) }

// @Summary List enabled login methods
// @Tags framework/auth
// @Success 200 {object} Methods
// @Router /auth/methods [get]
func (s *Service) GetMethods(c *gin.Context) {
	out := Methods{APIKey: &APIKey{Enabled: true}}
	if s.cfg != nil && s.cfg.OIDCEnabled {
		for _, name := range s.cfg.ProviderNames() {
			pc := s.cfg.Providers[name]
			out.Providers = append(out.Providers, ProviderInfo{
				Name:        name,
				DisplayName: pc.DisplayName,
				LoginURL:    PathLogin + "?provider=" + url.QueryEscape(name),
			})
		}
	}
	shared.ApiOutputSuccess(c, out, http.StatusOK)
}

func LoginInit(c *gin.Context) { defaultService.LoginInit(c) }

// @Summary Begin OIDC login
// @Tags framework/auth
// @Param provider query string false "provider name (required when more than one is configured)"
// @Param return_url query string false "where to redirect after login"
// @Success 303
// @Router /auth/login [get]
func (s *Service) LoginInit(c *gin.Context) {
	if !s.ensureOIDC(c) {
		return
	}
	name, p, ok := s.pickProvider(c, c.Query("provider"))
	if !ok {
		return
	}
	returnURL := safeReturnURL(c.Query("return_url"))

	verifier, err := newPKCEVerifier()
	if err != nil {
		fail(c, http.StatusInternalServerError, "pkce generate", err)
		return
	}
	nonce, err := oidchelper.NewNonce()
	if err != nil {
		fail(c, http.StatusInternalServerError, "state nonce", err)
		return
	}
	encoded, err := oidchelper.EncodeState(s.cfg.SessionSecret, &oidchelper.StatePayload{
		Provider:     name,
		Nonce:        nonce,
		ReturnURL:    returnURL,
		PKCEVerifier: verifier,
		IssuedAt:     time.Now(),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "state encode", err)
		return
	}
	oidchelper.SetStateCookie(c, s.cfg, encoded)

	oa, err := p.OAuth2Config(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadGateway, "oauth2 config", err)
		return
	}
	authURL := oa.AuthCodeURL(nonce,
		oauth2.SetAuthURLParam("code_challenge", pkceChallenge(verifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	c.Redirect(http.StatusSeeOther, authURL)
}

func Callback(c *gin.Context) { defaultService.Callback(c) }

// @Summary OIDC callback
// @Tags framework/auth
// @Success 303
// @Router /auth/callback [get]
func (s *Service) Callback(c *gin.Context) {
	if !s.ensureOIDC(c) {
		return
	}

	encoded, err := c.Cookie(oidchelper.StateCookieName)
	if err != nil || encoded == "" {
		fail(c, http.StatusBadRequest, "missing state cookie", err)
		return
	}
	oidchelper.ClearStateCookie(c, s.cfg)

	state, err := oidchelper.DecodeState(s.cfg.SessionSecret, encoded)
	if err != nil {
		fail(c, http.StatusBadRequest, "state decode", err)
		return
	}
	// Constant-time compare even though the nonce is non-secret. Keeps the
	// auth path free of timing-leak audit questions.
	if subtle.ConstantTimeCompare([]byte(c.Query("state")), []byte(state.Nonce)) != 1 {
		fail(c, http.StatusBadRequest, "state mismatch", nil)
		return
	}
	p, ok := s.providers[state.Provider]
	if !ok {
		fail(c, http.StatusBadRequest, "unknown provider in state: "+state.Provider, nil)
		return
	}
	code := c.Query("code")
	if code == "" {
		if msg := c.Query("error"); msg != "" {
			fail(c, http.StatusBadRequest, "idp error: "+msg+" "+c.Query("error_description"), nil)
			return
		}
		fail(c, http.StatusBadRequest, "missing authorization code", nil)
		return
	}

	oa, err := p.OAuth2Config(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadGateway, "oauth2 config", err)
		return
	}
	exchangeOpts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", state.PKCEVerifier),
	}
	if pc := s.cfg.Providers[state.Provider]; pc != nil && pc.UseWorkloadIdentity {
		assertion, werr := oidchelper.FederatedAssertion()
		if werr != nil {
			fail(c, http.StatusInternalServerError, "workload identity assertion", werr)
			return
		}
		exchangeOpts = append(exchangeOpts,
			oauth2.SetAuthURLParam("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"),
			oauth2.SetAuthURLParam("client_assertion", assertion),
		)
	}
	tok, err := oa.Exchange(c.Request.Context(), code, exchangeOpts...)
	if err != nil {
		fail(c, http.StatusBadGateway, "code exchange", err)
		return
	}
	rawID, _ := tok.Extra("id_token").(string)
	if rawID == "" {
		fail(c, http.StatusBadGateway, "id_token missing from token response", nil)
		return
	}
	idTok, err := p.VerifyIDToken(c.Request.Context(), rawID)
	if err != nil {
		fail(c, http.StatusUnauthorized, "id_token verify", err)
		return
	}

	sub, email, name, err := extractUser(idTok)
	if err != nil {
		fail(c, http.StatusBadGateway, "extract claims", err)
		return
	}
	jti := uuid.NewString()
	jwt, expiresAt, err := oidchelper.IssueSession(s.cfg, jti, state.Provider, sub, email, name)
	if err != nil {
		fail(c, http.StatusInternalServerError, "issue session", err)
		return
	}
	now := time.Now()
	if dbErr := CreateSession(s.db, &AuthSession{
		Jti:        jti,
		Sub:        sub,
		Email:      email,
		Name:       name,
		IssuedAt:   now,
		ExpiresAt:  expiresAt,
		LastSeenAt: now,
	}); dbErr != nil {
		fail(c, http.StatusInternalServerError, "persist session", dbErr)
		return
	}
	csrf, err := oidchelper.NewCSRFToken()
	if err != nil {
		fail(c, http.StatusInternalServerError, "csrf token", err)
		return
	}
	oidchelper.SetSessionCookie(c, s.cfg, jwt)
	oidchelper.SetCSRFCookie(c, s.cfg, csrf)
	s.logger.Info("oidc login: provider=%s sub=%s email=%s jti=%s", state.Provider, sub, email, jti)

	c.Redirect(http.StatusSeeOther, state.ReturnURL)
}

type logoutResponse struct {
	OK        bool   `json:"ok"`
	LogoutURL string `json:"logoutUrl,omitempty"`
}

func Logout(c *gin.Context) { defaultService.Logout(c) }

// @Summary Logout
// @Tags framework/auth
// @Success 200 {object} logoutResponse
// @Router /auth/logout [post]
func (s *Service) Logout(c *gin.Context) {
	if s.cfg == nil || !s.cfg.AuthEnabled {
		shared.ApiOutputSuccess(c, logoutResponse{OK: true}, http.StatusOK)
		return
	}

	var sessionProvider string
	if len(s.cfg.SessionSecret) > 0 {
		if raw, err := c.Cookie(oidchelper.SessionCookieName); err == nil && raw != "" {
			if claims, err := oidchelper.ParseSession(s.cfg.SessionSecret, raw); err == nil && claims.ID != "" {
				sessionProvider = claims.Provider
				if err := RevokeSession(s.db, claims.ID); err != nil {
					s.logger.Error(err, "auth: revoke session row")
				}
				s.revoked.Add(claims.ID)
			}
		}
	}
	oidchelper.ClearSessionCookie(c, s.cfg)
	oidchelper.ClearCSRFCookie(c, s.cfg)

	out := logoutResponse{OK: true}
	if s.cfg.OIDCEnabled && s.cfg.LogoutRedirect && sessionProvider != "" {
		if p, ok := s.providers[sessionProvider]; ok {
			if u, err := p.EndSessionURL(c.Request.Context()); err == nil && u != "" {
				out.LogoutURL = u
			}
		}
	}
	shared.ApiOutputSuccess(c, out, http.StatusOK)
}

type userInfoResponse struct {
	Authenticated bool   `json:"authenticated"`
	Name          string `json:"name"`
	Email         string `json:"email"`
}

func UserInfo(c *gin.Context) { defaultService.UserInfo(c) }

// UserInfo always returns 200 so calling it does not trigger the UI's
// 401-redirect loop; the `authenticated` field discriminates.
//
// @Summary Current user
// @Tags framework/auth
// @Success 200 {object} userInfoResponse
// @Router /auth/userinfo [get]
func (s *Service) UserInfo(c *gin.Context) {
	u, ok := shared.GetUser(c)
	if !ok || u == nil {
		shared.ApiOutputSuccess(c, userInfoResponse{Authenticated: false}, http.StatusOK)
		return
	}
	shared.ApiOutputSuccess(c, userInfoResponse{
		Authenticated: true,
		Name:          u.Name,
		Email:         u.Email,
	}, http.StatusOK)
}

// pickProvider resolves the requested provider name. Empty names are allowed
// only when exactly one provider is configured (single-IdP convenience).
func (s *Service) pickProvider(c *gin.Context, requested string) (string, *oidchelper.Provider, bool) {
	name := strings.ToLower(strings.TrimSpace(requested))
	if name == "" {
		if len(s.providers) == 1 {
			for n := range s.providers {
				name = n
			}
		} else {
			fail(c, http.StatusBadRequest, "provider query param required (multiple configured)", nil)
			return "", nil, false
		}
	}
	p, ok := s.providers[name]
	if !ok {
		fail(c, http.StatusBadRequest, "unknown provider: "+name, nil)
		return "", nil, false
	}
	return name, p, true
}

func (s *Service) ensureOIDC(c *gin.Context) bool {
	if s.cfg == nil || !s.cfg.OIDCEnabled || len(s.providers) == 0 {
		shared.ApiOutputError(c, errors.HttpStatus(http.StatusServiceUnavailable).New("OIDC is not enabled"))
		return false
	}
	return true
}

// bumpLastSeen records that jti was used. The DB write happens at most once
// per lastSeenThrottle per jti, off the request path.
func (s *Service) bumpLastSeen(jti string) {
	now := time.Now()
	s.lastSeenMu.Lock()
	if last, ok := s.lastSeen[jti]; ok && now.Sub(last) < lastSeenThrottle {
		s.lastSeenMu.Unlock()
		return
	}
	s.lastSeen[jti] = now
	s.lastSeenMu.Unlock()

	go func() {
		if err := UpdateLastSeen(s.db, jti, now); err != nil {
			s.logger.Debug("auth: bump last_seen jti=%s: %v", jti, err)
		}
	}()
}

func fail(c *gin.Context, status int, msg string, cause error) {
	wrapped := errors.HttpStatus(status).New(msg)
	if cause != nil {
		wrapped = errors.HttpStatus(status).Wrap(cause, msg)
	}
	shared.ApiOutputError(c, wrapped)
}

// safeReturnURL prevents open-redirect by stripping anything that doesn't
// look like a same-origin path. The // and /\ prefixes get a fast path
// because some browsers normalize backslash → slash and would treat
// /\evil.com as a protocol-relative URL.
func safeReturnURL(raw string) string {
	if raw == "" {
		return "/"
	}
	if strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, `/\`) {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "/"
	}
	if u.IsAbs() || u.Host != "" || !strings.HasPrefix(u.Path, "/") {
		return "/"
	}
	return u.RequestURI()
}

// extractUser pulls user identity out of the verified ID token. Email is
// taken strictly from the `email` claim; we never coerce a username into
// the email column. Display name falls back to preferred_username, then
// email, so the UI always has *something* to render.
func extractUser(tok *oidc.IDToken) (sub, email, name string, err error) {
	var claims struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
		Name              string `json:"name"`
	}
	if err := tok.Claims(&claims); err != nil {
		return "", "", "", err
	}
	if claims.Sub == "" {
		return "", "", "", fmt.Errorf("id_token missing sub claim")
	}
	name = claims.Name
	if name == "" {
		name = claims.PreferredUsername
	}
	if name == "" {
		name = claims.Email
	}
	return claims.Sub, claims.Email, name, nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// newPKCEVerifier returns a 64-char random URL-safe string per RFC 7636.
func newPKCEVerifier() (string, error) {
	b := make([]byte, 48)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
