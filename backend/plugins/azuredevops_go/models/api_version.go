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
	"net/http"
	"net/url"
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

const DefaultAPIVersion = "7.1"

var _ plugin.PrepareApiClient = (*AzuredevopsConn)(nil)

// PrepareApiClient negotiates once per client so existing connections also adapt to server upgrades.
func (conn *AzuredevopsConn) PrepareApiClient(client plugin.ApiClient) errors.Error {
	version := DefaultAPIVersion
	before := client.GetBeforeFunction()
	if conn.Endpoint != "" {
		// NewApiClientFromConnection installs authentication after preparation.
		client.SetBeforeFunction(func(req *http.Request) errors.Error {
			if before != nil {
				if err := before(req); err != nil {
					return err
				}
			}
			return conn.SetupAuthentication(req)
		})
		var err errors.Error
		version, err = conn.detectAPIVersion(client)
		client.SetBeforeFunction(before)
		if err != nil {
			return err
		}
	}

	setVersion := func(req *http.Request) errors.Error {
		if before != nil {
			if err := before(req); err != nil {
				return err
			}
		}
		query := req.URL.Query()
		if conn.Endpoint != "" {
			query.Set("api-version", apiVersionForPath(version, req.URL.Path))
			if (version == "6.0" || version == "7.0") && strings.HasSuffix(strings.TrimRight(strings.ToLower(req.URL.Path), "/"), "/pullrequests") {
				// These filters were introduced in 7.1. Older servers must scan all PR pages.
				query.Del("searchCriteria.minTime")
				query.Del("searchCriteria.maxTime")
				query.Del("searchCriteria.queryTimeRangeType")
			}
			req.URL.RawQuery = query.Encode()
		} else if query.Get("api-version") == "" {
			query.Set("api-version", version)
			req.URL.RawQuery = query.Encode()
		}
		return nil
	}
	client.SetBeforeFunction(setVersion)
	return nil
}

func (conn *AzuredevopsConn) detectAPIVersion(client plugin.ApiClient) (string, errors.Error) {
	path := "_apis/projects"
	for _, version := range []string{"7.2", "7.1", "7.0", "6.0"} {
		res, err := client.Get(path, url.Values{"api-version": {apiVersionForPath(version, path)}, "$top": {"1"}}, nil)
		if err != nil {
			return "", errors.Default.Wrap(err, "failed to detect Azure DevOps API version")
		}
		unsupported, err := checkAPIVersionResponse(res)
		if err != nil {
			return "", err
		}
		if !unsupported {
			return version, nil
		}
	}
	return "", errors.BadInput.New("Azure DevOps Server must support REST API version 6.0 or later")
}

// Preview revisions belong to individual REST resources, not the server as a whole.
func apiVersionForPath(version, path string) string {
	_, resource, _ := strings.Cut(strings.ToLower(path), "_apis/")
	resource = strings.TrimRight(resource, "/")
	if version == "6.0" {
		switch {
		case resource == "serviceendpoint/endpoints":
			return "6.0-preview.4"
		case strings.HasPrefix(resource, "sourceproviders/") && strings.HasSuffix(resource, "/repositories"):
			return "6.0-preview.1"
		}
	}
	if version == "7.2" {
		switch {
		case resource == "projects", resource == "serviceendpoint/endpoints":
			return "7.2-preview.4"
		case strings.HasPrefix(resource, "git/repositories"):
			if strings.Contains(resource, "/pullrequests/") && strings.HasSuffix(resource, "/commits") {
				return "7.2-preview.1"
			}
			return "7.2-preview.2"
		case resource == "build/builds":
			return "7.2-preview.8"
		case strings.HasPrefix(resource, "build/builds/") && strings.HasSuffix(resource, "/timeline"):
			return "7.2-preview.3"
		case strings.HasPrefix(resource, "sourceproviders/") && strings.HasSuffix(resource, "/repositories"):
			return "7.2-preview.1"
		}
	}
	return version
}

func checkAPIVersionResponse(res *http.Response) (bool, errors.Error) {
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusNonAuthoritativeInfo {
		return false, errors.Unauthorized.New("Azure DevOps API version detection failed: check credentials and project read permissions")
	}
	var body struct {
		Value   []json.RawMessage `json:"value"`
		TypeKey string            `json:"typeKey"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return false, errors.BadInput.Wrap(err, fmt.Sprintf("Azure DevOps API version detection returned an invalid response (HTTP %d)", res.StatusCode))
	}
	if res.StatusCode == http.StatusOK && body.Value != nil {
		return false, nil
	}
	if res.StatusCode == http.StatusBadRequest &&
		(body.TypeKey == "VssVersionNotSupportedException" || body.TypeKey == "VssVersionOutOfRangeException") {
		return true, nil
	}
	return false, errors.BadInput.New(fmt.Sprintf("Azure DevOps API version detection failed (HTTP %d, %s)", res.StatusCode, body.TypeKey))
}
