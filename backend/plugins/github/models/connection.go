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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/utils"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessToken = "AccessToken"
	AppKey      = "AppKey"
)

// GithubAccessToken supports fetching data with multiple tokens
type GithubAccessToken struct {
	helper.AccessToken `mapstructure:",squash"`
	tokens             []string `gorm:"-" json:"-" mapstructure:"-"`
	tokenIndex         int      `gorm:"-" json:"-" mapstructure:"-"`
}

type GithubAppKey struct {
	helper.AppKey  `mapstructure:",squash"`
	InstallationID int `mapstructure:"installationId" validate:"required" json:"installationId"`
}

// GithubConn holds the essential information to connect to the GitHub API
type GithubConn struct {
	helper.RestConnection `mapstructure:",squash"`
	helper.MultiAuth      `mapstructure:",squash"`
	GithubAccessToken     `mapstructure:",squash" authMethod:"AccessToken"`
	GithubAppKey          `mapstructure:",squash" authMethod:"AppKey"`
	RefreshToken          string     `mapstructure:"refreshToken" json:"refreshToken" gorm:"type:text;serializer:encdec"`
	TokenExpiresAt        *time.Time `mapstructure:"tokenExpiresAt" json:"tokenExpiresAt"`
	RefreshTokenExpiresAt *time.Time `mapstructure:"refreshTokenExpiresAt" json:"refreshTokenExpiresAt"`
}

// UpdateToken updates the token and refresh token information
func (conn *GithubConn) UpdateToken(newToken, newRefreshToken string, expiry, refreshExpiry *time.Time) {
	conn.Token = newToken
	conn.RefreshToken = newRefreshToken
	conn.TokenExpiresAt = expiry
	conn.RefreshTokenExpiresAt = refreshExpiry

	// Update the internal tokens slice used by SetupAuthentication
	conn.tokens = []string{newToken}
	conn.tokenIndex = 0
}

// PrepareApiClient splits Token to tokens for SetupAuthentication to utilize
func (conn *GithubConn) PrepareApiClient(apiClient plugin.ApiClient) errors.Error {

	if conn.AuthMethod == AccessToken {
		conn.tokens = strings.Split(conn.Token, ",")
	}

	if conn.AuthMethod == AppKey && conn.InstallationID != 0 {
		token, err := conn.GetInstallationAccessToken(apiClient)
		if err != nil {
			return err
		}
		var expiresAt *time.Time
		if !token.ExpiresAt.IsZero() {
			expiresAt = &token.ExpiresAt
		}
		conn.UpdateToken(token.Token, "", expiresAt, nil)
	}

	return nil
}

// SetupAuthentication sets up the HTTP Request Authentication
func (conn *GithubConn) SetupAuthentication(req *http.Request) errors.Error {
	// Rotates token on each request.
	if len(conn.tokens) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", conn.tokens[conn.tokenIndex]))
		// Set next token index
		conn.tokenIndex = (conn.tokenIndex + 1) % len(conn.tokens)
	}

	return nil
}

func (gat *GithubAccessToken) GetTokensCount() int {
	return len(gat.tokens)
}

// GithubConnection holds GithubConn plus ID/Name for database storage
type GithubConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	GithubConn            `mapstructure:",squash"`
	EnableGraphql         bool `mapstructure:"enableGraphql" json:"enableGraphql"`
}

const (
	GithubTokenTypeClassical                = "classical"
	GithubTokenTypeClassicalPrefixLen       = 4
	GithubTokenTypeClassicalShowPrefixLen   = 8
	GithubTokenTypeClassicalHiddenLen       = 20
	GithubTokenTypeFineGrained              = "fine-grained"
	GithubTokenTypeFineGrainedPrefixLen     = 11
	GithubTokenTypeFineGrainedShowPrefixLen = 8
	GithubTokenTypeFineGrainedHiddenLen     = 66
	GithubTokenTypeUnknown                  = "unknown"
)

func (connection GithubConnection) TableName() string {
	return "_tool_github_connections"
}

func (connection GithubConnection) GetHash() string {
	if connection.AuthMethod == AppKey {
		// GitHub App installation tokens expire after ~1 hour, disable API client caching
		// to ensure fresh tokens are fetched for admin APIs (remote-scopes, test-connection)
		return ""
	}
	// Use default caching for PAT connections (they don't expire)
	return connection.BaseConnection.GetHash()
}

func (connection *GithubConnection) MergeFromRequest(target *GithubConnection, body map[string]interface{}) error {
	modifiedConnection := GithubConnection{}
	if err := helper.DecodeMapStruct(body, &modifiedConnection, true); err != nil {
		return err
	}
	return connection.Merge(target, &modifiedConnection, body)
}

