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

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/mock"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/oidchelper"
	"github.com/apache/devlake/impls/logruslog"
	mockdal "github.com/apache/devlake/mocks/core/dal"
)

// fakeIdP is a minimal OIDC provider sufficient for go-oidc discovery,
// token exchange, and JWKS verification.
type fakeIdP struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	keyID  string
	issuer string

	mu       sync.Mutex
	lastCode string
	subject  string
	email    string
	name     string
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	idp := &fakeIdP{
		key:     key,
		keyID:   "test-key",
		subject: "user-123",
		email:   "alice@example.com",
		name:    "Alice",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", idp.handleDiscovery)
	mux.HandleFunc("/jwks", idp.handleJWKS)
	mux.HandleFunc("/token", idp.handleToken)
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "authorize endpoint not exercised by these tests", http.StatusNotImplemented)
	})
	mux.HandleFunc("/logout", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	idp.server = httptest.NewServer(mux)
	idp.issuer = idp.server.URL
	t.Cleanup(idp.server.Close)
	return idp
}

func (f *fakeIdP) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]any{
		"issuer":                                f.issuer,
		"authorization_endpoint":                f.issuer + "/authorize",
		"token_endpoint":                        f.issuer + "/token",
		"jwks_uri":                              f.issuer + "/jwks",
		"end_session_endpoint":                  f.issuer + "/logout",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

func (f *fakeIdP) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	pub := f.key.PublicKey
	doc := map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"use": "sig",
			"kid": f.keyID,
			"alg": "RS256",
			"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

func (f *fakeIdP) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	code := r.FormValue("code")
	f.mu.Lock()
	f.lastCode = code
	f.mu.Unlock()

	// oauth2.Config.Exchange sends client credentials via HTTP Basic by
	// default. Fall back to the form for clients that send them there.
	clientID := r.FormValue("client_id")
	if u, _, ok := r.BasicAuth(); ok {
		clientID = u
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   f.issuer,
		"aud":   clientID,
		"sub":   f.subject,
		"email": f.email,
		"name":  f.name,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
	})
	tok.Header["kid"] = f.keyID
	signed, err := tok.SignedString(f.key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "dummy-access",
		"token_type":   "Bearer",
		"expires_in":   300,
		"id_token":     signed,
	})
}

// newTestService builds a Service backed by the fake IdP and a permissive
// dal mock that accepts the auth-store writes the handlers issue.
func newTestService(t *testing.T, idp *fakeIdP) (*Service, *mockdal.Dal) {
	t.Helper()
	pc := &oidchelper.ProviderConfig{
		Name:         "test",
		IssuerURL:    idp.issuer,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/auth/callback",
		Scopes:       []string{"openid", "profile", "email"},
		DisplayName:  "Test IdP",

		PostLogoutRedirectURL: "http://localhost/",
	}
	cfg := &oidchelper.Config{
		AuthEnabled:    true,
		OIDCEnabled:    true,
		Providers:      map[string]*oidchelper.ProviderConfig{"test": pc},
		LogoutRedirect: true,
		SessionSecret:  []byte("test-secret-with-at-least-32-bytes!"),
		SessionTTL:     time.Hour,
		CookieSecure:   false,
	}
	db := &mockdal.Dal{}
	// Remember the last persisted session so First can serve its id_token.
	var lastSession AuthSession
	var lastSessionMu sync.Mutex
	recordSession := func(args mock.Arguments) {
		if row, ok := args.Get(0).(*AuthSession); ok {
			lastSessionMu.Lock()
			lastSession = *row
			lastSessionMu.Unlock()
		}
	}
	db.On("Create", mock.Anything).Run(recordSession).Return(nil)
	db.On("Create", mock.Anything, mock.Anything).Run(recordSession).Return(nil)
	db.On("First", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		if dst, ok := args.Get(0).(*AuthSession); ok {
			lastSessionMu.Lock()
			dst.IDToken = lastSession.IDToken
			lastSessionMu.Unlock()
		}
	}).Return(nil)
	db.On("UpdateColumn", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	db.On("UpdateColumns", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	db.On("All", mock.Anything, mock.Anything).Return(nil)
	db.On("Delete", mock.Anything, mock.Anything).Return(nil)

	s := &Service{
		cfg:       cfg,
		providers: map[string]*oidchelper.Provider{"test": oidchelper.NewProvider(pc)},
		logger:    logruslog.Global,
		db:        db,
		revoked:   newRevocationCache(),
		lastSeen:  map[string]time.Time{},
	}
	return s, db
}

func newTestRouter(s *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(s.OIDCAuthentication())
	r.Use(s.RequireAuth())
	r.Use(s.CSRFProtect())
	r.GET(PathLogin, s.LoginInit)
	r.GET(PathCallback, s.Callback)
	r.GET(PathUserInfo, s.UserInfo)
	r.POST(PathLogout, s.Logout)
	r.GET(PathSwitchAccount, s.SwitchAccount)
	r.GET(PathMethods, s.GetMethods)
	return r
}

// extractCookie pulls a Set-Cookie value by name from a recorded response.
func extractCookie(t *testing.T, resp *http.Response, name string) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %q not found in response", name)
	return nil
}

