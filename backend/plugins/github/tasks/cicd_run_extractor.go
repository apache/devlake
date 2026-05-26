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
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractRunsMeta)
}

var ExtractRunsMeta = plugin.SubTaskMeta{
	Name:             "Extract Workflow Runs",
	EntryPoint:       ExtractRuns,
	EnabledByDefault: true,
	Description:      "Extract raw run data into tool layer table github_runs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{RAW_RUN_TABLE},
	ProductTables:    []string{models.GithubRun{}.TableName()},
}

func ExtractRuns(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[models.GithubRun]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_RUN_TABLE,
		},
		Extract: func(body *models.GithubRun, row *api.RawData) ([]any, errors.Error) {
			body.RepoId = repoId
			body.ConnectionId = data.Options.ConnectionId
			body.Type = data.RegexEnricher.ReturnNameIfMatched(devops.DEPLOYMENT, body.Name)
			body.Environment = data.RegexEnricher.ReturnNameIfOmittedOrMatched(devops.PRODUCTION, body.Name, body.HeadBranch)
			return []any{body}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
