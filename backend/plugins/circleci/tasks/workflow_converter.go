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
	"fmt"
	"reflect"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/circleci/models"
)

var ConvertWorkflowsMeta = plugin.SubTaskMeta{
	Name:             "convertWorkflows",
	EntryPoint:       ConvertWorkflows,
	EnabledByDefault: true,
	Description:      "convert circleci workflows",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

func ConvertWorkflows(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RAW_WORKFLOW_TABLE)
	logger := taskCtx.GetLogger()
	logger.Info("convert workflows")

	db := taskCtx.GetDal()
	clauses := []dal.Clause{
		dal.From(&models.CircleciWorkflow{}),
		dal.Where("connection_id = ? AND project_slug = ?", data.Options.ConnectionId, data.Options.ProjectSlug),
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()
	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(models.CircleciWorkflow{}),
		Input:              cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			userTool := inputRow.(*models.CircleciWorkflow)
			// Skip if CreatedAt is null or empty string - still enters into the `_tool_circleci_workflows` table with null values
			if userTool.CreatedDate.ToNullableTime() == nil {
				logger.Info("CreatedDate is null or empty string in the CircleCI API response for %s", userTool.PipelineId)
				return []interface{}{}, nil
			}
			createdAt := userTool.CreatedDate.ToTime()
			pipeline := &devops.CICDPipeline{
				DomainEntity: domainlayer.DomainEntity{
					Id: getWorkflowIdGen().Generate(data.Options.ConnectionId, userTool.Id),
				},
				Name:        userTool.Name,
				DurationSec: userTool.DurationSec,
				TaskDatesInfo: devops.TaskDatesInfo{
					CreatedDate:  createdAt,
					StartedDate:  &createdAt,
					FinishedDate: userTool.StoppedAt.ToNullableTime(),
				},
				CicdScopeId: getProjectIdGen().Generate(data.Options.ConnectionId, userTool.ProjectSlug),
				// reference: https://circleci.com/docs/api/v2/index.html#operation/getWorkflowById
				Status: devops.GetStatus(&devops.StatusRule{
					Done:    []string{"canceled", "failed", "failing", "success", "not_run", "error"}, // on_hold
					Default: devops.STATUS_OTHER,
				}, userTool.Status),
				OriginalStatus: userTool.Status,
				Result: devops.GetResult(&devops.ResultRule{
					Success: []string{"success"},
					Failure: []string{"failed", "failing", "error"}, // not_run,canceled
					Default: devops.RESULT_DEFAULT,
				}, userTool.Status),
				Type:         data.RegexEnricher.ReturnNameIfMatched(devops.DEPLOYMENT, userTool.Name),
				Environment:  data.RegexEnricher.ReturnNameIfOmittedOrMatched(devops.PRODUCTION, userTool.Name),
				DisplayTitle: fmt.Sprintf("%s#%d", userTool.Name, userTool.PipelineNumber),
			}
			result := make([]interface{}, 0, 2)
			result = append(result, pipeline)

			// CircleCI does not support multiple repositories in one pipeline, so we can get the commit sha from the pipeline
			// and convert it to a pipeline commit
			if p, err := findPipelineById(db, userTool.PipelineId); err == nil {
				if p.Vcs.Revision != "" {
					result = append(result, &devops.CiCDPipelineCommit{
						PipelineId:   pipeline.Id,
						CommitSha:    p.Vcs.Revision,
						Branch:       p.Vcs.Branch,
						RepoId:       p.Vcs.OriginRepositoryUrl,
						RepoUrl:      p.Vcs.OriginRepositoryUrl,
						DisplayTitle: pipeline.DisplayTitle,
					})
				}
			}
			return result, nil
		},
	})

	if err != nil {
		return err
	}

	return converter.Execute()
}
