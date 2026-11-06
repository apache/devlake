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
	"github.com/apache/devlake/plugins/circleci/models"
)

var _ plugin.SubTaskEntryPoint = ExtractJobs

var ExtractJobsMeta = plugin.SubTaskMeta{
	Name:             "extractJobs",
	EntryPoint:       ExtractJobs,
	EnabledByDefault: true,
	Description:      "Extract raw workspace data into tool layer table _tool_circleci_jobs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

func ExtractJobs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CircleciTaskData)
	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[models.CircleciJob]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_JOB_TABLE,
			Params: models.CircleciApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectSlug:  data.Options.ProjectSlug,
			},
		},
		Extract: func(userRes *models.CircleciJob, row *api.RawData) ([]any, errors.Error) {
			input := &models.CircleciWorkflow{}
			err := errors.Convert(json.Unmarshal(row.Input, input))
			if err != nil {
				return nil, err
			}
			toolL := *userRes
			toolL.ConnectionId = data.Options.ConnectionId
			toolL.ProjectSlug = data.Options.ProjectSlug
			toolL.WorkflowId = input.Id
			toolL.PipelineId = input.PipelineId
			if userRes.StartedAt != nil && userRes.StoppedAt != nil {
				startTime := userRes.StartedAt.ToTime()
				stopTime := userRes.StoppedAt.ToTime()
				toolL.DurationSec = float64(stopTime.Sub(startTime).Milliseconds() / 1e3)
			}
			return []any{
				&toolL,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return extractor.Execute()
}
