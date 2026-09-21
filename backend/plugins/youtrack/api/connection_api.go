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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/apache/devlake/server/api/shared"
)

// endpointExample is the canonical endpoint shape shown in validation errors.
const endpointExample = "https://example.myjetbrains.com/youtrack/api"

// validateEndpoint normalizes the endpoint (trailing slashes are trimmed) and
// enforces the one hard rule about YouTrack base URLs: the user enters the
// full API base URL and it must end in `/api`. The endpoint is never inferred
// from the hostname — self-hosted deployments are genuinely unconstrained
// (both `https://host/youtrack/api` and `https://youtrack.host/api` are valid
// shapes), so guessing is impossible in general.
func validateEndpoint(endpoint string) (string, errors.Error) {
	normalized := strings.TrimRight(endpoint, "/")
	reject := func() (string, errors.Error) {
		return "", errors.BadInput.New(fmt.Sprintf(
			"endpoint must be the full API base URL ending in `/api` — e.g. `%s`", endpointExample))
	}
	u, err := url.Parse(normalized)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return reject()
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return reject()
	}
	if !strings.HasSuffix(u.Path, "/api") {
		return reject()
	}
	return normalized, nil
}

// youtrackUser mirrors the `users/me` response fields the connection test
// reads (requested via ?fields=id,login,fullName,guest).
type youtrackUser struct {
	Id       string `json:"id"`
	Login    string `json:"login"`
	FullName string `json:"fullName"`
	Guest    bool   `json:"guest"`
}

// checkAuthenticated asserts the token was actually accepted: a bare 2xx
// proves nothing because YouTrack silently degrades to the guest user when
// the Authorization header is missing or not accepted.
func checkAuthenticated(user youtrackUser) errors.Error {
	if user.Guest {
		return errors.HttpStatus(http.StatusUnauthorized).New(
			"authenticated as guest — the token was not accepted; check that you pasted a permanent token")
	}
	return nil
}

// diagnoseTestFailure maps a non-200 `users/me` response to an explicit,
// actionable failure. The InCloud special case exists because a missing
// `/youtrack` path prefix is the most common setup error on
// `*.myjetbrains.com` — the API root answers 404 there, not a redirect.
func diagnoseTestFailure(endpoint string, statusCode int) errors.Error {
	switch statusCode {
	case http.StatusUnauthorized:
		return errors.HttpStatus(http.StatusBadRequest).New("authentication failed, please check your token")
	case http.StatusNotFound:
		if missingInCloudPrefix(endpoint) {
			u, _ := url.Parse(endpoint)
			return errors.HttpStatus(http.StatusBadRequest).New(fmt.Sprintf(
				"InCloud instances serve the API under the `/youtrack` path prefix — your endpoint likely should be `https://%s/youtrack/api`",
				u.Host))
		}
		return errors.HttpStatus(http.StatusBadRequest).New("not found — check the base URL (it must end in `/api`)")
	default:
		return errors.HttpStatus(statusCode).New(fmt.Sprintf(
			"unexpected status code %d while testing connection", statusCode))
	}
}

// missingInCloudPrefix reports whether the endpoint is an InCloud host whose
// path lacks the `/youtrack` context path. Detection is by hostname pattern
// only — no auto-retry with a guessed URL, guessing was rejected by design.
func missingInCloudPrefix(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(u.Hostname()), ".myjetbrains.com") &&
		!strings.HasPrefix(u.Path, "/youtrack/")
}

// permanentTokenWarning returns a soft warning when the token does not carry
// the `perm:`/`perm-` prefix of a YouTrack permanent token. It never blocks:
// newer token shapes may differ, so a mismatch is worth noticing, not failing.
func permanentTokenWarning(token string) string {
	if strings.HasPrefix(token, "perm:") || strings.HasPrefix(token, "perm-") {
		return ""
	}
	return "warning: the token does not look like a permanent token (those start with `perm:`) — it may still work, but check that you pasted the right token"
}

// successMessage composes the test-connection success message: it names the
// token's login (so a wrong-account paste is visible), reports the server
// version when the best-effort probe returned one, and appends the soft
// token-shape warning when the token lacks the permanent-token prefix.
func successMessage(login, version, build, token string) string {
	message := fmt.Sprintf("success: authenticated as %s", login)
	if version != "" {
		message += fmt.Sprintf(" — YouTrack %s (build %s)", version, build)
	}
	if warning := permanentTokenWarning(token); warning != "" {
		message += " " + warning
	}
	return message
}

// defaultRateLimitPerHour is the connection-level default applied at creation
// when the request carries 0. No documented YouTrack limits exist; the
// observed failure mode is 504 on expensive queries, so the policy is
// generous-but-bounded and user-configurable.
const defaultRateLimitPerHour = 10000

// youtrackServerConfig mirrors the fields read by the best-effort version
// probe (`GET {endpoint}/config?fields=version,build`).
type youtrackServerConfig struct {
	Version string `json:"version"`
	Build   string `json:"build"`
}

type YoutrackTestConnResponse struct {
	shared.ApiBody
	Connection *models.YoutrackConn
}

