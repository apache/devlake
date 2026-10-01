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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractApiBuildsMeta)
}

var ExtractApiBuildsMeta = plugin.SubTaskMeta{
	Name:             "extractApiBuilds",
	EntryPoint:       ExtractApiBuilds,
	EnabledByDefault: true,
	Description:      "Extract raw build data into tool layer table",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{RawBuildTable},
	ProductTables: []string{
		models.AzuredevopsBuild{}.TableName(),
	},
}

func ExtractApiBuilds(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawBuildTable)
	logger := taskCtx.GetLogger()

	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			results := make([]interface{}, 0, 1)

			buildApi := &models.AzuredevopsApiBuild{}
			err := errors.Convert(json.Unmarshal(row.Data, buildApi))
			if err != nil {
				return nil, err
			}

			tagsB, er := json.Marshal(buildApi.Tags)
			if er != nil {
				logger.Warn(err, "failed to marshal build/builds.tags into a string. fallback to empty string")
				logger.Debug("failed to marshal following value %v", buildApi.Tags)
				tagsB = []byte(nil)
			}

			build := &models.AzuredevopsBuild{
				ConnectionId:  data.Options.ConnectionId,
				AzuredevopsId: buildApi.Id,
				RepositoryId:  data.Options.RepositoryId,
				Status:        buildApi.Status,
				Result:        buildApi.Result,
				Name:          buildApi.Definition.Name,
				RunName:       buildApi.BuildNumber,
				SourceBranch:  buildApi.SourceBranch,
				SourceVersion: buildApi.SourceVersion,
				QueueTime:     buildApi.QueueTime,
				StartTime:     buildApi.StartTime,
				FinishTime:    buildApi.FinishTime,
				Tags:          string(tagsB),
			}

			results = append(results, build)

			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
