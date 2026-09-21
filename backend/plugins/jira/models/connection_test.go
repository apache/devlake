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

package models

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

func TestGatewayEndpoint(t *testing.T) {
	jc := &JiraConn{CloudId: "abc-123"}
	assert.Equal(t, "https://api.atlassian.com/ex/jira/abc-123/rest/", jc.GatewayEndpoint())

	jc.CloudId = "  abc-123  "
	assert.Equal(t, "https://api.atlassian.com/ex/jira/abc-123/rest/", jc.GatewayEndpoint())

	jc.CloudId = ""
	assert.Equal(t, "", jc.GatewayEndpoint())

	jc.CloudId = "../evil"
	assert.Equal(t, "", jc.GatewayEndpoint())

	jc.CloudId = "foo/bar"
	assert.Equal(t, "", jc.GatewayEndpoint())

	jc.CloudId = "foo?x=1"
	assert.Equal(t, "", jc.GatewayEndpoint())
}

func TestApplyGatewayEndpoint(t *testing.T) {
	jc := &JiraConn{
		CloudId: "cloud-1",
	}
	jc.AuthMethod = AUTH_METHOD_OAUTH2
	jc.ApplyGatewayEndpoint()
	assert.Equal(t, "https://api.atlassian.com/ex/jira/cloud-1/rest/", jc.Endpoint)

	basic := &JiraConn{CloudId: "cloud-1"}
	basic.AuthMethod = "BasicAuth"
	basic.Endpoint = "https://example.atlassian.net/rest/"
	basic.ApplyGatewayEndpoint()
	assert.Equal(t, "https://example.atlassian.net/rest/", basic.Endpoint)
}

func TestValidateConnectionOAuth2(t *testing.T) {
	v := validator.New()
	jc := &JiraConn{
		ClientId:     "id",
		ClientSecret: "secret",
		CloudId:      "cloud-1",
	}
	jc.AuthMethod = AUTH_METHOD_OAUTH2

	err := jc.ValidateConnection(jc, v)
	require.NoError(t, err)
	assert.Equal(t, "https://api.atlassian.com/ex/jira/cloud-1/rest/", jc.Endpoint)

	missing := &JiraConn{CloudId: "cloud-1"}
	missing.AuthMethod = AUTH_METHOD_OAUTH2
	err = missing.ValidateConnection(missing, v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clientId, clientSecret and cloudId are required")

	invalid := &JiraConn{
		ClientId:     "id",
		ClientSecret: "secret",
		CloudId:      "../evil",
	}
	invalid.AuthMethod = AUTH_METHOD_OAUTH2
	err = invalid.ValidateConnection(invalid, v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cloudId must contain only letters, digits, and hyphens")
}

func TestValidateConnectionOAuth2DoesNotUseMultiAuthOneOf(t *testing.T) {
	v := validator.New()
	conn := &JiraConnection{}
	conn.Name = "jira-oauth"
	conn.AuthMethod = AUTH_METHOD_OAUTH2
	conn.ClientId = "id"
	conn.ClientSecret = "secret"
	conn.CloudId = "cloud-1"

	err := conn.CustomValidate(conn, v)
	require.NoError(t, err)
}

func TestSanitizeAndMergeFromRequestClientSecret(t *testing.T) {
	conn := &JiraConnection{}
	conn.AuthMethod = AUTH_METHOD_OAUTH2
	conn.ClientId = "id"
	conn.ClientSecret = "super-secret"
	conn.CloudId = "cloud-1"
	conn.ApplyGatewayEndpoint()

	sanitized := conn.Sanitize()
	assert.Equal(t, utils.SanitizeString("super-secret"), sanitized.ClientSecret)
	assert.Empty(t, sanitized.OAuthAccessToken())

	err := conn.MergeFromRequest(conn, map[string]interface{}{
		"authMethod":   AUTH_METHOD_OAUTH2,
		"clientId":     "id",
		"clientSecret": utils.SanitizeString("super-secret"),
		"cloudId":      "cloud-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "super-secret", conn.ClientSecret)
	assert.Equal(t, "https://api.atlassian.com/ex/jira/cloud-1/rest/", conn.Endpoint)
}

func TestSetupAuthenticationOAuth2(t *testing.T) {
	jc := &JiraConn{}
	jc.AuthMethod = AUTH_METHOD_OAUTH2
	jc.SetOAuthAccessToken("access-token", time.Now().Add(time.Hour))

	req := httptest.NewRequest(http.MethodGet, "https://api.atlassian.com/ex/jira/c/rest/api/2/serverInfo", nil)
	err := jc.SetupAuthentication(req)
	require.NoError(t, err)
	assert.Equal(t, "Bearer access-token", req.Header.Get("Authorization"))
}

func TestSetupAuthenticationOAuth2MissingToken(t *testing.T) {
	jc := &JiraConn{}
	jc.AuthMethod = AUTH_METHOD_OAUTH2
	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	err := jc.SetupAuthentication(req)
	require.Error(t, err)
}

func TestValidateConnectionBasicAuthStillWorks(t *testing.T) {
	v := validator.New()
	jc := &JiraConn{
		RestConnection: helper.RestConnection{Endpoint: "https://example.atlassian.net/rest/"},
		BasicAuth:      helper.BasicAuth{Username: "user@example.com", Password: "token"},
	}
	jc.AuthMethod = "BasicAuth"
	err := jc.ValidateConnection(jc, v)
	require.NoError(t, err)
}
