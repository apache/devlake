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
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

const (
	// AUTH_METHOD_OAUTH2 is Jira Cloud service-account OAuth 2.0 (client credentials / 2LO).
	// It is plugin-specific and is not registered in core MultiAuth.
	AUTH_METHOD_OAUTH2 = "OAuth2"
)

// cloudIDPattern allows Atlassian Cloud IDs (UUIDs) and test fixtures such as "cloud-1".
// It rejects path separators and URL metacharacters so CloudId cannot change the gateway path.
var cloudIDPattern = regexp.MustCompile(`^[a-zA-Z0-9-]{1,64}$`)

type EpicResponse struct {
	Id    int
	Title string
	Value string
}

type BoardResponse struct {
	Id    int
	Title string
	Value string
}

// JiraConn holds the essential information to connect to the Jira API
type JiraConn struct {
	helper.RestConnection `mapstructure:",squash"`
	helper.MultiAuth      `mapstructure:",squash"`
	helper.BasicAuth      `mapstructure:",squash"`
	helper.AccessToken    `mapstructure:",squash"`

	ClientId     string `mapstructure:"clientId" json:"clientId" gorm:"type:varchar(255)"`
	ClientSecret string `mapstructure:"clientSecret" json:"clientSecret" gorm:"type:text;serializer:encdec"`
	CloudId      string `mapstructure:"cloudId" json:"cloudId" gorm:"type:varchar(255)"`

	// OAuthTokenURL overrides the Atlassian token endpoint (tests only).
	OAuthTokenURL string `json:"-" mapstructure:"-" gorm:"-"`

	oauthToken          string
	oauthTokenExpiresAt *time.Time
}

func (jc *JiraConn) Sanitize() JiraConn {
	jc.Password = ""
	jc.AccessToken.Token = utils.SanitizeString(jc.AccessToken.Token)
	jc.ClientSecret = utils.SanitizeString(jc.ClientSecret)
	jc.oauthToken = ""
	jc.oauthTokenExpiresAt = nil
	return *jc
}

func (jc *JiraConn) IsOAuth2() bool {
	return jc.AuthMethod == AUTH_METHOD_OAUTH2
}

func sanitizedCloudID(cloudId string) string {
	cloudId = strings.TrimSpace(cloudId)
	if !cloudIDPattern.MatchString(cloudId) {
		return ""
	}
	return cloudId
}

// GatewayEndpoint returns the Atlassian API gateway base URL for this cloud ID.
func (jc *JiraConn) GatewayEndpoint() string {
	cloudId := sanitizedCloudID(jc.CloudId)
	if cloudId == "" {
		return ""
	}
	return fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/", cloudId)
}

// ApplyGatewayEndpoint sets Endpoint to the OAuth 2.0 gateway URL.
func (jc *JiraConn) ApplyGatewayEndpoint() {
	if !jc.IsOAuth2() {
		return
	}
	if endpoint := jc.GatewayEndpoint(); endpoint != "" {
		jc.Endpoint = endpoint
	}
}

func (jc *JiraConn) OAuthAccessToken() string {
	return jc.oauthToken
}

func (jc *JiraConn) OAuthAccessTokenExpiresAt() *time.Time {
	return jc.oauthTokenExpiresAt
}

func (jc *JiraConn) SetOAuthAccessToken(token string, expiresAt time.Time) {
	jc.oauthToken = token
	expiry := expiresAt
	jc.oauthTokenExpiresAt = &expiry
}

// SetupAuthentication implements the `IAuthentication` interface by delegating
// the actual logic to the `MultiAuth` struct to help us write less code
func (jc *JiraConn) SetupAuthentication(req *http.Request) errors.Error {
	if jc.IsOAuth2() {
		token := jc.OAuthAccessToken()
		if token == "" {
			return errors.Unauthorized.New("oauth2 access token is missing")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
	return jc.MultiAuth.SetupAuthenticationForConnection(jc, req)
}

// ValidateConnection shadows MultiAuth so authMethod=OAuth2 is not rejected by
// the shared oneof=BasicAuth AccessToken AppKey constraint.
func (jc *JiraConn) ValidateConnection(connection interface{}, v *validator.Validate) errors.Error {
	if jc.IsOAuth2() {
		if strings.TrimSpace(jc.ClientId) == "" || strings.TrimSpace(jc.ClientSecret) == "" || strings.TrimSpace(jc.CloudId) == "" {
			return errors.BadInput.New("clientId, clientSecret and cloudId are required for OAuth2")
		}
		if sanitizedCloudID(jc.CloudId) == "" {
			return errors.BadInput.New("cloudId must contain only letters, digits, and hyphens")
		}
		jc.ApplyGatewayEndpoint()
		if conn, ok := connection.(*JiraConnection); ok && strings.TrimSpace(conn.Name) == "" {
			return errors.BadInput.New("name is required")
		}
		return nil
	}
	return jc.MultiAuth.ValidateConnection(connection, v)
}

var _ plugin.PrepareApiClient = (*JiraConn)(nil)

// JiraConnection holds JiraConn plus ID/Name for database storage
type JiraConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	JiraConn              `mapstructure:",squash"`
}

func (JiraConnection) TableName() string {
	return "_tool_jira_connections"
}

func (connection *JiraConnection) CustomValidate(entity interface{}, v *validator.Validate) errors.Error {
	return connection.JiraConn.ValidateConnection(entity, v)
}

func (connection *JiraConnection) MergeFromRequest(target *JiraConnection, body map[string]interface{}) error {
	token := target.Token
	password := target.Password
	clientSecret := target.ClientSecret
	authMethod := target.AuthMethod

	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}

	modifiedToken := target.Token
	modifiedPassword := target.Password
	modifiedClientSecret := target.ClientSecret
	modifiedAuthMethod := target.AuthMethod

	// maybe auth method has changed
	if authMethod == modifiedAuthMethod {
		if modifiedToken == "" || modifiedToken == utils.SanitizeString(token) {
			target.Token = token
		}
		if modifiedPassword == "" || modifiedPassword == utils.SanitizeString(password) {
			target.Password = password
		}
		if modifiedClientSecret == "" || modifiedClientSecret == utils.SanitizeString(clientSecret) {
			target.ClientSecret = clientSecret
		}
	}

	if target.IsOAuth2() {
		target.ApplyGatewayEndpoint()
	}

	return nil
}

func (connection JiraConnection) Sanitize() JiraConnection {
	connection.JiraConn = connection.JiraConn.Sanitize()
	return connection
}
