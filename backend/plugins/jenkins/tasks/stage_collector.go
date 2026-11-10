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
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
)

const RAW_STAGE_TABLE = "jenkins_api_stages"

// FinishedSince selects the builds (aliased tjb) that finished at or after since.
// Builds are stored only once they have a result, so a build that started before
// the last collection and finished after it (e.g. one waiting days on an input step)
// appears in _tool_jenkins_builds only now; filtering on start_time would skip its
// stages forever. timestamp and duration are both in milliseconds, so the sum is
// plain arithmetic on every supported database.
func FinishedSince(since time.Time) dal.Clause {
	return dal.Where(`tjb.timestamp + tjb.duration >= ?`, since.UnixMilli())
}

var CollectApiStagesMeta = plugin.SubTaskMeta{
	Name:             "collectApiStages",
	EntryPoint:       CollectApiStages,
	EnabledByDefault: true,
	Description:      "Collect stages data from jenkins api, supports timeFilter but not diffSync.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

type SimpleBuild struct {
	Number   string
	FullName string
	JobPath  string
}

func CollectApiStages(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*JenkinsTaskData)

	if data.Options.Class == WORKFLOW_MULTI_BRANCH_PROJECT {
		return collectMultiBranchBuildApiStages(taskCtx)
	}

	return collectSingleBuildApiStages(taskCtx)
}

func collectSingleBuildApiStages(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*JenkinsTaskData)

	apiCollector, err := api.NewStatefulApiCollector(api.RawDataSubTaskArgs{
		Params: JenkinsApiParams{
			ConnectionId: data.Options.ConnectionId,
			FullName:     data.Options.JobFullName,
		},
		Ctx:   taskCtx,
		Table: RAW_STAGE_TABLE,
	})
	if err != nil {
		return err
	}

	clauses := []dal.Clause{
		dal.Select("tjb.number,tjb.full_name"),
		dal.From("_tool_jenkins_builds as tjb"),
		dal.Where(`tjb.connection_id = ? and tjb.job_path = ? and tjb.job_name = ? and tjb.class = ?`,
			data.Options.ConnectionId, data.Options.JobPath, data.Options.JobName, "WorkflowRun"),
	}
	if apiCollector.IsIncremental() && apiCollector.GetSince() != nil {
		clauses = append(clauses, FinishedSince(*apiCollector.GetSince()))
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(SimpleBuild{}))
	if err != nil {
		return err
	}

	err = apiCollector.InitCollector(api.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: fmt.Sprintf("%sjob/%s/{{ .Input.Number }}/wfapi/describe", data.Options.JobPath, data.Options.JobName),
		/*
			(Optional) Return query string for request, or you can plug them into UrlTemplate directly
		*/
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var data struct {
				Stages []json.RawMessage `json:"stages"`
			}
			err := api.UnmarshalResponse(res, &data)
			if err != nil {
				return nil, err
			}
			return data.Stages, nil
		},
		AfterResponse: ignoreHTTPStatus404,
	})

	if err != nil {
		return err
	}

	return apiCollector.Execute()
}

func collectMultiBranchBuildApiStages(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*JenkinsTaskData)

	apiCollector, err := api.NewStatefulApiCollector(api.RawDataSubTaskArgs{
		Params: JenkinsApiParams{
			ConnectionId: data.Options.ConnectionId,
			FullName:     data.Options.JobFullName,
		},
		Ctx:   taskCtx,
		Table: RAW_STAGE_TABLE,
	})
	if err != nil {
		return err
	}

	clauses := []dal.Clause{
		dal.Select("tjb.number,tjb.full_name,tjb.job_path"),
		dal.From("_tool_jenkins_builds as tjb"),
		dal.Where(`tjb.connection_id = ? and tjb.full_name like ? and tjb.class = ?`,
			data.Options.ConnectionId, fmt.Sprintf("%s%%", data.Options.JobFullName), "WorkflowRun"),
	}
	if apiCollector.IsIncremental() && apiCollector.GetSince() != nil {
		clauses = append(clauses, FinishedSince(*apiCollector.GetSince()))
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(SimpleBuild{}))
	if err != nil {
		return err
	}

	err = apiCollector.InitCollector(api.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: "{{ .Input.JobPath }}{{ .Input.Number }}/wfapi/describe",
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var data struct {
				Stages []json.RawMessage `json:"stages"`
			}
			err := api.UnmarshalResponse(res, &data)
			if err != nil {
				return nil, err
			}
			return data.Stages, nil
		},
		AfterResponse: ignoreHTTPStatus404,
	})

	if err != nil {
		return err
	}

	return apiCollector.Execute()
}