// stateNonceFromCookie decodes the encrypted state cookie back into the
// nonce, which the test then echoes as the OIDC `state` query parameter.
func stateNonceFromCookie(t *testing.T, secret []byte, cookieValue string) string {
	t.Helper()
	state, err := oidchelper.DecodeState(secret, cookieValue)
	if err != nil {
		t.Fatalf("decode state cookie: %v", err)
	}
	return state.Nonce
}

func TestLoginInitSetsStateCookieAndRedirects(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	r := newTestRouter(s)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, PathLogin+"?provider=test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "code_challenge=") || !strings.Contains(loc, "code_challenge_method=S256") {
		t.Fatalf("expected PKCE params on auth URL, got %q", loc)
	}
	resp := w.Result()
	defer resp.Body.Close()
	if c := extractCookie(t, resp, oidchelper.StateCookieName); c.Value == "" {
		t.Fatal("state cookie was empty")
	}
}

func TestFullLoginCallbackFlow(t *testing.T) {
	idp := newFakeIdP(t)
	s, db := newTestService(t, idp)
	r := newTestRouter(s)

	// 1. /auth/login: capture the state cookie.
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, httptest.NewRequest(http.MethodGet, PathLogin+"?provider=test", nil))
	if loginW.Code != http.StatusSeeOther {
		t.Fatalf("login: expected 303, got %d", loginW.Code)
	}
	stateCookie := extractCookie(t, loginW.Result(), oidchelper.StateCookieName)
	nonce := stateNonceFromCookie(t, s.cfg.SessionSecret, stateCookie.Value)

	// 2. /auth/callback: must succeed and set session + csrf cookies.
	cbReq := httptest.NewRequest(http.MethodGet,
		PathCallback+"?code=fake-code&state="+url.QueryEscape(nonce), nil)
	cbReq.AddCookie(stateCookie)
	cbW := httptest.NewRecorder()
	r.ServeHTTP(cbW, cbReq)
	if cbW.Code != http.StatusSeeOther {
		t.Fatalf("callback: expected 303, got %d body=%s", cbW.Code, cbW.Body.String())
	}
	sessionCookie := extractCookie(t, cbW.Result(), oidchelper.SessionCookieName)
	csrfCookie := extractCookie(t, cbW.Result(), oidchelper.CSRFCookieName)
	if sessionCookie.Value == "" || csrfCookie.Value == "" {
		t.Fatal("expected session and csrf cookies to be set")
	}
	created, ok := db.Calls[indexOfCall(db, "Create")].Arguments.Get(0).(*AuthSession)
	if !ok || created.IDToken == "" {
		t.Fatalf("expected the session row to persist the id_token, got %+v", created)
	}

	// 3. /auth/userinfo with the session cookie should report authenticated.
	uiReq := httptest.NewRequest(http.MethodGet, PathUserInfo, nil)
	uiReq.AddCookie(sessionCookie)
	uiW := httptest.NewRecorder()
	r.ServeHTTP(uiW, uiReq)
	if uiW.Code != http.StatusOK {
		t.Fatalf("userinfo: expected 200, got %d", uiW.Code)
	}
	var userResp userInfoResponse
	if err := json.Unmarshal(uiW.Body.Bytes(), &userResp); err != nil {
		t.Fatalf("decode userinfo: %v: %s", err, uiW.Body.String())
	}
	if !userResp.Authenticated || userResp.Email != idp.email {
		t.Fatalf("unexpected userinfo: %+v body=%s", userResp, uiW.Body.String())
	}

	// 4. /auth/logout with session + csrf header should succeed and revoke.
	logoutReq := httptest.NewRequest(http.MethodPost, PathLogout, nil)
	logoutReq.AddCookie(sessionCookie)
	logoutReq.AddCookie(csrfCookie)
	logoutReq.Header.Set(oidchelper.CSRFHeaderName, csrfCookie.Value)
	logoutW := httptest.NewRecorder()
	r.ServeHTTP(logoutW, logoutReq)
	if logoutW.Code != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d body=%s", logoutW.Code, logoutW.Body.String())
	}
	db.AssertCalled(t, "UpdateColumns", mock.Anything, mock.MatchedBy(func(set []dal.DalSet) bool {
		return len(set) == 2 && set[0].ColumnName == "revoked_at" && set[1].ColumnName == "id_token" && set[1].Value == ""
	}), mock.Anything)
	var logoutBody logoutResponse
	if err := json.Unmarshal(logoutW.Body.Bytes(), &logoutBody); err != nil {
		t.Fatalf("decode logout: %v: %s", err, logoutW.Body.String())
	}
	assertEndSessionURL(t, idp, logoutBody.LogoutURL, created.IDToken)

	// 5. The same session cookie now belongs to a revoked jti; userinfo
	// should report not authenticated.
	uiReq2 := httptest.NewRequest(http.MethodGet, PathUserInfo, nil)
	uiReq2.AddCookie(sessionCookie)
	uiW2 := httptest.NewRecorder()
	r.ServeHTTP(uiW2, uiReq2)
	var userResp2 userInfoResponse
	if err := json.Unmarshal(uiW2.Body.Bytes(), &userResp2); err != nil {
		t.Fatalf("decode userinfo (revoked): %v", err)
	}
	if userResp2.Authenticated {
		t.Fatal("expected revoked session to no longer be authenticated")
	}
}

