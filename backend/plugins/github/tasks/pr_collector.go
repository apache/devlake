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
	"io"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&CollectApiPullRequestsMeta)
}

const RAW_PULL_REQUEST_TABLE = "github_api_pull_requests"

var CollectApiPullRequestsMeta = plugin.SubTaskMeta{
	Name:             "Collect Pull Requests",
	EntryPoint:       CollectApiPullRequests,
	EnabledByDefault: true,
	Description:      "Collect PullRequests data from Github api, supports both timeFilter and diffSync.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS, plugin.DOMAIN_TYPE_CODE_REVIEW},
	DependencyTables: []string{},
	ProductTables:    []string{RAW_PULL_REQUEST_TABLE},
}

type SimpleGithubPr struct {
	Number int64
}

type SimpleGithubApiPr struct {
	CreatedAt time.Time `json:"created_at"`
}

func CollectApiPullRequests(taskCtx plugin.SubTaskContext) errors.Error {
	// pull requests are Finalizable, they can't be re-open once closed
	data := taskCtx.GetData().(*GithubTaskData)
	db := taskCtx.GetDal()

	collector, err := helper.NewStatefulApiCollectorForFinalizableEntity(helper.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_PULL_REQUEST_TABLE,
		},
		ApiClient: data.ApiClient,
		CollectNewRecordsByList: helper.FinalizableApiCollectorListArgs{
			PageSize:    100,
			Concurrency: 10,
			FinalizableApiCollectorCommonArgs: helper.FinalizableApiCollectorCommonArgs{
				UrlTemplate: "repos/{{ .Params.Name }}/pulls",
				Query: func(reqData *helper.RequestData, createdAfter *time.Time) (url.Values, errors.Error) {
					query := url.Values{}
					query.Set("state", "all")
					query.Set("direction", "desc")
					query.Set("page", fmt.Sprintf("%v", reqData.Pager.Page))
					query.Set("per_page", fmt.Sprintf("%v", reqData.Pager.Size))
					return query, nil
				},
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					var items []json.RawMessage
					err := helper.UnmarshalResponse(res, &items)
					if err != nil {
						return nil, err
					}
					return items, nil
				},
			},
			GetCreated: func(item json.RawMessage) (time.Time, errors.Error) {
				pr := &SimpleGithubApiPr{}
				err := json.Unmarshal(item, pr)
				if err != nil {
					return time.Time{}, errors.BadInput.Wrap(err, "failed to unmarshal github pull request")
				}
				return pr.CreatedAt, nil
			},
		},
		CollectUnfinishedDetails: &helper.FinalizableApiCollectorDetailArgs{
			BuildInputIterator: func() (helper.Iterator, errors.Error) {
				// select pull id from database
				cursor, err := db.Cursor(
					dal.Select("number"),
					dal.From(&models.GithubPullRequest{}),
					dal.Where(
						"repo_id = ? AND connection_id = ? AND (state != 'closed' OR merged = false)",
						data.Options.GithubId, data.Options.ConnectionId,
					),
				)
				if err != nil {
					return nil, err
				}
				return helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(SimpleGithubPr{}))
			},
			FinalizableApiCollectorCommonArgs: helper.FinalizableApiCollectorCommonArgs{
				UrlTemplate: "repos/{{ .Params.Name }}/pulls/{{ .Input.Number }}",
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					body, err := io.ReadAll(res.Body)
					if err != nil {
						return nil, errors.Convert(err)
					}
					res.Body.Close()
					return []json.RawMessage{body}, nil
				},
				AfterResponse: ignoreHTTPStatus404,
			},
		},
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}
