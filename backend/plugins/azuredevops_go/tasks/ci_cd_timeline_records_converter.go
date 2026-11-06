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
	"time"
)

func init() {
	RegisterSubtaskMeta(&ConvertApiTimelineRecordsMeta)
}

var ConvertApiTimelineRecordsMeta = plugin.SubTaskMeta{
	Name:             "convertApiTimelineRecords",
	EntryPoint:       ConvertApiTimelineRecords,
	EnabledByDefault: true,
	Description:      "Convert tool layer table azuredevops_timeline_records into domain layer table cicd_tasks",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{
		models.AzuredevopsTimelineRecord{}.TableName(),
	},
}

func ConvertApiTimelineRecords(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawPrCommitTable)
	db := taskCtx.GetDal()

	clauses := []dal.Clause{
		dal.From(&models.AzuredevopsTimelineRecord{}),
		dal.Join(`left join _tool_azuredevops_go_builds
			on _tool_azuredevops_go_builds.azuredevops_id =
			_tool_azuredevops_go_timeline_records.build_id`),
		dal.Where("_tool_azuredevops_go_builds.repository_id = ? and _tool_azuredevops_go_builds.connection_id = ?", data.Options.RepositoryId,
			data.Options.ConnectionId),
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}

	tlRecordIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsTimelineRecord{})
	repoIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsRepo{})
	buildIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsBuild{})

	converter, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(models.AzuredevopsTimelineRecord{}),
		Input:              cursor,

		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			tlRecord := inputRow.(*models.AzuredevopsTimelineRecord)

			var duration = 0.0
			if tlRecord.FinishTime != nil && tlRecord.StartTime != nil {
				duration = float64(tlRecord.FinishTime.Sub(*tlRecord.StartTime).Milliseconds() / 1e3)
			}

			createdAt := time.Now()
			if tlRecord.StartTime != nil {
				createdAt = *tlRecord.StartTime
			}

			domainTask := &devops.CICDTask{
				DomainEntity: domainlayer.DomainEntity{
					Id: tlRecordIdGen.Generate(data.Options.ConnectionId, tlRecord.RecordId, tlRecord.BuildId),
				},
				Name:           tlRecord.Name,
				PipelineId:     buildIdGen.Generate(data.Options.ConnectionId, tlRecord.BuildId),
				Result:         devops.GetResult(cicdTaskResultRule, tlRecord.Result),
				Status:         devops.GetStatus(cicdTaskStatusRule, tlRecord.State),
				OriginalStatus: tlRecord.State,
				OriginalResult: tlRecord.Result,
				DurationSec:    duration,
				Environment:    data.RegexEnricher.ReturnNameIfMatched(devops.PRODUCTION, tlRecord.Name),
				Type:           data.RegexEnricher.ReturnNameIfMatched(devops.DEPLOYMENT, tlRecord.Name),
				TaskDatesInfo: devops.TaskDatesInfo{
					CreatedDate:  createdAt,
					StartedDate:  tlRecord.StartTime,
					FinishedDate: tlRecord.FinishTime,
				},
				CicdScopeId: repoIdGen.Generate(data.Options.ConnectionId, data.Options.RepositoryId),
			}

			return []interface{}{
				domainTask,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