func TestCallbackRejectsStateMismatch(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	r := newTestRouter(s)

	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, httptest.NewRequest(http.MethodGet, PathLogin+"?provider=test", nil))
	stateCookie := extractCookie(t, loginW.Result(), oidchelper.StateCookieName)

	cbReq := httptest.NewRequest(http.MethodGet, PathCallback+"?code=x&state=not-the-real-nonce", nil)
	cbReq.AddCookie(stateCookie)
	cbW := httptest.NewRecorder()
	r.ServeHTTP(cbW, cbReq)
	assertAuthErrorRedirect(t, cbW, AuthErrorExpired, "")
}

func TestCallbackRejectsMissingStateCookie(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	r := newTestRouter(s)

	cbReq := httptest.NewRequest(http.MethodGet, PathCallback+"?code=x&state=anything", nil)
	cbW := httptest.NewRecorder()
	r.ServeHTTP(cbW, cbReq)
	assertAuthErrorRedirect(t, cbW, AuthErrorExpired, "")
}

func TestCSRFRequiredOnUnsafeMethod(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	r := newTestRouter(s)
	r.POST("/some-mutation", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	// Issue a session via the fake-callback so we have valid cookies.
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, httptest.NewRequest(http.MethodGet, PathLogin+"?provider=test", nil))
	stateCookie := extractCookie(t, loginW.Result(), oidchelper.StateCookieName)
	nonce := stateNonceFromCookie(t, s.cfg.SessionSecret, stateCookie.Value)

	cbReq := httptest.NewRequest(http.MethodGet,
		PathCallback+"?code=c&state="+url.QueryEscape(nonce), nil)
	cbReq.AddCookie(stateCookie)
	cbW := httptest.NewRecorder()
	r.ServeHTTP(cbW, cbReq)
	sessionCookie := extractCookie(t, cbW.Result(), oidchelper.SessionCookieName)
	csrfCookie := extractCookie(t, cbW.Result(), oidchelper.CSRFCookieName)

	// POST without the CSRF header is rejected.
	bad := httptest.NewRequest(http.MethodPost, "/some-mutation", nil)
	bad.AddCookie(sessionCookie)
	bad.AddCookie(csrfCookie)
	badW := httptest.NewRecorder()
	r.ServeHTTP(badW, bad)
	if badW.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without csrf header, got %d", badW.Code)
	}

	// POST with a matching header passes.
	good := httptest.NewRequest(http.MethodPost, "/some-mutation", nil)
	good.AddCookie(sessionCookie)
	good.AddCookie(csrfCookie)
	good.Header.Set(oidchelper.CSRFHeaderName, csrfCookie.Value)
	goodW := httptest.NewRecorder()
	r.ServeHTTP(goodW, good)
	if goodW.Code != http.StatusNoContent {
		t.Fatalf("expected 204 with csrf header, got %d body=%s", goodW.Code, goodW.Body.String())
	}
}

