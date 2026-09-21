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
	"encoding/base64"
	"testing"

	"github.com/spf13/viper"
)

func basicAuthHeader(username, password string) map[string]string {
	return map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password)),
	}
}

func TestOAuth2ProxyAuthenticationIgnoresBasicAuthWhenAuthEnabled(t *testing.T) {
	cfg := viper.New()
	cfg.Set("AUTH_ENABLED", true)
	router := newProxyAuthRouterWithConfig(cfg)
	body := performProxyAuthRequest(t, router, basicAuthHeader("anyone", "anything"))
	if body.Authenticated {
		t.Fatalf("expected Basic auth header to be ignored with AUTH_ENABLED=true, got %+v", body)
	}
}

func TestOAuth2ProxyAuthenticationTrustsBasicAuthWhenAuthDisabled(t *testing.T) {
	cases := map[string]func(*viper.Viper){
		"AUTH_ENABLED unset":    func(*viper.Viper) {},
		"AUTH_ENABLED explicit": func(cfg *viper.Viper) { cfg.Set("AUTH_ENABLED", false) },
	}
	for name, configure := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := viper.New()
			configure(cfg)
			router := newProxyAuthRouterWithConfig(cfg)
			body := performProxyAuthRequest(t, router, basicAuthHeader("admin", "secret"))
			if !body.Authenticated || body.Name != "admin" {
				t.Fatalf("expected Basic auth user to be kept without AUTH_ENABLED, got %+v", body)
			}
		})
	}
}
