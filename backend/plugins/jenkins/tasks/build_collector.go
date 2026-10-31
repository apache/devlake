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

	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

const RAW_BUILD_TABLE = "jenkins_api_builds"

// buildFields is the tree of fields requested for every build, as a list item or on its own
const buildFields = "timestamp,number,duration,building,estimatedDuration,fullDisplayName,result,actions[lastBuiltRevision[SHA1,branch[name]],remoteUrls,mercurialRevisionNumber,causes[*]],changeSet[kind,revisions[revision]]"

// UnfinishedBuilds selects the builds of a single job that were still running when last
// collected, so their final state can be collected once they finish.
func UnfinishedBuilds(options *JenkinsOptions) []dal.Clause {
	return []dal.Clause{
		dal.Select("tjb.number,tjb.full_name"),
		dal.From("_tool_jenkins_builds as tjb"),
		dal.Where(`tjb.connection_id = ? and tjb.job_path = ? and tjb.job_name = ? and tjb.building = ?`,
			options.ConnectionId, options.JobPath, options.JobName, true),
	}
}

var CollectApiBuildsMeta = plugin.SubTaskMeta{
	Name:             "collectApiBuilds",
	EntryPoint:       CollectApiBuilds,
	EnabledByDefault: true,
	Description:      "Collect builds data from jenkins api, supports both timeFilter and diffSync.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

type SimpleJob struct {
	FullName string
	Name     string
	Path     string
	Class    string
	URL      string
}

type SimpleJenkinsApiBuild struct {
	Number    int64
	Timestamp int64 `json:"timestamp"`
}

func CollectApiBuilds(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*JenkinsTaskData)
	logger := taskCtx.GetLogger()

	logger.Debug("Collecting builds data from jenkins api, class: %s", data.Options.Class)

	if data.Options.Class == WORKFLOW_MULTI_BRANCH_PROJECT {
		return collectMultiBranchJobApiBuilds(taskCtx)
	}

	return collectSingleJobApiBuilds(taskCtx)
}

// collectSingleJobApiBuilds collects builds data from a single job using Jenkins api.
func collectSingleJobApiBuilds(taskCtx plugin.SubTaskContext) errors.Error {
	// The API input is defined in the plugin's task definition, be that the UI or advanced blueprint.
	data := taskCtx.GetData().(*JenkinsTaskData)
	db := taskCtx.GetDal()
	collector, err := helper.NewStatefulApiCollectorForFinalizableEntity(helper.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Params: JenkinsApiParams{
				ConnectionId: data.Options.ConnectionId,
				FullName:     data.Options.JobFullName,
			},
			Ctx:   taskCtx,
			Table: RAW_BUILD_TABLE,
		},
		ApiClient: data.ApiClient,
		CollectNewRecordsByList: helper.FinalizableApiCollectorListArgs{
			PageSize:    100,
			Concurrency: 10,
			FinalizableApiCollectorCommonArgs: helper.FinalizableApiCollectorCommonArgs{
				UrlTemplate: fmt.Sprintf("%sjob/%s/api/json", data.Options.JobPath, data.Options.JobName),
				Query: func(reqData *helper.RequestData, createdAfter *time.Time) (url.Values, errors.Error) {
					query := url.Values{}
					treeValue := fmt.Sprintf("allBuilds[%s]{%d,%d}",
						buildFields, reqData.Pager.Skip, reqData.Pager.Skip+reqData.Pager.Size)
					query.Set("tree", treeValue)
					return query, nil
				},
				// Running builds are kept too: the list only returns builds that started since
				// the last collection, so a build still running now would never be listed
				// again. Stored as building, CollectUnfinishedDetails re-collects it until it
				// finishes.
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					var data struct {
						Builds []json.RawMessage `json:"allBuilds"`
					}
					err := helper.UnmarshalResponse(res, &data)
					if err != nil {
						return nil, err
					}
					return data.Builds, nil
				},
			},
			GetCreated: func(item json.RawMessage) (time.Time, errors.Error) {
				b := &SimpleJenkinsApiBuild{}
				err := json.Unmarshal(item, b)
				if err != nil {
					return time.Time{}, errors.BadInput.Wrap(err, "failed to unmarshal jenkins build")
				}
				seconds := b.Timestamp / 1000
				nanos := (b.Timestamp % 1000) * 1000000
				return time.Unix(seconds, nanos), nil
			},
		},
		CollectUnfinishedDetails: &helper.FinalizableApiCollectorDetailArgs{
			BuildInputIterator: func() (helper.Iterator, errors.Error) {
				cursor, err := db.Cursor(UnfinishedBuilds(data.Options)...)
				if err != nil {
					return nil, err
				}
				return helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(SimpleBuild{}))
			},
			FinalizableApiCollectorCommonArgs: helper.FinalizableApiCollectorCommonArgs{
				UrlTemplate: fmt.Sprintf("%sjob/%s/{{ .Input.Number }}/api/json", data.Options.JobPath, data.Options.JobName),
				Query: func(reqData *helper.RequestData, createdAfter *time.Time) (url.Values, errors.Error) {
					return url.Values{"tree": {buildFields}}, nil
				},
				// A build deleted while running (e.g. by hand) is skipped instead of failing the task
				AfterResponse: func(res *http.Response) errors.Error {
					if res.StatusCode == http.StatusNotFound {
						return helper.ErrIgnoreAndContinue
					}
					return nil
				},
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					var build json.RawMessage
					err := helper.UnmarshalResponse(res, &build)
					return []json.RawMessage{build}, err
				},
			},
		},
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}