func TestLogoutPublicWhenNoSession(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	r := newTestRouter(s)

	// No session cookie, no csrf header. publicPaths should let it through
	// and the handler should respond 200.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, PathLogout, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected logout-without-session to return 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// indexOfCall returns the index of the first recorded call to method.
func indexOfCall(db *mockdal.Dal, method string) int {
	for i, c := range db.Calls {
		if c.Method == method {
			return i
		}
	}
	panic("no call to " + method)
}

// assertAuthErrorRedirect checks that w redirects to the UI error page with
// the given code (and provider, if non-empty) and leaks no error details.
func assertAuthErrorRedirect(t *testing.T, w *httptest.ResponseRecorder, code, provider string) {
	t.Helper()
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if loc.Path != uiAuthErrorPath || loc.IsAbs() {
		t.Fatalf("expected relative redirect to %s, got %s", uiAuthErrorPath, loc)
	}
	q := loc.Query()
	if q.Get("code") != code {
		t.Fatalf("code = %q, want %q (Location=%s)", q.Get("code"), code, loc)
	}
	if q.Get("provider") != provider {
		t.Fatalf("provider = %q, want %q (Location=%s)", q.Get("provider"), provider, loc)
	}
	if len(q) > 2 {
		t.Fatalf("unexpected extra query params in %s", loc)
	}
}

// assertEndSessionURL checks an RP-initiated logout URL against the fake IdP.
func assertEndSessionURL(t *testing.T, idp *fakeIdP, raw, wantHint string) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil || raw == "" {
		t.Fatalf("invalid end-session URL %q: %v", raw, err)
	}
	if got := u.Scheme + "://" + u.Host + u.Path; got != idp.issuer+"/logout" {
		t.Fatalf("end-session URL %q does not target the IdP logout endpoint", raw)
	}
	q := u.Query()
	if q.Get("client_id") != "test-client" {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("post_logout_redirect_uri") != "http://localhost/" {
		t.Errorf("post_logout_redirect_uri = %q", q.Get("post_logout_redirect_uri"))
	}
	if q.Get("id_token_hint") != wantHint {
		t.Errorf("id_token_hint = %q, want %q", q.Get("id_token_hint"), wantHint)
	}
}

// startLogin runs /auth/login and returns the state cookie and its nonce.
func startLogin(t *testing.T, r *gin.Engine, s *Service) (*http.Cookie, string) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, PathLogin+"?provider=test", nil))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("login: expected 303, got %d", w.Code)
	}
	c := extractCookie(t, w.Result(), oidchelper.StateCookieName)
	return c, stateNonceFromCookie(t, s.cfg.SessionSecret, c.Value)
}

func findCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestCallbackForbiddenRedirectsToErrorPage(t *testing.T) {
	idp := newFakeIdP(t)
	s, db := newTestService(t, idp)
	s.cfg.AllowDomains = map[string]struct{}{"other.example": {}}
	r := newTestRouter(s)

	stateCookie, nonce := startLogin(t, r, s)
	cbReq := httptest.NewRequest(http.MethodGet, PathCallback+"?code=c&state="+url.QueryEscape(nonce), nil)
	cbReq.AddCookie(stateCookie)
	cbW := httptest.NewRecorder()
	r.ServeHTTP(cbW, cbReq)

	assertAuthErrorRedirect(t, cbW, AuthErrorForbidden, "test")
	resp := cbW.Result()
	if c := findCookie(resp, oidchelper.SessionCookieName); c != nil && c.Value != "" {
		t.Fatal("rejected user must not get a session cookie")
	}
	db.AssertNotCalled(t, "Create", mock.Anything)
	hint := extractCookie(t, resp, oidchelper.LogoutHintCookieName)
	if !hint.HttpOnly || hint.Value == "" {
		t.Fatalf("expected an HttpOnly logout hint cookie, got %+v", hint)
	}

	// "Sign in with a different account" ends the IdP session using the
	// id_token from the hint and clears the cookie.
	swReq := httptest.NewRequest(http.MethodGet, PathSwitchAccount, nil)
	swReq.AddCookie(hint)
	swW := httptest.NewRecorder()
	r.ServeHTTP(swW, swReq)
	if swW.Code != http.StatusSeeOther {
		t.Fatalf("switch-account: expected 303, got %d body=%s", swW.Code, swW.Body.String())
	}
	decoded, err := oidchelper.DecodeLogoutHint(s.cfg.SessionSecret, hint.Value)
	if err != nil {
		t.Fatalf("decode hint: %v", err)
	}
	assertEndSessionURL(t, idp, swW.Header().Get("Location"), decoded.IDToken)
	if c := findCookie(swW.Result(), oidchelper.LogoutHintCookieName); c == nil || c.MaxAge >= 0 {
		t.Fatalf("expected logout hint cookie to be cleared, got %+v", c)
	}
}

func TestSwitchAccountWithoutHint(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	r := newTestRouter(s)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, PathSwitchAccount+"?provider=test", nil))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}
	assertEndSessionURL(t, idp, w.Header().Get("Location"), "")
}

func TestSwitchAccountUnknownProvider(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	r := newTestRouter(s)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, PathSwitchAccount+"?provider=nope", nil))
	assertAuthErrorRedirect(t, w, AuthErrorServerError, "")
}

func TestCallbackIdPErrorRedirectsToErrorPage(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	r := newTestRouter(s)

	stateCookie, nonce := startLogin(t, r, s)
	cbReq := httptest.NewRequest(http.MethodGet,
		PathCallback+"?error=access_denied&error_description=denied&state="+url.QueryEscape(nonce), nil)
	cbReq.AddCookie(stateCookie)
	cbW := httptest.NewRecorder()
	r.ServeHTTP(cbW, cbReq)
	assertAuthErrorRedirect(t, cbW, AuthErrorIdP, "test")
}

