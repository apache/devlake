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
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/helpers/oidchelper"
	"github.com/apache/incubator-devlake/server/api/shared"
)

// publicPaths is the set of routes reachable without authentication.
// /auth/userinfo and /auth/logout are public so the UI can poll identity
// and clear its session even when the cookie has lapsed; both handlers
// short-circuit gracefully when no user is set.
var publicPaths = map[string]struct{}{
	"/ping":                 {},
	"/ready":                {},
	"/health":               {},
	"/version":              {},
	"/proceed-db-migration": {},
	PathMethods:             {},
	PathLogin:               {},
	PathCallback:            {},
	PathLogout:              {},
	PathUserInfo:            {},
}

func OIDCAuthentication() gin.HandlerFunc { return defaultService.OIDCAuthentication() }

func RequireAuth() gin.HandlerFunc { return defaultService.RequireAuth() }

func CSRFProtect() gin.HandlerFunc { return defaultService.CSRFProtect() }

// OIDCAuthentication reads the session cookie, verifies the JWT, and sets
// common.USER on the context. Soft authenticator: invalid/missing cookies
// pass through and RequireAuth decides whether to reject.
func (s *Service) OIDCAuthentication() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.cfg == nil || !s.cfg.AuthEnabled {
			c.Next()
			return
		}
		if _, ok := shared.GetUser(c); ok {
			c.Next()
			return
		}
		if len(s.cfg.SessionSecret) == 0 {
			c.Next()
			return
		}
		raw, err := c.Cookie(oidchelper.SessionCookieName)
		if err != nil || raw == "" {
			c.Next()
			return
		}
		claims, err := oidchelper.ParseSession(s.cfg.SessionSecret, raw)
		if err != nil {
			s.logger.Debug("invalid session cookie: %v", err)
			oidchelper.ClearSessionCookie(c, s.cfg)
			c.Next()
			return
		}
		if s.revoked != nil && s.revoked.IsRevoked(claims.ID) {
			s.logger.Debug("revoked session presented: jti=%s", claims.ID)
			oidchelper.ClearSessionCookie(c, s.cfg)
			c.Next()
			return
		}
		c.Set(common.USER, &common.User{
			Name:  claims.Name,
			Email: claims.Email,
		})
		s.bumpLastSeen(claims.ID)
		c.Next()
	}
}

// RequireAuth is the terminal gate. It only becomes a no-op when
// AUTH_ENABLED=false is set explicitly.
func (s *Service) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.cfg == nil || !s.cfg.AuthEnabled {
			c.Next()
			return
		}
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		if _, ok := shared.GetUser(c); ok {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "unauthorized",
		})
	}
}

func isPublicPath(path string) bool {
	if _, ok := publicPaths[path]; ok {
		return true
	}
	return strings.HasPrefix(path, "/swagger/")
}

// CSRFProtect rejects unsafe (POST/PUT/DELETE/PATCH) requests authenticated
// via the session cookie unless they echo the CSRF cookie back as the
// X-CSRF-Token header. Other auth methods (API key Bearer, oauth2-proxy
// header) are not subject to CSRF; they don't ride on ambient cookies.
//
// SameSite=Lax already blocks the textbook cross-origin form-POST attack;
// this is defense-in-depth for shared-parent-domain deployments and any
// future GET endpoint that is upgraded to a mutation.
func (s *Service) CSRFProtect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.cfg == nil || !s.cfg.AuthEnabled {
			c.Next()
			return
		}
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		// CSRF only applies when the caller is authenticating via the session
		// cookie. Bearer tokens and proxy headers aren't replayable cross-site.
		if _, err := c.Cookie(oidchelper.SessionCookieName); err != nil {
			c.Next()
			return
		}
		cookie, err := c.Cookie(oidchelper.CSRFCookieName)
		header := c.GetHeader(oidchelper.CSRFHeaderName)
		if err != nil || cookie == "" || header == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "csrf token missing or invalid",
			})
			return
		}
		c.Next()
	}
}
