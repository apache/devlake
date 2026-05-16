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
	"reflect"
	"testing"

	"github.com/spf13/viper"

	"github.com/apache/incubator-devlake/core/config"
	corectx "github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/log"
)

func TestParseScopes(t *testing.T) {
	cases := map[string][]string{
		"":                           {"openid", "profile", "email"},
		"   ":                        {"openid", "profile", "email"},
		"openid":                     {"openid"},
		"openid,profile":             {"openid", "profile"},
		" openid , profile , email ": {"openid", "profile", "email"},
		"openid,,email":              {"openid", "email"},
	}
	for input, want := range cases {
		got := parseScopes(input)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseScopes(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestValueOr(t *testing.T) {
	if got := valueOr("hello", "fallback"); got != "hello" {
		t.Errorf("valueOr(hello) = %q", got)
	}
	if got := valueOr("", "fallback"); got != "fallback" {
		t.Errorf("valueOr(empty) = %q", got)
	}
	if got := valueOr("   ", "fallback"); got != "fallback" {
		t.Errorf("valueOr(whitespace) = %q", got)
	}
}

func TestParseProviderNames(t *testing.T) {
	cases := map[string][]string{
		"":                {},
		"  ":              {},
		"entra":           {"entra"},
		"entra,google":    {"entra", "google"},
		" Entra , GOOGLE": {"entra", "google"},
		"entra,,google":   {"entra", "google"},
		"entra,entra":     {"entra"},
		"Entra,entra":     {"entra"},
	}
	for input, want := range cases {
		got := parseProviderNames(input)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseProviderNames(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestProviderNamesSorted(t *testing.T) {
	c := &Config{
		Providers: map[string]*ProviderConfig{
			"google": {Name: "google"},
			"entra":  {Name: "entra"},
		},
	}
	if names := c.ProviderNames(); !reflect.DeepEqual(names, []string{"entra", "google"}) {
		t.Errorf("ProviderNames = %v, want sorted [entra google]", names)
	}
}

type basicResStub struct {
	cfg config.ConfigReader
}

func (b basicResStub) GetConfigReader() config.ConfigReader { return b.cfg }
func (b basicResStub) GetConfig(string) string              { return "" }
func (b basicResStub) GetLogger() log.Logger                { return nil }
func (b basicResStub) NestedLogger(string) corectx.BasicRes { return nil }
func (b basicResStub) ReplaceLogger(log.Logger) corectx.BasicRes {
	return nil
}
func (b basicResStub) GetDal() dal.Dal { return nil }

func TestLoadConfigDefaultsAuthEnabled(t *testing.T) {
	v := viper.New()

	cfg, err := LoadConfig(basicResStub{cfg: v})
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if !cfg.AuthEnabled {
		t.Fatal("AuthEnabled should default to true when AUTH_ENABLED is unset")
	}
	if cfg.OIDCEnabled {
		t.Fatal("OIDCEnabled should default to false")
	}
	if len(cfg.SessionSecret) != 0 {
		t.Fatalf("SessionSecret = %q, want empty when OIDC is disabled", string(cfg.SessionSecret))
	}
}

func TestLoadConfigRequiresSessionSecretForOIDC(t *testing.T) {
	v := viper.New()
	v.Set("OIDC_ENABLED", true)

	if _, err := LoadConfig(basicResStub{cfg: v}); err == nil {
		t.Fatal("LoadConfig should reject OIDC-enabled config without SESSION_SECRET")
	}
}
