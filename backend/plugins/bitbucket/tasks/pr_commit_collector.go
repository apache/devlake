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
	"fmt"
	"net/url"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

const RAW_PULL_REQUEST_COMMITS_TABLE = "bitbucket_api_pull_request_commits"

var CollectApiPrCommitsMeta = plugin.SubTaskMeta{
	Name:             "Collect PR Commits",
	EntryPoint:       CollectApiPullRequestCommits,
	EnabledByDefault: true,
	Description:      "Collect PullRequestCommits data from Bitbucket api",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
}

func CollectApiPullRequestCommits(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RAW_PULL_REQUEST_COMMITS_TABLE)
	collectorWithState, err := helper.NewStatefulApiCollector(*rawDataSubTaskArgs)
	if err != nil {
		return err
	}

	iterator, err := GetPullRequestsIterator(taskCtx, collectorWithState)
	if err != nil {
		return err
	}
	defer iterator.Close()

	err = collectorWithState.InitCollector(helper.ApiCollectorArgs{
		ApiClient:             data.ApiClient,
		PageSize:              100,
		Input:                 iterator,
		UrlTemplate:           "repositories/{{ .Params.FullName }}/pullrequests/{{ .Input.BitbucketId }}/commits",
		GetNextPageCustomData: GetNextPageCustomData,
		Query:                 getPrCommitsQuery,
		ResponseParser:        GetRawMessageFromResponse,
		// some pr have no commit
		// such as: https://bitbucket.org/amdatulabs/amdatu-kubernetes-deployer/pull-requests/21
		AfterResponse: ignoreHTTPStatus404,
	})
	if err != nil {
		return err
	}

	return collectorWithState.Execute()
}

// getPrCommitsQuery builds the query for one page of a pull request's commits.
// The fields list must include `next`: Bitbucket only returns the fields that are
// listed, and GetNextPageCustomData reads the next page from `next`.
func getPrCommitsQuery(reqData *helper.RequestData) (url.Values, errors.Error) {
	query := url.Values{}
	query.Set("state", "all")
	query.Set("pagelen", fmt.Sprintf("%v", reqData.Pager.Size))
	query.Set("sort", "created_on")
	query.Set("fields", "values.hash,values.date,values.message,values.author.raw,values.author.user.username,values.author.user.account_id,values.links.self,next")

	if reqData.CustomData != nil {
		query.Set("page", reqData.CustomData.(string))
	}
	return query, nil
}
