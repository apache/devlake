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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/apache/incubator-devlake/core/config"
	corecontext "github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/impls/logruslog"
	"github.com/apache/incubator-devlake/server/api/shared"
)

type proxyAuthTestBasicRes struct {
	cfg    config.ConfigReader
	logger log.Logger
}

func (b *proxyAuthTestBasicRes) GetConfigReader() config.ConfigReader { return b.cfg }
func (b *proxyAuthTestBasicRes) GetConfig(name string) string         { return b.cfg.GetString(name) }
func (b *proxyAuthTestBasicRes) GetLogger() log.Logger                { return b.logger }
func (b *proxyAuthTestBasicRes) NestedLogger(name string) corecontext.BasicRes {
	return &proxyAuthTestBasicRes{cfg: b.cfg, logger: b.logger.Nested(name)}
}
func (b *proxyAuthTestBasicRes) ReplaceLogger(logger log.Logger) corecontext.BasicRes {
	return &proxyAuthTestBasicRes{cfg: b.cfg, logger: logger}
}
func (b *proxyAuthTestBasicRes) GetDal() dal.Dal { return nil }

type proxyAuthResponse struct {
	Authenticated bool   `json:"authenticated"`
	Name          string `json:"name"`
	Email         string `json:"email"`
}

func newProxyAuthRouter(secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := viper.New()
	cfg.Set("FORWARDED_USER_SECRET", secret)
	basicRes := &proxyAuthTestBasicRes{
		cfg:    cfg,
		logger: logruslog.Global,
	}
	r := gin.New()
	r.Use(OAuth2ProxyAuthentication(basicRes))
	r.GET("/me", func(c *gin.Context) {
		user, ok := shared.GetUser(c)
		if !ok {
			c.JSON(http.StatusOK, proxyAuthResponse{})
			return
		}
		c.JSON(http.StatusOK, proxyAuthResponse{
			Authenticated: true,
			Name:          user.Name,
			Email:         user.Email,
		})
	})
	return r
}

func performProxyAuthRequest(t *testing.T, router *gin.Engine, headers map[string]string) proxyAuthResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxyAuthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}

func TestOAuth2ProxyAuthenticationRejectsUntrustedForwardedHeaders(t *testing.T) {
	router := newProxyAuthRouter("shared-secret")
	cases := map[string]map[string]string{
		"missing secret header": {
			forwardedUserHeader:  "admin@example.com",
			forwardedEmailHeader: "admin@example.com",
		},
		"mismatched secret header": {
			forwardedUserHeader:       "admin@example.com",
			forwardedEmailHeader:      "admin@example.com",
			forwardedUserSecretHeader: "wrong-secret",
		},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			body := performProxyAuthRequest(t, router, headers)
			if body.Authenticated {
				t.Fatalf("expected forwarded headers to be rejected, got %+v", body)
			}
		})
	}
}

func TestOAuth2ProxyAuthenticationRequiresConfiguredSecret(t *testing.T) {
	router := newProxyAuthRouter("")
	body := performProxyAuthRequest(t, router, map[string]string{
		forwardedUserHeader:       "admin@example.com",
		forwardedEmailHeader:      "admin@example.com",
		forwardedUserSecretHeader: "shared-secret",
	})
	if body.Authenticated {
		t.Fatalf("expected forwarded headers to be ignored without FORWARDED_USER_SECRET, got %+v", body)
	}
}

func TestOAuth2ProxyAuthenticationAcceptsTrustedForwardedHeaders(t *testing.T) {
	router := newProxyAuthRouter("shared-secret")
	body := performProxyAuthRequest(t, router, map[string]string{
		forwardedUserHeader:       "admin@example.com",
		forwardedEmailHeader:      "admin@example.com",
		forwardedUserSecretHeader: "shared-secret",
	})
	if !body.Authenticated || body.Name != "admin@example.com" || body.Email != "admin@example.com" {
		t.Fatalf("expected trusted forwarded user, got %+v", body)
	}
}
