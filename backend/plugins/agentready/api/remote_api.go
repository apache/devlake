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
	helperapi "github.com/apache/devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/plugins/agentready/models"
	"github.com/apache/devlake/plugins/agentready/tasks"
)

func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.ModelApiHelper.FindByPk(input)
	if err != nil {
		return nil, err
	}

	db := basicRes.GetDal()
	var ghConn githubConn
	if dbErr := db.First(&ghConn, dal.Where("id = ?", connection.GitHubConnectionId)); dbErr != nil {
		return nil, errors.Default.New(fmt.Sprintf("GitHub connection %d not found", connection.GitHubConnectionId))
	}

	endpoint := ghConn.Endpoint
	if endpoint == "" {
		endpoint = "https://api.github.com"
	}

	branch := connection.Branch
	if branch == "" {
		resolvedBranch, branchErr := tasks.FetchDefaultBranch(gocontext.Background(), endpoint, connection.SubmissionsRepo, ghConn.Token)
		if branchErr != nil {
			return nil, errors.Default.Wrap(branchErr, "failed to resolve default branch")
		}
		branch = resolvedBranch
	}
	submissionsPath := connection.SubmissionsPath
	if submissionsPath == "" {
		submissionsPath = "submissions"
	}

	treeResp, fetchErr := tasks.FetchGithubTree(gocontext.Background(), endpoint, connection.SubmissionsRepo, branch, ghConn.Token)
	if fetchErr != nil {
		return nil, errors.Default.Wrap(fetchErr, "failed to fetch submissions tree")
	}

	entries := tasks.ParseSubmissionEntries(treeResp.Tree, submissionsPath)

	seen := map[string]bool{}
	var children []dsmodels.DsRemoteApiScopeListEntry[models.AgentReadyScope]

	for _, entry := range entries {
		fullName := fmt.Sprintf("%s/%s", entry.Org, entry.Repo)
		if seen[fullName] {
			continue
		}
		seen[fullName] = true

		scopeData := &models.AgentReadyScope{
			FullName: fullName,
			Name:     entry.Repo,
		}
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.AgentReadyScope]{
			Type:     helperapi.RAS_ENTRY_TYPE_SCOPE,
			ParentId: nil,
			Id:       scopeData.ScopeId(),
			Name:     scopeData.ScopeName(),
			FullName: scopeData.ScopeFullName(),
			Data:     scopeData,
		})
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"children":      children,
			"nextPageToken": "",
		},
		Status: http.StatusOK,
	}, nil
}