func testConnection(ctx context.Context, connection models.YoutrackConn) (*YoutrackTestConnResponse, errors.Error) {
	if vld != nil {
		if err := vld.Struct(connection); err != nil {
			return nil, errors.Default.Wrap(err, "error validating target")
		}
	}
	// there is no default endpoint: the URL shape rule is enforced on test too
	endpoint, err := validateEndpoint(connection.Endpoint)
	if err != nil {
		return nil, err
	}
	connection.Endpoint = endpoint
	apiClient, err := helper.NewApiClientFromConnection(ctx, basicRes, &connection)
	if err != nil {
		return nil, err
	}
	res, err := apiClient.Get("users/me", url.Values{"fields": []string{"id,login,fullName,guest"}}, nil)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "error checking YouTrack connection")
	}
	if res.StatusCode != http.StatusOK {
		return nil, diagnoseTestFailure(connection.Endpoint, res.StatusCode)
	}
	var user youtrackUser
	if err := helper.UnmarshalResponse(res, &user); err != nil {
		return nil, err
	}
	// a bare 2xx proves nothing: missing auth silently degrades to guest
	if err := checkAuthenticated(user); err != nil {
		return nil, err
	}
	version, build := probeServerVersion(apiClient)
	message := successMessage(user.Login, version, build, connection.Token)
	connection = connection.Sanitize()
	body := YoutrackTestConnResponse{}
	body.Success = true
	body.Message = message
	body.Connection = &connection
	return &body, nil
}

// probeServerVersion makes a best-effort `GET {endpoint}/config` to report the
// server version in the success message. The endpoint is undocumented, so any
// failure (404 included) degrades silently to empty strings.
func probeServerVersion(apiClient plugin.ApiClient) (version, build string) {
	res, err := apiClient.Get("config", url.Values{"fields": []string{"version,build"}}, nil)
	if err != nil || res.StatusCode != http.StatusOK {
		return "", ""
	}
	var config youtrackServerConfig
	if err := helper.UnmarshalResponse(res, &config); err != nil {
		return "", ""
	}
	return config.Version, config.Build
}

// TestConnection test youtrack connection
// @Summary test youtrack connection
// @Description Test YouTrack Connection
// @Tags plugins/youtrack
// @Param body body models.YoutrackConn true "json body"
// @Success 200  {object} YoutrackTestConnResponse "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/youtrack/test [POST]
func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var connection models.YoutrackConn
	if err := helper.Decode(input.Body, &connection, vld); err != nil {
		return nil, err
	}
	result, err := testConnection(context.TODO(), connection)
	if err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}
	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}

// TestExistingConnection test youtrack connection by ID
// @Summary test youtrack connection
// @Description Test YouTrack Connection
// @Tags plugins/youtrack
// @Param connectionId path int true "connection ID"
// @Success 200  {object} YoutrackTestConnResponse "Success"
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/test [POST]
func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "find connection from db")
	}
	result, testErr := testConnection(context.TODO(), connection.YoutrackConn)
	if testErr != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, testErr)
	}
	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}

// PostConnections create youtrack connection
// @Summary create youtrack connection
// @Description Create YouTrack connection
// @Tags plugins/youtrack
// @Param body body models.YoutrackConnection true "json body"
// @Success 200  {object} models.YoutrackConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/youtrack/connections [POST]
func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var connection models.YoutrackConnection
	if err := helper.DecodeMapStruct(input.Body, &connection, false); err != nil {
		return nil, err
	}
	// fail fast on a malformed base URL; the normalized form is what gets stored
	endpoint, err := validateEndpoint(connection.Endpoint)
	if err != nil {
		return nil, err
	}
	input.Body["endpoint"] = endpoint
	if connection.RateLimitPerHour == 0 {
		input.Body["rateLimitPerHour"] = defaultRateLimitPerHour
	}
	return dsHelper.ConnApi.Post(input)
}

// PatchConnection patch youtrack connection
// @Summary patch youtrack connection
// @Description Patch YouTrack connection
// @Tags plugins/youtrack
// @Param body body models.YoutrackConnection true "json body"
// @Success 200  {object} models.YoutrackConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId} [PATCH]
func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// validate only when the patch touches the endpoint — a rename-only PATCH
	// carries no endpoint and must not re-trigger the URL rule
	if rawEndpoint, ok := input.Body["endpoint"]; ok {
		endpoint, ok := rawEndpoint.(string)
		if !ok {
			return nil, errors.BadInput.New("endpoint must be a string")
		}
		normalized, err := validateEndpoint(endpoint)
		if err != nil {
			return nil, err
		}
		input.Body["endpoint"] = normalized
	}
	return dsHelper.ConnApi.Patch(input)
}

// DeleteConnection delete a youtrack connection
// @Summary delete a youtrack connection
// @Description Delete a YouTrack connection
// @Tags plugins/youtrack
// @Success 200  {object} models.YoutrackConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 409  {object} services.BlueprintProjectPairs "References exist to this connection"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId} [DELETE]
func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

// ListConnections get all youtrack connections
// @Summary get all youtrack connections
// @Description Get all YouTrack connections
// @Tags plugins/youtrack
// @Success 200  {object} []models.YoutrackConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/youtrack/connections [GET]
func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

// GetConnection get youtrack connection detail
// @Summary get youtrack connection detail
// @Description Get YouTrack connection detail
// @Tags plugins/youtrack
// @Success 200  {object} models.YoutrackConnection
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId} [GET]
func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}
