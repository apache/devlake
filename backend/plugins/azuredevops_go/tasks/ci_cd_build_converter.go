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
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
	"reflect"
)

func init() {
	RegisterSubtaskMeta(&ConvertBuildsMeta)
}

var ConvertBuildsMeta = plugin.SubTaskMeta{
	Name:             "convertApiBuilds",
	EntryPoint:       ConvertBuilds,
	EnabledByDefault: true,
	Description:      "Convert tool layer table azuredevops_builds into  domain layer table cicd_pipelines",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{
		models.AzuredevopsBuild{}.TableName(),
	},
}

type JoinedBuild struct {
	models.AzuredevopsBuild

	URL string
}

func ConvertBuilds(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawPullRequestTable)
	clauses := []dal.Clause{
		dal.Select("_tool_azuredevops_go_builds.*, _tool_azuredevops_go_repos.url"),
		dal.From(&models.AzuredevopsBuild{}),
		dal.Join(`left join _tool_azuredevops_go_repos
			on _tool_azuredevops_go_builds.repository_id = _tool_azuredevops_go_repos.id`),
		dal.Where(`_tool_azuredevops_go_builds.repository_id = ? and _tool_azuredevops_go_builds.connection_id = ?`,
			data.Options.RepositoryId, data.Options.ConnectionId),
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	buildIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsBuild{})
	repoIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsRepo{})

	converter, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(JoinedBuild{}),
		Input:              cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			build := inputRow.(*JoinedBuild)
			duration := 0.0

			if build.FinishTime != nil {
				duration = float64(build.FinishTime.Sub(*build.StartTime).Milliseconds() / 1e3)
			}

			domainPipeline := &devops.CICDPipeline{
				DomainEntity: domainlayer.DomainEntity{
					Id: buildIdGen.Generate(data.Options.ConnectionId, build.AzuredevopsId),
				},
				Name:           build.Name,
				Result:         devops.GetResult(&cicdBuildResultRule, build.Result),
				Status:         devops.GetStatus(&cicdBuildStatusRule, build.Status),
				OriginalStatus: build.Status,
				OriginalResult: build.Result,
				CicdScopeId:    repoIdGen.Generate(data.Options.ConnectionId, build.RepositoryId),
				Environment:    data.RegexEnricher.ReturnNameIfMatched(devops.PRODUCTION, build.Name+";"+build.RunName+";"+build.Tags),
				Type:           data.RegexEnricher.ReturnNameIfMatched(devops.DEPLOYMENT, build.Name+";"+build.RunName+";"+build.Tags),
				DurationSec:    duration,
				TaskDatesInfo: devops.TaskDatesInfo{
					CreatedDate:  *build.QueueTime,
					QueuedDate:   build.QueueTime,
					StartedDate:  build.StartTime,
					FinishedDate: build.FinishTime,
				},
			}

			pipelineCommit := &devops.CiCDPipelineCommit{
				PipelineId: domainPipeline.Id,
				CommitSha:  build.SourceVersion,
				Branch:     build.SourceBranch,
				RepoId:     build.RepositoryId,
				RepoUrl:    build.URL,
			}

			return []interface{}{
				domainPipeline,
				pipelineCommit,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
