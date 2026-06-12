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

	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/codequality"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/sonarqube/models"
	"github.com/apache/incubator-devlake/plugins/sonarqube/tasks"
)

func MakeDataSourcePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	// load connection, scope and scopeConfig from the db
	connection, err := dsHelper.ConnSrv.FindByPk(connectionId)
	if err != nil {
		return nil, nil, err
	}
	scopeDetails, err := dsHelper.ScopeSrv.MapScopeDetails(connectionId, bpScopes)
	if err != nil {
		return nil, nil, err
	}

	// needed for the connection to populate its access tokens
	// if AppKey authentication method is selected
	_, err = helper.NewApiClientFromConnection(context.TODO(), basicRes, connection)
	if err != nil {
		return nil, nil, err
	}

	plan, err := makeDataSourcePipelinePlanV200(subtaskMetas, scopeDetails, connection)
	if err != nil {
		return nil, nil, err
	}
	scopes, err := makeScopesV200(scopeDetails, connection)
	if err != nil {
		return nil, nil, err
	}

	return plan, scopes, nil
}

func makeDataSourcePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	scopeDetails []*srvhelper.ScopeDetail[models.SonarqubeProject, srvhelper.NoScopeConfig],
	connection *models.SonarqubeConnection,
) (coreModels.PipelinePlan, errors.Error) {
	plan := make(coreModels.PipelinePlan, len(scopeDetails))
	for i, scopeDetail := range scopeDetails {
		stage := plan[i]
		if stage == nil {
			stage = coreModels.PipelineStage{}
		}
		scope := scopeDetail.Scope
		// construct task options
		task, err := helper.MakePipelinePlanTask(
			"sonarqube",
			subtaskMetas,
			nil,
			tasks.SonarqubeOptions{
				ConnectionId: scope.ConnectionId,
				ProjectKey:   scope.ProjectKey,
			},
		)
		if err != nil {
			return nil, err
		}

		stage = append(stage, task)
		plan[i] = stage
	}

	return plan, nil
}

func makeScopesV200(
	scopeDetails []*srvhelper.ScopeDetail[models.SonarqubeProject, srvhelper.NoScopeConfig],
	connection *models.SonarqubeConnection,
) ([]plugin.Scope, errors.Error) {
	scopes := make([]plugin.Scope, 0)
	for _, scopeDetail := range scopeDetails {
		sonarqubeProject := scopeDetail.Scope
		// add board to scopes
		domainBoard := &codequality.CqProject{
			DomainEntityExtended: domainlayer.DomainEntityExtended{
				Id: didgen.NewDomainIdGenerator(&models.SonarqubeProject{}).Generate(sonarqubeProject.ConnectionId, sonarqubeProject.ProjectKey),
			},
			Name: sonarqubeProject.Name,
		}
		scopes = append(scopes, domainBoard)
	}

	return scopes, nil
}

func GetApiProject(
	projectKey string,
	apiClient plugin.ApiClient,
) (*models.SonarqubeApiProject, errors.Error) {
	var resData struct {
		Data []models.SonarqubeApiProject `json:"components"`
	}
	query := url.Values{}
	query.Set("p", "1")
	query.Set("ps", "100")
	query.Set("filter", sonarqubeSearchProjectsQueryFilter(projectKey))
	// Use components/search_projects for consistency and normal-token (Browse) support.
	res, err := apiClient.Get("components/search_projects", query, nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, errors.HttpStatus(res.StatusCode).New(fmt.Sprintf("unexpected status code when requesting project detail from %s", res.Request.URL.String()))
	}
	err = helper.UnmarshalResponse(res, &resData)
	if err != nil {
		return nil, err
	}
	for i := range resData.Data {
		if resData.Data[i].ProjectKey == projectKey {
			return &resData.Data[i], nil
		}
	}
	if len(resData.Data) > 0 {
		return &resData.Data[0], nil
	}
	return nil, errors.BadInput.New(fmt.Sprintf("Cannot find project: %s", projectKey))
}