func TestCallbackPersistFailureRedirectsToErrorPage(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	failing := &mockdal.Dal{}
	failing.On("Create", mock.Anything).Return(errors.Default.New("db down"))
	s.db = failing
	r := newTestRouter(s)

	stateCookie, nonce := startLogin(t, r, s)
	cbReq := httptest.NewRequest(http.MethodGet, PathCallback+"?code=c&state="+url.QueryEscape(nonce), nil)
	cbReq.AddCookie(stateCookie)
	cbW := httptest.NewRecorder()
	r.ServeHTTP(cbW, cbReq)
	assertAuthErrorRedirect(t, cbW, AuthErrorServerError, "test")
}

func TestLoginInitUnknownProviderRedirectsToErrorPage(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	r := newTestRouter(s)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, PathLogin+"?provider=nope", nil))
	assertAuthErrorRedirect(t, w, AuthErrorServerError, "")
}

func TestLoginInitUnreachableIdPRedirectsToErrorPage(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	idp.server.Close()
	r := newTestRouter(s)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, PathLogin+"?provider=test", nil))
	assertAuthErrorRedirect(t, w, AuthErrorIdP, "test")
	if c := findCookie(w.Result(), oidchelper.StateCookieName); c != nil {
		t.Fatal("no state cookie expected when the login cannot start")
	}
}

func TestLogoutRedirectDisabledKeepsNoIDToken(t *testing.T) {
	idp := newFakeIdP(t)
	s, db := newTestService(t, idp)
	s.cfg.LogoutRedirect = false
	r := newTestRouter(s)

	stateCookie, nonce := startLogin(t, r, s)
	cbReq := httptest.NewRequest(http.MethodGet, PathCallback+"?code=c&state="+url.QueryEscape(nonce), nil)
	cbReq.AddCookie(stateCookie)
	cbW := httptest.NewRecorder()
	r.ServeHTTP(cbW, cbReq)
	if cbW.Code != http.StatusSeeOther {
		t.Fatalf("callback: expected 303, got %d", cbW.Code)
	}
	created := db.Calls[indexOfCall(db, "Create")].Arguments.Get(0).(*AuthSession)
	if created.IDToken != "" {
		t.Fatal("id_token must not be stored when OIDC_LOGOUT_REDIRECT is off")
	}

	sessionCookie := extractCookie(t, cbW.Result(), oidchelper.SessionCookieName)
	csrfCookie := extractCookie(t, cbW.Result(), oidchelper.CSRFCookieName)
	logoutReq := httptest.NewRequest(http.MethodPost, PathLogout, nil)
	logoutReq.AddCookie(sessionCookie)
	logoutReq.AddCookie(csrfCookie)
	logoutReq.Header.Set(oidchelper.CSRFHeaderName, csrfCookie.Value)
	logoutW := httptest.NewRecorder()
	r.ServeHTTP(logoutW, logoutReq)
	var body logoutResponse
	if err := json.Unmarshal(logoutW.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode logout: %v", err)
	}
	if !body.OK || body.LogoutURL != "" {
		t.Fatalf("expected local-only logout, got %+v", body)
	}
	db.AssertNotCalled(t, "First", mock.Anything, mock.Anything)
}

func TestGetMethodsExposesAccessRequestURL(t *testing.T) {
	idp := newFakeIdP(t)
	s, _ := newTestService(t, idp)
	s.cfg.AccessRequestURL = "https://intranet.example.com/access"
	r := newTestRouter(s)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, PathMethods, nil))
	var m Methods
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode methods: %v: %s", err, w.Body.String())
	}
	if m.AccessRequestURL != "https://intranet.example.com/access" {
		t.Fatalf("accessRequestUrl = %q", m.AccessRequestURL)
	}
}

// Compile-time assertion that the testify mock satisfies the dal.Dal
// interface so the tests catch any new method that gets added.
var _ dal.Dal = (*mockdal.Dal)(nil)
