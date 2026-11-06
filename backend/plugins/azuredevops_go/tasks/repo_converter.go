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
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
	"reflect"
)

func init() {
	RegisterSubtaskMeta(&ConvertRepoMeta)
}

var ConvertRepoMeta = plugin.SubTaskMeta{
	Name:             "convertRepo",
	EntryPoint:       ConvertRepo,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_azuredevops_go_repos into domain layer table repos and cicd scope",
	DomainTypes: []string{
		plugin.DOMAIN_TYPE_CODE,
		plugin.DOMAIN_TYPE_CICD,
		plugin.DOMAIN_TYPE_CODE_REVIEW,
		plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{
		models.AzuredevopsRepo{}.TableName(),
	},
	ProductTables: []string{
		code.Repo{}.TableName(),
		devops.CicdScope{}.TableName()},
}

func ConvertRepo(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, models.AzuredevopsRepo{}.TableName())
	if data.Options.RepositoryType == models.RepositoryTypeProject {
		return nil
	}
	db := taskCtx.GetDal()
	clauses := []dal.Clause{
		dal.From(&models.AzuredevopsRepo{}),
		dal.Where("id=? and connection_id = ?", data.Options.RepositoryId, data.Options.ConnectionId),
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := api.NewDataConverter(api.DataConverterArgs{
		InputRowType:       reflect.TypeOf(models.AzuredevopsRepo{}),
		Input:              cursor,
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			repository := inputRow.(*models.AzuredevopsRepo)

			domainRepository := convertToRepositoryModel(repository)
			domainCiCdScope := convertToCicdScopeModel(repository)
			return []interface{}{
				domainRepository,
				domainCiCdScope,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}

func convertToCicdScopeModel(repo *models.AzuredevopsRepo) *devops.CicdScope {
	domainCicdScope := &devops.CicdScope{
		DomainEntity: domainlayer.DomainEntity{
			Id: didgen.NewDomainIdGenerator(repo).Generate(repo.ConnectionId, repo.Id),
		},
		Name:        repo.DomainName(),
		Url:         repo.Url,
		Description: "", // Not Supported in Azure DevOps
	}
	return domainCicdScope
}

func convertToRepositoryModel(repo *models.AzuredevopsRepo) *code.Repo {
	repoIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsRepo{})
	domainRepository := &code.Repo{
		DomainEntity: domainlayer.DomainEntity{
			Id: repoIdGen.Generate(repo.ConnectionId, repo.Id),
		},
		Name: repo.DomainName(),
		Url:  repo.Url,
	}
	return domainRepository
}