// collectMultiBranchJobApiBuilds collects builds data from a multi-branch workflow using Jenkins api.
func collectMultiBranchJobApiBuilds(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*JenkinsTaskData)
	logger := taskCtx.GetLogger()

	// Jobs added through the multi-branch workflow have _raw_data_table set to "jenkins_api_jobs".
	// Filter by _raw_data_params to scope to only this multi-branch project's branch jobs,
	// preventing cross-project data mixing when multiple multi-branch pipelines exist.
	jobParams := plugin.MarshalScopeParams(JenkinsApiParams{
		ConnectionId: data.Options.ConnectionId,
		FullName:     data.Options.JobFullName,
	})
	clauses := []dal.Clause{
		dal.Select("j.full_name,j.name,j.path,j.class,j.url"),
		dal.From("_tool_jenkins_jobs as j"),
		dal.Where(`j.connection_id = ? and j.class = ? and j._raw_data_table = ? and j._raw_data_params = ?`,
			data.Options.ConnectionId, WORKFLOW_JOB, fmt.Sprintf("_raw_%s", RAW_JOB_TABLE), jobParams),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	iterator, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(SimpleJob{}))
	if err != nil {
		return err
	}

	collectorWithState, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Params: JenkinsApiParams{
			ConnectionId: data.Options.ConnectionId,
			FullName:     data.Options.JobFullName,
		},
		Ctx:   taskCtx,
		Table: RAW_BUILD_TABLE,
	})
	if err != nil {
		return err
	}

	logger.Debug("About to call collectorWithState.InitCollector")

	err = collectorWithState.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: "{{ .Input.Path }}api/json",
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			treeValue := fmt.Sprintf("allBuilds[%s]", buildFields)
			query.Set("tree", treeValue)

			logger.Debug("Query: %v", query)

			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var data struct {
				Builds []json.RawMessage `json:"allBuilds"`
			}
			err := helper.UnmarshalResponse(res, &data)
			if err != nil {
				return nil, err
			}

			builds := make([]json.RawMessage, 0, len(data.Builds))
			for _, build := range data.Builds {
				var buildObj map[string]interface{}
				err := json.Unmarshal(build, &buildObj)
				if err != nil {
					return nil, errors.Convert(err)
				}
				if buildObj["result"] != nil {
					builds = append(builds, build)
				}
			}

			logger.Debug("Returning this number of builds: %v", len(builds))
			return builds, nil
		},
		AfterResponse: ignoreHTTPStatus404,
	})

	if err != nil {
		return err
	}

	return collectorWithState.Execute()
}
