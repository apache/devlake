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
	"encoding/json"
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/utils"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/github/models"
	githubTasks "github.com/apache/devlake/plugins/github/tasks"
)

var _ plugin.SubTaskEntryPoint = ExtractAccounts

var ExtractJobsMeta = plugin.SubTaskMeta{
	Name:             "Extract Jobs",
	EntryPoint:       ExtractJobs,
	EnabledByDefault: true,
	Description:      "Extract raw run data into tool layer table github_jobs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

func ExtractJobs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*githubTasks.GithubTaskData)
	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[DbCheckRun]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: githubTasks.GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_GRAPHQL_JOBS_TABLE,
			// Type and Environment come from these patterns, so a change must re-extract every job
			SubtaskConfig: data.RegexEnricher.PlainMap(),
		},
		Extract: func(checkRun *DbCheckRun, row *api.RawData) ([]interface{}, errors.Error) {
			results := make([]interface{}, 0, 1)
			// Normalize zero-date times to nil
			checkRun.StartedAt = utils.NilIfZeroTime(checkRun.StartedAt)
			checkRun.CompletedAt = utils.NilIfZeroTime(checkRun.CompletedAt)
			paramsBytes, marshalError := json.Marshal(checkRun.Steps.Nodes)
			err := errors.Convert(marshalError)
			if err != nil {
				taskCtx.GetLogger().Error(err, `Marshal checkRun.Steps.Nodes failed`)
			}
			githubJob := &models.GithubJob{
				ConnectionId: data.Options.ConnectionId,
				RunID:        checkRun.RunId,
				RepoId:       data.Options.GithubId,
				ID:           checkRun.DatabaseId,
				NodeID:       checkRun.Id,
				HTMLURL:      checkRun.DetailsUrl,
				Status:       strings.ToUpper(checkRun.Status),
				Conclusion:   strings.ToUpper(checkRun.Conclusion),
				StartedAt:    checkRun.StartedAt,
				CompletedAt:  checkRun.CompletedAt,
				Name:         checkRun.Name,
				Steps:        paramsBytes,
				Type:         data.RegexEnricher.ReturnNameIfMatched(devops.DEPLOYMENT, checkRun.Name),
				Environment:  data.RegexEnricher.ReturnNameIfOmittedOrMatched(devops.PRODUCTION, checkRun.Name),
			}
			results = append(results, githubJob)

			return results, nil
		},
	})

	if err != nil {
		return errors.Convert(err)
	}

	return extractor.Execute()
}
