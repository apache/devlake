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
	gocontext "context"
	"fmt"
	"net/http"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/agentready/models"
	"github.com/apache/devlake/plugins/agentready/tasks"
)

func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var conn models.AgentReadyConnection
	if err := api.Decode(input.Body, &conn, nil); err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode connection")
	}
	resolveDefaultBranch(gocontext.Background(), &conn)
	if conn.Branch != "" {
		input.Body["branch"] = conn.Branch
	}
	return dsHelper.ConnApi.Post(input)
}

func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	model, err := dsHelper.ConnApi.PatchModel(input, true)
	if err != nil {
		return nil, errors.Convert(err)
	}
	resolveDefaultBranch(gocontext.Background(), model)
	if updateErr := dsHelper.ConnApi.ConnectionSrvHelper.Update(model); updateErr != nil {
		return nil, updateErr
	}
	return &plugin.ApiResourceOutput{
		Body: dsHelper.ConnApi.Sanitize(model),
	}, nil
}

func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}

type githubConn struct {
	ID       uint64 `gorm:"primaryKey;column:id"`
	Endpoint string `gorm:"column:endpoint"`
	Token    string `gorm:"column:token;serializer:encdec"`
}

func (githubConn) TableName() string { return "_tool_github_connections" }

func resolveDefaultBranch(ctx gocontext.Context, conn *models.AgentReadyConnection) {
	if conn.Branch != "" {
		return
	}
	if conn.SubmissionsRepo == "" {
		return
	}
	if conn.GitHubConnectionId == 0 {
		return
	}

	logger := basicRes.GetLogger()

	db := basicRes.GetDal()
	var ghConn githubConn
	if err := db.First(&ghConn, dal.Where("id = ?", conn.GitHubConnectionId)); err != nil {
		logger.Warn(err, "could not look up GitHub connection %d for branch resolution, defaulting to main", conn.GitHubConnectionId)
		conn.Branch = "main"
		return
	}

	endpoint := ghConn.Endpoint
	if endpoint == "" {
		endpoint = "https://api.github.com"
	}

	branch, fetchErr := tasks.FetchDefaultBranch(ctx, endpoint, conn.SubmissionsRepo, ghConn.Token)
	if fetchErr != nil {
		logger.Warn(nil, "could not resolve default branch for %s: %v, defaulting to main", conn.SubmissionsRepo, fetchErr)
		conn.Branch = "main"
		return
	}
	conn.Branch = branch
}

func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var conn models.AgentReadyConnection
	if err := api.Decode(input.Body, &conn, nil); err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode connection")
	}
	return testAgentReadyConnection(gocontext.Background(), &conn)
}

func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.Convert(err)
	}
	return testAgentReadyConnection(gocontext.Background(), connection)
}

func testAgentReadyConnection(ctx gocontext.Context, conn *models.AgentReadyConnection) (*plugin.ApiResourceOutput, errors.Error) {
	if conn.GitHubConnectionId == 0 {
		return nil, errors.BadInput.New("githubConnectionId is required")
	}
	if conn.SubmissionsRepo == "" {
		return nil, errors.BadInput.New("submissionsRepo is required")
	}

	db := basicRes.GetDal()
	var ghConn githubConn
	if err := db.First(&ghConn, dal.Where("id = ?", conn.GitHubConnectionId)); err != nil {
		return nil, errors.BadInput.New(fmt.Sprintf("GitHub connection %d not found", conn.GitHubConnectionId))
	}

	endpoint := ghConn.Endpoint
	if endpoint == "" {
		endpoint = "https://api.github.com"
	}
	token := ghConn.Token
	if token == "" {
		return nil, errors.BadInput.New("referenced GitHub connection has no token")
	}

	apiClient, err := api.NewApiClient(ctx, endpoint, nil, 0, "", basicRes)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create API client")
	}
	apiClient.SetHeaders(map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	})

	repoURL := fmt.Sprintf("repos/%s", conn.SubmissionsRepo)
	resp, err := apiClient.Get(repoURL, nil, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to reach GitHub API")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.BadInput.New(fmt.Sprintf("repository %q not found or not accessible", conn.SubmissionsRepo))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Default.New(fmt.Sprintf("GitHub API returned %d for repo %s", resp.StatusCode, conn.SubmissionsRepo))
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"success": true,
			"message": fmt.Sprintf("Successfully connected to %s", conn.SubmissionsRepo),
		},
		Status: http.StatusOK,
	}, nil
}
