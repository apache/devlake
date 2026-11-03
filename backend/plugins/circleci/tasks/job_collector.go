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
	"reflect"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/circleci/models"
)

const RAW_JOB_TABLE = "circleci_api_jobs"

var _ plugin.SubTaskEntryPoint = CollectJobs

var CollectJobsMeta = plugin.SubTaskMeta{
	Name:             "collectJobs",
	EntryPoint:       CollectJobs,
	EnabledByDefault: true,
	Description:      "collect circleci jobs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

// UnfinishedJobsInputClauses returns the DAL clauses that select the workflows whose
// jobs are still in a non-terminal status and therefore need their job details
// recollected by the CollectJobs "unfinished details" collector. Only workflows created
// at or after createdAfter are selected (#9304); jobs have no created date of their own.
func UnfinishedJobsInputClauses(connectionId uint64, projectSlug string, createdAfter time.Time) []dal.Clause {
	return []dal.Clause{
		dal.Select("DISTINCT j.workflow_id AS id"), // #8907: alias to id so {{ .Input.Id }} resolves when scanned into CircleciJob
		dal.From("_tool_circleci_jobs j"),
		dal.Join("JOIN _tool_circleci_workflows w ON w.connection_id = j.connection_id AND w.id = j.workflow_id"),
		dal.Where(
			"j.connection_id = ? AND j.project_slug = ? AND j.status IN ('running', 'not_running', 'queued', 'on_hold') AND w.created_date >= ?",
			connectionId, projectSlug, createdAfter,
		),
	}
}

func CollectJobs(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RAW_JOB_TABLE)
	logger := taskCtx.GetLogger()
	logger.Info("collect jobs")

	collector, err := api.NewStatefulApiCollectorForFinalizableEntity(api.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		ApiClient:          data.ApiClient,
		CollectNewRecordsByList: api.FinalizableApiCollectorListArgs{
			PageSize:              int(data.Options.PageSize),
			GetNextPageCustomData: ExtractNextPageToken,
			BuildInputIterator: func(isIncremental bool, createdAfter *time.Time) (api.Iterator, errors.Error) {
				clauses := []dal.Clause{
					dal.Select("id, pipeline_id"), // pipeline_id not on individual job response but required for result
					dal.From(&models.CircleciWorkflow{}),
					dal.Where("connection_id = ? and project_slug = ?", data.Options.ConnectionId, data.Options.ProjectSlug),
				}

				// Incremental: workflows newer than last successful collectJobs.
				// Full sync: workflows within SyncPolicy.TimeAfter window.
				if createdAfter != nil {
					clauses = append(clauses, dal.Where("created_date >= ?", createdAfter))
				}

				db := taskCtx.GetDal()
				cursor, err := db.Cursor(clauses...)
				if err != nil {
					return nil, err
				}
				return api.NewDalCursorIterator(db, cursor, reflect.TypeOf(models.CircleciWorkflow{}))
			},
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate:    "/v2/workflow/{{ .Input.Id }}/job",
				Query:          BuildQueryParamsWithPageToken,
				ResponseParser: ParseCircleciPageTokenResp,
				AfterResponse:  ignoreDeletedOrBrokenBuilds,
			},
			GetCreated: func(item json.RawMessage) (time.Time, errors.Error) {
				var job struct { // Individual job response lacks created_at field, so have to use started_at
					CreatedAt time.Time `json:"started_at"` // This will be null in some cases (e.g. queued, not_running, blocked)
				}
				if err := json.Unmarshal(item, &job); err != nil {
					return time.Time{}, errors.Default.Wrap(err, "failed to unmarshal job")
				}
				return job.CreatedAt, nil
			},
		},
		CollectUnfinishedDetails: &api.FinalizableApiCollectorDetailArgs{
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate:    "/v2/workflow/{{ .Input.Id }}/job", // The individual job endpoint has different fields so need to recollect all jobs for a workflow
				Query:          BuildQueryParamsWithPageToken,
				ResponseParser: ParseCircleciPageTokenResp,
				AfterResponse:  ignoreDeletedOrBrokenBuilds,
			},
			BuildInputIterator: func() (api.Iterator, errors.Error) {
				db := taskCtx.GetDal()
				clauses := UnfinishedJobsInputClauses(data.Options.ConnectionId, data.Options.ProjectSlug, UnfinishedDetailsCreatedAfter(time.Now()))
				cursor, err := db.Cursor(clauses...)
				if err != nil {
					return nil, err
				}
				return api.NewDalCursorIterator(db, cursor, reflect.TypeOf(models.CircleciJob{}))
			},
		},
	})
	if err != nil {
		logger.Error(err, "collect jobs error")
		return err
	}
	return collector.Execute()
}
