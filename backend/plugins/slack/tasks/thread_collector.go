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
	"net/url"
	"reflect"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/slack/apimodels"
)

const RAW_THREAD_TABLE = "slack_thread"

var _ plugin.SubTaskEntryPoint = CollectThread

type ThreadInput struct {
	ChannelId string `json:"channel_id"`
	ThreadTs  string `json:"thread_ts"`
}

func CollectThread(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*SlackTaskData)
	db := taskCtx.GetDal()

	clauses := []dal.Clause{
		dal.Select("DISTINCT CASE WHEN thread_ts = '' OR thread_ts IS NULL THEN ts ELSE thread_ts END AS thread_ts, channel_id"),
		dal.From("_tool_slack_channel_messages"),
		dal.Where("connection_id = ? AND channel_id = ? AND reply_count > 0 AND (subtype = '' OR subtype IS NULL)", data.Options.ConnectionId, data.Options.ChannelId),
	}

	// construct the input iterator
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	// smaller struct can reduce memory footprint, we should try to avoid using big struct
	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(ThreadInput{}))
	if err != nil {
		return err
	}

	collector, err := api.NewApiCollector(newThreadCollectorArgs(
		api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_THREAD_TABLE,
		},
		data.ApiClient,
		iterator,
	))
	if err != nil {
		return err
	}

	return collector.Execute()
}

func newThreadCollectorArgs(rawArgs api.RawDataSubTaskArgs, apiClient api.RateLimitedApiClient, input api.Iterator) api.ApiCollectorArgs {
	limit := 50
	return api.ApiCollectorArgs{
		RawDataSubTaskArgs:    rawArgs,
		ApiClient:             apiClient,
		Incremental:           false,
		Input:                 input,
		UrlTemplate:           "conversations.replies",
		PageSize:              slackCollectorPageSize,
		GetNextPageCustomData: getSlackNextCursor,
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(*ThreadInput)
			query := url.Values{}
			query.Set("channel", input.ChannelId)
			query.Set("ts", input.ThreadTs)
			setSlackCursorQuery(query, reqData, limit)
			return query, nil
		},
		ResponseParser: parseThreadResponse,
	}
}

func parseThreadResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	body := &apimodels.SlackThreadsApiResult{}
	err := api.UnmarshalResponse(res, body)
	if err != nil {
		return nil, err
	}
	// The parent message was deleted after the channel messages were collected.
	// Skip that thread instead of failing the whole collection.
	if !body.Ok && body.Error == "thread_not_found" {
		return nil, nil
	}
	if err = checkSlackApiResult("conversations.replies", body.Ok, body.Error); err != nil {
		return nil, err
	}
	return body.Threads, nil
}

var CollectThreadMeta = plugin.SubTaskMeta{
	Name:             "collectThread",
	EntryPoint:       CollectThread,
	EnabledByDefault: true,
	Description:      "Collect thread from Slack api",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
}
