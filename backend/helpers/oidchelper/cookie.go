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
	"net/http"

	"github.com/gin-gonic/gin"
)

func writeCookie(c *gin.Context, cfg *Config, name, value string, maxAge int, httpOnly bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   cfg.CookieDomain,
		MaxAge:   maxAge,
		HttpOnly: httpOnly,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func SetSessionCookie(c *gin.Context, cfg *Config, jwt string) {
	writeCookie(c, cfg, SessionCookieName, jwt, int(cfg.SessionTTL.Seconds()), true)
}

func ClearSessionCookie(c *gin.Context, cfg *Config) {
	writeCookie(c, cfg, SessionCookieName, "", -1, true)
}

func SetStateCookie(c *gin.Context, cfg *Config, value string) {
	writeCookie(c, cfg, StateCookieName, value, int(StateCookieMaxAge.Seconds()), true)
}

func ClearStateCookie(c *gin.Context, cfg *Config) {
	writeCookie(c, cfg, StateCookieName, "", -1, true)
}

// SetCSRFCookie writes the CSRF token. Not HttpOnly so that JS can read
// it and echo the value back as the X-CSRF-Token header.
func SetCSRFCookie(c *gin.Context, cfg *Config, token string) {
	writeCookie(c, cfg, CSRFCookieName, token, int(cfg.SessionTTL.Seconds()), false)
}

func ClearCSRFCookie(c *gin.Context, cfg *Config) {
	writeCookie(c, cfg, CSRFCookieName, "", -1, false)
}
