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
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractJobsMeta)
}

var ExtractJobsMeta = plugin.SubTaskMeta{
	Name:             "Extract Jobs",
	EntryPoint:       ExtractJobs,
	EnabledByDefault: true,
	Description:      "Extract raw run data into tool layer table github_jobs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{RAW_JOB_TABLE},
	ProductTables:    []string{models.GithubJob{}.TableName()},
}

func ExtractJobs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[models.GithubJob]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_JOB_TABLE,
		},
		Extract: func(githubJob *models.GithubJob, row *api.RawData) ([]any, errors.Error) {
			githubJobResult := &models.GithubJob{
				ConnectionId:  data.Options.ConnectionId,
				RepoId:        repoId,
				ID:            githubJob.ID,
				RunID:         githubJob.RunID,
				RunURL:        githubJob.RunURL,
				NodeID:        githubJob.NodeID,
				HeadSha:       githubJob.HeadSha,
				URL:           githubJob.URL,
				HTMLURL:       githubJob.HTMLURL,
				Status:        strings.ToUpper(githubJob.Status),
				Conclusion:    strings.ToUpper(githubJob.Conclusion),
				StartedAt:     githubJob.StartedAt,
				CompletedAt:   githubJob.CompletedAt,
				Name:          githubJob.Name,
				Steps:         githubJob.Steps,
				CheckRunURL:   githubJob.CheckRunURL,
				Labels:        githubJob.Labels,
				RunnerID:      githubJob.RunID,
				RunnerName:    githubJob.RunnerName,
				RunnerGroupID: githubJob.RunnerGroupID,
				Type:          data.RegexEnricher.ReturnNameIfMatched(devops.DEPLOYMENT, githubJob.Name),
				Environment:   data.RegexEnricher.ReturnNameIfOmittedOrMatched(devops.PRODUCTION, githubJob.Name),
			}
			return []any{githubJobResult}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
