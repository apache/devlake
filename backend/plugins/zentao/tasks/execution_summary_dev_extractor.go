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
	"strconv"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/zentao/models"
)

var _ plugin.SubTaskEntryPoint = ExtractExecutionSummaryDev

var ExtractExecutionSummaryDevMeta = plugin.SubTaskMeta{
	Name:             "extractExecutionSummaryDev",
	EntryPoint:       ExtractExecutionSummaryDev,
	EnabledByDefault: true,
	Description:      "extract Zentao execution summary from built-in page api",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ExtractExecutionSummaryDev(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*ZentaoTaskData)

	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_EXECUTION_SUMMARY_DEV_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			executionSummary := &models.ZentaoExecutionSummary{}
			executionSummary.ConnectionId = data.Options.ConnectionId
			executionSummary.Project = data.Options.ProjectId
			// for example:
			// "{\"locate\":\"http:\\\/\\\/*.*.*.*\\\/zentao\\\/execution-task-259.json\"}"
			parts := strings.Split(string(row.Data), "-")
			value := strings.Split(parts[len(parts)-1], ".")[0]
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				// does not meet the format, skip it
				return nil, nil
			}
			executionSummary.Id = id

			return []interface{}{executionSummary}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