func (connection *GithubConnection) Merge(existed, modified *GithubConnection, body map[string]interface{}) error {
	// There are many kinds of update, we just update all fields simply.
	existedTokenStr := existed.Token
	existSecretKey := existed.SecretKey

	existed.Name = modified.Name
	if _, ok := body["enableGraphql"]; ok {
		existed.EnableGraphql = modified.EnableGraphql
	}
	existed.AppId = modified.AppId
	existed.SecretKey = modified.SecretKey
	existed.InstallationID = modified.InstallationID
	existed.AuthMethod = modified.AuthMethod
	existed.Proxy = modified.Proxy
	existed.Endpoint = modified.Endpoint
	existed.RateLimitPerHour = modified.RateLimitPerHour

	// handle secret
	if existSecretKey == "" {
		if modified.SecretKey != "" {
			// add secret key, store it
			existed.SecretKey = modified.SecretKey
		}
		// doesn't input secret key, pass
	} else {
		if modified.SecretKey == "" {
			// delete secret key
			existed.SecretKey = modified.SecretKey
		} else {
			// update secret key
			sanitizeSecretKey := existed.SanitizeSecret().SecretKey
			if sanitizeSecretKey == modified.SecretKey {
				// change nothing, restore it
				existed.SecretKey = existSecretKey
			} else {
				// has changed, replace it with the new secret key
				existed.SecretKey = modified.SecretKey
			}
		}
	}

	// handle tokens
	existedTokens := strings.Split(strings.TrimSpace(existedTokenStr), ",")
	existedTokenMap := make(map[string]string)          // {originalToken:sanitizedToken}
	existedSanitizedTokenMap := make(map[string]string) // {sanitizedToken:originalToken}
	for _, token := range existedTokens {
		existedTokenMap[token] = existed.SanitizeToken(token)
		existedSanitizedTokenMap[existed.SanitizeToken(token)] = token
	}

	modifiedTokens := strings.Split(strings.TrimSpace(modified.Token), ",")
	modifiedTokenMap := make(map[string]string) // {originalToken:sanitizedToken}
	for _, token := range modifiedTokens {
		modifiedTokenMap[token] = existed.SanitizeToken(token)
	}

	var mergedToken []string
	mergedTokenMap := make(map[string]struct{})

	for token, sanitizeToken := range modifiedTokenMap {
		// check token
		if _, ok := existedTokenMap[token]; ok {
			// find in db, modified but no update, ignore it
			if _, ok := mergedTokenMap[token]; !ok {
				mergedToken = append(mergedToken, token)
				mergedTokenMap[token] = struct{}{}
			}
		} else {
			// not found, a new token, we should keep it
			// cannot be a sanitized token
			if _, ok := existedSanitizedTokenMap[token]; !ok {
				if token != sanitizeToken {
					if _, ok := mergedTokenMap[token]; !ok {
						mergedToken = append(mergedToken, token)
						mergedTokenMap[token] = struct{}{}
					}
				}
			}
		}

		// token may be a sanitized token
		if v, ok := existedSanitizedTokenMap[token]; ok {
			// find in db, modify nothing, just keep it
			if _, ok := mergedTokenMap[v]; !ok {
				mergedToken = append(mergedToken, v)
				mergedTokenMap[v] = struct{}{}
			}
		} else {
			// unexpected
			fmt.Printf("unexpected token: %+v will be ignored\n", token)
		}
		// check sanitized token
		if v, ok := existedSanitizedTokenMap[sanitizeToken]; ok {
			// find in db, modify nothing, just keep it
			if _, ok := mergedTokenMap[v]; !ok {
				mergedToken = append(mergedToken, v)
				mergedTokenMap[v] = struct{}{}
			}
		} else {
			// a new token
			// but we should check it
			if sanitizeToken != token {
				if _, ok := mergedTokenMap[token]; !ok {
					mergedToken = append(mergedToken, token)
					mergedTokenMap[token] = struct{}{}
				}
			}
		}
	}

	existed.Token = strings.Join(mergedToken, ",")
	return nil
}

func (conn *GithubConn) typeIs(token string) string {
	// classical tokens:
	// ghp_ for Personal Access Tokens
	// gho_ for OAuth Access tokens
	// ghu_ for GitHub App user-to-server tokens
	// ghs_ for GitHub App server-to-server tokens
	// ghr_ for GitHub App refresh tokens
	// total len is 40, {prefix}{showPrefix}{secret}{showSuffix}
	// fine-grained tokens
	// github_pat_{82_characters}
	classicalTokenClassicalPrefixes := []string{"ghp_", "gho_", "ghs_", "ghr_", "ghu_"}
	classicalTokenFindGrainedPrefixes := []string{"github_pat_"}
	for _, prefix := range classicalTokenClassicalPrefixes {
		if strings.HasPrefix(token, prefix) {
			return GithubTokenTypeClassical
		}
	}
	for _, prefix := range classicalTokenFindGrainedPrefixes {
		if strings.HasPrefix(token, prefix) {
			return GithubTokenTypeFineGrained
		}
	}
	return GithubTokenTypeUnknown
}

