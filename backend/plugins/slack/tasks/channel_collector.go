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
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/slack/apimodels"
	"net/http"
	"net/url"
)

const RAW_CHANNEL_TABLE = "slack_channel"

var _ plugin.SubTaskEntryPoint = CollectChannel

// CollectChannel collect all channels that bot is in
func CollectChannel(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*SlackTaskData)
	collector, err := api.NewApiCollector(newChannelCollectorArgs(
		api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: SlackApiParams{
				ConnectionId: data.Options.ConnectionId,
			},
			Table: RAW_CHANNEL_TABLE,
		},
		data.ApiClient,
	))
	if err != nil {
		return err
	}

	return collector.Execute()
}

func newChannelCollectorArgs(rawArgs api.RawDataSubTaskArgs, apiClient api.RateLimitedApiClient) api.ApiCollectorArgs {
	limit := 100
	return api.ApiCollectorArgs{
		RawDataSubTaskArgs:    rawArgs,
		ApiClient:             apiClient,
		Incremental:           false,
		UrlTemplate:           "conversations.list",
		PageSize:              slackCollectorPageSize,
		GetNextPageCustomData: getSlackNextCursor,
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			setSlackCursorQuery(query, reqData, limit)
			return query, nil
		},
		ResponseParser: parseChannelResponse,
	}
}

func parseChannelResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	body := &apimodels.SlackChannelApiResult{}
	err := api.UnmarshalResponse(res, body)
	if err != nil {
		return nil, err
	}
	if err = checkSlackApiResult("conversations.list", body.Ok, body.Error); err != nil {
		return nil, err
	}
	return body.Channels, nil
}

var CollectChannelMeta = plugin.SubTaskMeta{
	Name:             "collectChannel",
	EntryPoint:       CollectChannel,
	EnabledByDefault: true,
	Description:      "Collect channels from Slack api",
}
