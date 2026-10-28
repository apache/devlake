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
	"net/http"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helperapi "github.com/apache/devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/plugins/aisdlc/models"
	"github.com/apache/devlake/plugins/aisdlc/tasks"
)

func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.ModelApiHelper.FindByPk(input)
	if err != nil {
		return nil, err
	}
	glConn, err := tasks.LoadGitlabConnection(basicRes.GetDal(), connection.GitlabConnectionId)
	if err != nil {
		return nil, err
	}
	client, clientErr := tasks.NewGitlabClient(gocontext.Background(), glConn.Endpoint, glConn.Token)
	if clientErr != nil {
		return nil, errors.Default.Wrap(clientErr, "gitlab client")
	}
	search := input.Query.Get("search")
	page := input.Query.Get("pageToken")
	projects, next, listErr := client.ListProjects(gocontext.Background(), search, page)
	if listErr != nil {
		return nil, errors.Default.Wrap(listErr, "list gitlab projects")
	}
	children := make([]dsmodels.DsRemoteApiScopeListEntry[models.AisdlcScope], 0, len(projects))
	for _, project := range projects {
		if project.PathWithNamespace == "" {
			continue
		}
		scopeData := &models.AisdlcScope{
			FullName: project.PathWithNamespace,
			Name:     project.Name,
		}
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.AisdlcScope]{
			Type:     helperapi.RAS_ENTRY_TYPE_SCOPE,
			Id:       scopeData.ScopeId(),
			Name:     scopeData.ScopeName(),
			FullName: scopeData.ScopeFullName(),
			Data:     scopeData,
		})
	}
	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"children":      children,
			"nextPageToken": next,
		},
		Status: http.StatusOK,
	}, nil
}
