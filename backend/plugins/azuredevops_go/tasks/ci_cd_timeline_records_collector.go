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
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/azuredevops_go/models"
	"reflect"
)

func init() {
	RegisterSubtaskMeta(&CollectJobsMeta)
}

const RawTimelineRecordTable = "azuredevops_go_api_timeline_records"

var CollectJobsMeta = plugin.SubTaskMeta{
	Name:             "collectApiTimelineRecords",
	EntryPoint:       CollectRecords,
	EnabledByDefault: true,
	Description:      "Collect Timeline Records data from Azure DevOps API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{models.AzuredevopsBuild{}.TableName()},
	ProductTables:    []string{RawTimelineRecordTable},
}

func CollectRecords(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawTimelineRecordTable)

	db := taskCtx.GetDal()
	cursor, err := db.Cursor(
		dal.Select("azuredevops_id"),
		dal.From(models.AzuredevopsBuild{}.TableName()),
		dal.Where("repository_id = ? and connection_id=? and result NOT IN ?", data.Options.RepositoryId, data.Options.ConnectionId, []string{"failed", "none"}),
	)
	if err != nil {
		return err
	}
	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(SimplePr{}))
	if err != nil {
		return err
	}

	// The timeline api does not support any kind of pagination
	collector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		ApiClient:          data.ApiClient,
		Input:              iterator,
		Incremental:        false,
		UrlTemplate:        "{{ .Params.OrganizationId }}/{{ .Params.ProjectId }}/_apis/build/builds/{{ .Input.AzuredevopsId }}/Timeline?api-version=7.1",
		Query:              BuildPaginator(true),
		ResponseParser:     ParseRawMessageFromRecords,
		AfterResponse:      ignoreInvalidTimelineResponse, // Skip builds with missing/malformed timelines (e.g. YAML syntax errors)
	})
	if err != nil {
		return err
	}

	return collector.Execute()
}
