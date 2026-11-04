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

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira/tasks/apiv2models"
)

const RAW_WORKLOGS_TABLE = "jira_api_worklogs"

var CollectWorklogsMeta = plugin.SubTaskMeta{
	Name:             "collectWorklogs",
	EntryPoint:       CollectWorklogs,
	EnabledByDefault: true,
	Description:      "collect Jira work logs, supports both timeFilter and diffSync.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func CollectWorklogs(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*JiraTaskData)

	logger := taskCtx.GetLogger()

	apiCollector, err := api.NewStatefulApiCollector(api.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: JiraApiParams{
			ConnectionId: data.Options.ConnectionId,
			BoardId:      data.Options.BoardId,
		},
		Table: RAW_WORKLOGS_TABLE,
	})
	if err != nil {
		return err
	}

	// filter out issue_ids that needed collection
	clauses := []dal.Clause{
		dal.Select("i.issue_id AS issue_id, i.updated AS update_time"),
		dal.From("_tool_jira_board_issues bi"),
		dal.Join("LEFT JOIN _tool_jira_issues i ON (bi.connection_id = i.connection_id AND bi.issue_id = i.issue_id)"),
		dal.Where("bi.connection_id=? and bi.board_id = ? AND i.std_type != ? and i.worklog_total > 20", data.Options.ConnectionId, data.Options.BoardId, "Epic"),
	}
	if apiCollector.IsIncremental() && apiCollector.GetSince() != nil {
		clauses = append(clauses, dal.Where("i.updated > ?", apiCollector.GetSince()))
	}

	// construct the input iterator
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(apiv2models.Input{}))
	if err != nil {
		return err
	}

	err = apiCollector.InitCollector(api.ApiCollectorArgs{
		Input:         iterator,
		ApiClient:     data.ApiClient,
		UrlTemplate:   "api/2/issue/{{ .Input.IssueId }}/worklog",
		PageSize:      50,
		GetTotalPages: GetTotalPagesFromResponse,
		Query:         buildWorklogQuery,
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var data struct {
				Worklogs []json.RawMessage `json:"worklogs"`
			}
			err := api.UnmarshalResponse(res, &data)
			if err != nil {
				return nil, err
			}
			return data.Worklogs, nil
		},
		AfterResponse: ignoreHTTPStatus404,
	})
	if err != nil {
		logger.Error(err, "collect board error")
		return err
	}

	return apiCollector.Execute()
}

// buildWorklogQuery sets the page offset and size. GetTotalPagesFromResponse makes the collector
// request one URL per page, so without startAt every page would return the first worklogs again.
func buildWorklogQuery(reqData *api.RequestData) (url.Values, errors.Error) {
	query := url.Values{}
	query.Set("startAt", fmt.Sprintf("%v", reqData.Pager.Skip))
	query.Set("maxResults", fmt.Sprintf("%v", reqData.Pager.Size))
	return query, nil
}
