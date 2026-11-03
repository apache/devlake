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
	"net/http"
	"reflect"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/circleci/models"
)

const RAW_WORKFLOW_TABLE = "circleci_api_workflows"

var _ plugin.SubTaskEntryPoint = CollectWorkflows

var CollectWorkflowsMeta = plugin.SubTaskMeta{
	Name:             "collectWorkflows",
	EntryPoint:       CollectWorkflows,
	EnabledByDefault: true,
	Description:      "collect circleci workflows",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

// UnfinishedWorkflowsInputClauses returns the DAL clauses that select the workflows that
// are still in a non-terminal status and therefore need their details recollected by the
// CollectWorkflows "unfinished details" collector. Only workflows created at or after
// createdAfter are selected (#9304).
func UnfinishedWorkflowsInputClauses(connectionId uint64, projectSlug string, createdAfter time.Time) []dal.Clause {
	return []dal.Clause{
		dal.Select("id"),
		dal.From(&models.CircleciWorkflow{}),
		dal.Where(
			"connection_id = ? AND project_slug = ? AND status IN ('running', 'on_hold', 'failing') AND created_date >= ?",
			connectionId, projectSlug, createdAfter,
		),
	}
}

func CollectWorkflows(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RAW_WORKFLOW_TABLE)
	logger := taskCtx.GetLogger()
	logger.Info("collect workflows")

	collector, err := api.NewStatefulApiCollectorForFinalizableEntity(api.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		ApiClient:          data.ApiClient,
		CollectNewRecordsByList: api.FinalizableApiCollectorListArgs{
			PageSize:              int(data.Options.PageSize),
			GetNextPageCustomData: ExtractNextPageToken,
			BuildInputIterator: func(isIncremental bool, createdAfter *time.Time) (api.Iterator, errors.Error) {
				clauses := []dal.Clause{
					dal.Select("id"),
					dal.From(&models.CircleciPipeline{}),
					dal.Where("connection_id = ? AND project_slug = ?", data.Options.ConnectionId, data.Options.ProjectSlug),
				}

				// Incremental: pipelines newer than last successful collectWorkflows.
				// Full sync: pipelines within SyncPolicy.TimeAfter window.
				if createdAfter != nil {
					clauses = append(clauses, dal.Where("created_date >= ?", createdAfter))
				}

				db := taskCtx.GetDal()
				cursor, err := db.Cursor(clauses...)
				if err != nil {
					return nil, err
				}
				return api.NewDalCursorIterator(db, cursor, reflect.TypeOf(models.CircleciPipeline{}))
			},
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate:    "/v2/pipeline/{{ .Input.Id }}/workflow",
				Query:          BuildQueryParamsWithPageToken,
				ResponseParser: ParseCircleciPageTokenResp,
				AfterResponse:  ignoreDeletedOrBrokenBuilds,
			},
			GetCreated: extractCreatedAt,
		},
		CollectUnfinishedDetails: &api.FinalizableApiCollectorDetailArgs{
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate: "/v2/workflow/{{ .Input.Id }}",
				Query:       nil,
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					var data json.RawMessage
					err := api.UnmarshalResponse(res, &data)
					return []json.RawMessage{data}, err
				},
				AfterResponse: ignoreDeletedOrBrokenBuilds,
			},
			BuildInputIterator: func() (api.Iterator, errors.Error) {
				clauses := UnfinishedWorkflowsInputClauses(data.Options.ConnectionId, data.Options.ProjectSlug, UnfinishedDetailsCreatedAfter(time.Now()))
				db := taskCtx.GetDal()
				cursor, err := db.Cursor(clauses...)
				if err != nil {
					return nil, err
				}
				return api.NewDalCursorIterator(db, cursor, reflect.TypeOf(models.CircleciWorkflow{}))
			},
		},
	})
	if err != nil {
		logger.Error(err, "collect workflows error")
		return err
	}
	return collector.Execute()
}