func (connection GithubConnection) Sanitize() GithubConnection {
	connection.GithubConn = connection.GithubConn.Sanitize()
	return connection
}

func (conn *GithubConn) Sanitize() GithubConn {
	conn.SanitizeTokens()
	conn.SanitizeSecret()
	return *conn
}

func (conn *GithubConn) SanitizeSecret() GithubConn {
	if conn.SecretKey == "" {
		return *conn
	}
	secretKey := conn.SecretKey
	showPrefixLen, showSuffixLen := 50, 50
	hiddenLen := len(secretKey) - showSuffixLen - showSuffixLen
	secret := strings.Repeat("*", hiddenLen)
	conn.SecretKey = strings.Replace(secretKey, conn.SecretKey[showPrefixLen:showPrefixLen+hiddenLen], secret, -1)
	return *conn
}

func (conn *GithubConn) SanitizeToken(token string) string {
	if token == "" {
		return token
	}
	sanitizedToken := token
	var prefixLen, showPrefixLen, hiddenLen int

	tokenType := conn.typeIs(token)
	switch tokenType {
	case GithubTokenTypeClassical:
		prefixLen, showPrefixLen, hiddenLen = GithubTokenTypeClassicalPrefixLen, GithubTokenTypeClassicalShowPrefixLen, GithubTokenTypeClassicalHiddenLen
	case GithubTokenTypeFineGrained:
		prefixLen, showPrefixLen, hiddenLen = GithubTokenTypeFineGrainedPrefixLen, GithubTokenTypeFineGrainedShowPrefixLen, GithubTokenTypeFineGrainedHiddenLen
	case GithubTokenTypeUnknown:
		return utils.SanitizeString(token)
	}
	tokenLen := len(token)
	if tokenLen >= prefixLen && prefixLen != 0 && hiddenLen != 0 {
		secret := strings.Repeat("*", hiddenLen)
		sanitizedToken = strings.Replace(token, token[prefixLen+showPrefixLen:prefixLen+showPrefixLen+hiddenLen], secret, -1)
	}
	return sanitizedToken
}

func (conn *GithubConn) SanitizeTokens() GithubConn {
	if conn.Token == "" {
		return *conn
	}
	tokens := strings.Split(conn.Token, ",")
	var sanitizedTokens []string
	for _, token := range tokens {
		if token == "" {
			continue
		}
		sanitizedToken := conn.SanitizeToken(token)
		sanitizedTokens = append(sanitizedTokens, sanitizedToken)
	}

	if len(sanitizedTokens) > 0 {
		conn.Token = strings.Join(sanitizedTokens, ",")
	} else {
		conn.Token = ""
	}
	return *conn
}

// Using GithubUserOfToken because it requires authentication, and it is public information anyway.
type GithubUserOfToken struct {
	Login string `json:"login"`
}

type InstallationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type GithubApp struct {
	ID   int32  `json:"id"`
	Slug string `json:"slug"`
}

type GithubAppInstallation struct {
	Id      int `json:"id"`
	Account struct {
		Login string `json:"login"`
	} `json:"account"`
}

type GithubAppInstallationWithToken struct {
	GithubAppInstallation
	Token string
}

func (gak *GithubAppKey) CreateJwt() (string, errors.Error) {
	token := jwt.New(jwt.SigningMethodRS256)
	t := time.Now().Unix()

	token.Claims = jwt.MapClaims{
		"iat": t,
		"exp": t + (10 * 60),
		"iss": gak.AppId,
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(gak.SecretKey))
	if err != nil {
		return "", errors.BadInput.Wrap(err, "invalid private key")
	}

	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", errors.BadInput.Wrap(err, "invalid private key")
	}

	return tokenString, nil
}

func (gak *GithubAppKey) GetInstallationAccessToken(
	apiClient plugin.ApiClient,
) (*InstallationToken, errors.Error) {

	jwt, err := gak.CreateJwt()
	if err != nil {
		return nil, err
	}

	resp, err := apiClient.Post(fmt.Sprintf("/app/installations/%d/access_tokens", gak.InstallationID), nil, nil, http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer %s", jwt)},
	})
	if err != nil {
		return nil, err
	}

	body, err := errors.Convert01(io.ReadAll(resp.Body))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, errors.HttpStatus(resp.StatusCode).New(fmt.Sprintf("unexpected status code while getting installation access token: %s", string(body)))
	}

	var installationToken InstallationToken
	err = errors.Convert(json.Unmarshal(body, &installationToken))
	if err != nil {
		return nil, err
	}
	if installationToken.Token == "" {
		return nil, errors.Default.New("empty installation access token returned")
	}

	return &installationToken, nil
}
