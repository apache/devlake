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

package tasks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/plugins/checkmarxone/models"
)

// CheckmarxoneApiClient is a simple HTTP client for CheckmarxOne API
type CheckmarxoneApiClient struct {
	client       *http.Client
	logger       log.Logger
	serverUrl    string
	clientId     string
	clientSecret string
	token        string
	tokenExpire  time.Time
}

// NewCheckmarxoneApiClient creates a new authenticated API client
func NewCheckmarxoneApiClient(logger log.Logger, connection *models.CheckmarxoneConnection) (*CheckmarxoneApiClient, errors.Error) {
	c := &CheckmarxoneApiClient{
		client:       &http.Client{Timeout: 30 * time.Second},
		logger:       logger,
		serverUrl:    connection.ServerUrl,
		clientId:     connection.ClientId,
		clientSecret: connection.ClientSecret,
	}
	if err := c.authenticate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *CheckmarxoneApiClient) authenticate() errors.Error {
	tokenURL := fmt.Sprintf("%s/auth/oauth/token", c.serverUrl)
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.clientId, c.clientSecret)))

	body := url.Values{}
	body.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return errors.Default.Wrap(err, "failed to create auth request")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.client.Do(req)
	if err != nil {
		return errors.Default.Wrap(err, "failed to authenticate with CheckmarxOne")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return errors.HttpStatus(res.StatusCode).New("CheckmarxOne authentication failed")
	}

	type TokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	var tokenResp TokenResponse
	if err := json.NewDecoder(res.Body).Decode(&tokenResp); err != nil {
		return errors.Default.Wrap(err, "failed to parse token response")
	}

	c.token = tokenResp.AccessToken
	c.tokenExpire = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return nil
}

// GetFindings fetches findings for a project
func (c *CheckmarxoneApiClient) GetFindings(projectId string) ([]map[string]interface{}, errors.Error) {
	endpoint := fmt.Sprintf("%s/api/projects/%s/results-summary", c.serverUrl, projectId)
	return c.fetch(endpoint)
}

func (c *CheckmarxoneApiClient) fetch(endpoint string) ([]map[string]interface{}, errors.Error) {
	if err := c.checkAndRefreshToken(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to build request")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Accept", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Default.Wrap(err, "HTTP request failed")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return nil, errors.HttpStatus(res.StatusCode).New(fmt.Sprintf("API error: %s", string(b)))
	}

	type DataResponse struct {
		Results []map[string]interface{} `json:"results"`
		Data    []map[string]interface{} `json:"data"`
	}
	var dataResp DataResponse
	if err := json.NewDecoder(res.Body).Decode(&dataResp); err != nil {
		return nil, errors.Default.Wrap(err, "failed to parse response")
	}

	if len(dataResp.Results) > 0 {
		return dataResp.Results, nil
	}
	return dataResp.Data, nil
}

func (c *CheckmarxoneApiClient) checkAndRefreshToken() errors.Error {
	if time.Now().Before(c.tokenExpire) {
		return nil
	}
	return c.authenticate()
}

// Close releases idle connections
func (c *CheckmarxoneApiClient) Close() {
	c.client.CloseIdleConnections()
}