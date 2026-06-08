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

package api

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corectx "github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	contextimpl "github.com/apache/incubator-devlake/impls/context"
	"github.com/apache/incubator-devlake/impls/logruslog"
	"github.com/apache/incubator-devlake/server/api/auth"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// authChainStubDal is a minimal dal.Dal sufficient to drive the auth chain
// without a real database. Only the three methods exercised by this test are
// implemented; any other call would (intentionally) panic via the embedded
// nil dal.Dal, surfacing an unexpected dependency.
type authChainStubDal struct {
	dal.Dal
	firstErr errors.Error // returned by First; nil means "found"
}

// All backs the auth service's revocation-cache boot load. Returning no rows
// keeps the service from depending on a real DB.
func (s *authChainStubDal) All(dst interface{}, _ ...dal.Clause) errors.Error { return nil }

// First backs apikeyhelper.GetApiKey. On success it leaves dst as the zero
// ApiKey, whose empty AllowedPath regex matches every path — enough to model
// "a valid, correctly-scoped key" without persisting one.
func (s *authChainStubDal) First(dst interface{}, _ ...dal.Clause) errors.Error { return s.firstErr }

func (s *authChainStubDal) IsErrorNotFound(err error) bool { return err != nil }

// newAuthChainEnv builds a router whose middleware chain mirrors production
// with AUTH_ENABLED=true and OIDC_ENABLED=false (the hardened-image default):
//
//	RestAuthentication -> OIDCAuthentication -> RequireAuth
//
// firstErr controls how the stubbed API-key lookup resolves.
func newAuthChainEnv(t *testing.T, firstErr errors.Error) *gin.Engine {
	t.Helper()
	// apikeyhelper reads ENCRYPTION_SECRET from the global config (AutomaticEnv).
	t.Setenv("ENCRYPTION_SECRET", strings.Repeat("a", 32))

	cfg := viper.New()
	cfg.Set("ENCRYPTION_SECRET", strings.Repeat("a", 32))
	cfg.Set("AUTH_ENABLED", true)
	cfg.Set("OIDC_ENABLED", false)

	db := &authChainStubDal{firstErr: firstErr}
	basicRes := contextimpl.NewDefaultBasicRes(cfg, logruslog.Global, db)

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	t.Cleanup(cancel)
	svc, err := auth.NewService(ctx, basicRes)
	if err != nil {
		t.Fatalf("auth.NewService: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RestAuthentication(router, basicRes)) // API-key auth for /rest, then re-dispatches
	router.Use(svc.OIDCAuthentication())             // session cookie -> sets user (no-op here)
	router.Use(svc.RequireAuth())                    // terminal gate: no user -> 401

	// Open-API handler, reached only after RestAuthentication rewrites the
	// /rest-prefixed path and re-dispatches through the chain.
	router.POST("/plugins/webhook/connections/:id/deployments", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	// A normal protected route used to prove RequireAuth is actually active.
	router.GET("/plugins/webhook/connections", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	return router
}

// TestAuthChainHoldsForApiKeyWithOIDCEnabled is the regression test for the
// open-API 401 bug. With AUTH_ENABLED on and the OIDC RequireAuth gate live, a
// valid API key against a /rest endpoint must survive the internal
// HandleContext re-dispatch (which wipes gin Keys) and reach the handler.
func TestAuthChainHoldsForApiKeyWithOIDCEnabled(t *testing.T) {
	const restPath = "/rest/plugins/webhook/connections/1/deployments"

	t.Run("valid key reaches open-api handler (200)", func(t *testing.T) {
		router := newAuthChainEnv(t, nil)
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, restPath, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer valid-key")
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (RequireAuth wiped the API-key user)", resp.Code, http.StatusOK)
		}
	})

	t.Run("missing token (401)", func(t *testing.T) {
		router := newAuthChainEnv(t, nil)
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, restPath, strings.NewReader(`{}`))
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusUnauthorized)
		}
		if !strings.Contains(resp.Body.String(), "token is missing") {
			t.Errorf("body = %q, want it to mention 'token is missing'", resp.Body.String())
		}
	})

	t.Run("invalid key (403)", func(t *testing.T) {
		router := newAuthChainEnv(t, errors.NotFound.New("not found"))
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, restPath, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer wrong-key")
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusForbidden)
		}
		if !strings.Contains(resp.Body.String(), "api key is invalid") {
			t.Errorf("body = %q, want it to mention 'api key is invalid'", resp.Body.String())
		}
	})

	t.Run("protected route without user is gated (401)", func(t *testing.T) {
		router := newAuthChainEnv(t, nil)
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/plugins/webhook/connections", nil)
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d (RequireAuth not active?)", resp.Code, http.StatusUnauthorized)
		}
	})
}

var _ corectx.BasicRes = (*contextimpl.DefaultBasicRes)(nil)
