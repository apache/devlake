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
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ConvertRunsMeta)
}

var ConvertRunsMeta = plugin.SubTaskMeta{
	Name:             "Convert Workflow Runs",
	EntryPoint:       ConvertRuns,
	EnabledByDefault: true,
	Description:      "Convert tool layer table github_runs into  domain layer table cicd_pipeline",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{
		//models.GithubRepo{}.TableName(), // config will not regard as dependency
		models.GithubRun{}.TableName(),
		RAW_RUN_TABLE,
	},
	ProductTables: []string{
		devops.CICDPipeline{}.TableName(),
		devops.CiCDPipelineCommit{}.TableName(),
	},
}

func ConvertRuns(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	repo := &models.GithubRepo{}
	err := db.First(repo, dal.Where("connection_id = ? AND github_id = ?", data.Options.ConnectionId, data.Options.GithubId))
	if err != nil {
		return err
	}

	repoIdGen := didgen.NewDomainIdGenerator(&models.GithubRepo{})
	runIdGen := didgen.NewDomainIdGenerator(&models.GithubRun{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubRun]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_RUN_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.Select("*"),
				dal.From(&models.GithubRun{}),
				dal.Where("repo_id = ? and connection_id = ?", repoId, data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("github_updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(line *models.GithubRun) ([]interface{}, errors.Error) {
			createdAt := time.Now()
			if line.GithubCreatedAt != nil {
				createdAt = *line.GithubCreatedAt
			}
			domainPipeline := &devops.CICDPipeline{
				DomainEntity: domainlayer.DomainEntity{Id: runIdGen.Generate(
					data.Options.ConnectionId, line.RepoId, line.ID),
				},
				Name: line.Name,
				TaskDatesInfo: devops.TaskDatesInfo{
					CreatedDate:  createdAt,
					StartedDate:  line.RunStartedAt,
					FinishedDate: line.GithubUpdatedAt,
				},
				CicdScopeId: repoIdGen.Generate(data.Options.ConnectionId, line.RepoId),
				Type:        line.Type,
				Environment: line.Environment,
				Result: devops.GetResult(&devops.ResultRule{
					Success: []string{StatusSuccess},
					Failure: []string{StatusFailure, StatusCancelled, StatusTimedOut, StatusStartUpFailure},
					Default: devops.RESULT_DEFAULT,
				}, line.Conclusion),
				OriginalResult: line.Conclusion,
				Status: devops.GetStatus(&devops.StatusRule{
					Done:       []string{StatusCompleted, StatusSuccess, StatusFailure, StatusCancelled, StatusTimedOut, StatusStartUpFailure},
					InProgress: []string{StatusInProgress, StatusQueued, StatusWaiting, StatusPending},
					Default:    devops.STATUS_OTHER,
				}, line.Status),
				OriginalStatus: line.Status,
				DisplayTitle:   line.DisplayTitle,
				Url:            line.URL,
			}
			if line.GithubUpdatedAt != nil && line.RunStartedAt != nil {
				domainPipeline.DurationSec = float64(line.GithubUpdatedAt.Sub(*line.RunStartedAt).Milliseconds() / 1e3)
			}

			domainPipelineCommit := &devops.CiCDPipelineCommit{
				PipelineId: runIdGen.Generate(
					data.Options.ConnectionId, line.RepoId, line.ID),
				CommitSha:    line.HeadSha,
				Branch:       line.HeadBranch,
				RepoId:       repoIdGen.Generate(data.Options.ConnectionId, repoId),
				RepoUrl:      repo.HTMLUrl,
				DisplayTitle: line.DisplayTitle,
				Url:          line.HTMLURL,
			}

			return []interface{}{
				domainPipeline,
				domainPipelineCommit,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
