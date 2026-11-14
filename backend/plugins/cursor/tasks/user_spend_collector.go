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
	"io"
	"net/http"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

type cursorSpendCollectInput struct {
	CollectedAt time.Time `json:"collectedAt"`
}

type spendRawRecord struct {
	SubscriptionCycleStart int64           `json:"subscriptionCycleStart"`
	CollectedAt            time.Time       `json:"collectedAt"`
	Member                 json.RawMessage `json:"member"`
}

func newSpendIterator() *helper.QueueIterator {
	iter := helper.NewQueueIterator()
	iter.Push(cursorSpendCollectInput{CollectedAt: time.Now().UTC()})
	return iter
}

func parseSpendPageResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	if res == nil || res.Body == nil {
		return nil, errors.Default.New("response body is nil")
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to read spend response")
	}

	var response struct {
		SubscriptionCycleStart int64             `json:"subscriptionCycleStart"`
		TeamMemberSpend        []json.RawMessage `json:"teamMemberSpend"`
	}
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return nil, errors.Default.Wrap(errors.Convert(jsonErr), "failed to decode spend response")
	}

	collectedAt := time.Now().UTC()
	results := make([]json.RawMessage, 0, len(response.TeamMemberSpend))
	for _, member := range response.TeamMemberSpend {
		wrapped, marshalErr := json.Marshal(spendRawRecord{
			SubscriptionCycleStart: response.SubscriptionCycleStart,
			CollectedAt:            collectedAt,
			Member:                 member,
		})
		if marshalErr != nil {
			return nil, errors.Default.Wrap(errors.Convert(marshalErr), "failed to wrap spend record")
		}
		results = append(results, wrapped)
	}
	return results, nil
}

func spendPageBody(reqData *helper.RequestData) map[string]interface{} {
	page := 1
	if state, ok := reqData.CustomData.(cursorPageState); ok && state.Page > 0 {
		page = state.Page
	} else if reqData.Pager.Page > 0 {
		page = reqData.Pager.Page
	}
	return map[string]interface{}{
		"page":     page,
		"pageSize": reqData.Pager.Size,
	}
}

// nextSpendPage advances through POST /teams/spend pages. Unlike
// filtered-usage-events, this endpoint has no pagination object; it reports
// the page count as a top-level totalPages field.
func nextSpendPage(prevReqData *helper.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
	body, err := readResponseBody(prevPageResponse)
	if err != nil {
		return nil, err
	}
	var response struct {
		TotalPages int `json:"totalPages"`
	}
	if jsonErr := json.Unmarshal(body, &response); jsonErr != nil {
		return nil, errors.Default.Wrap(errors.Convert(jsonErr), "failed to decode spend pagination")
	}
	currentPage := 1
	if state, ok := prevReqData.CustomData.(cursorPageState); ok && state.Page > 0 {
		currentPage = state.Page
	} else if prevReqData.Pager != nil && prevReqData.Pager.Page > 0 {
		currentPage = prevReqData.Pager.Page
	}
	if currentPage >= response.TotalPages {
		return nil, helper.ErrFinishCollect
	}
	return cursorPageState{Page: currentPage + 1}, nil
}

// CollectUserSpend collects per-user billing cycle spend from POST /teams/spend.
func CollectUserSpend(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	apiClient, err := CreateApiClient(taskCtx.TaskContext(), data.Connection)
	if err != nil {
		return err
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Table:   rawUserSpendTable,
		Options: rawParamsFromTaskData(data),
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs:    rawArgs,
		ApiClient:             apiClient,
		Input:                 newSpendIterator(),
		Method:                http.MethodPost,
		UrlTemplate:           "teams/spend",
		PageSize:              cursorApiPageSize,
		RequestBody:           spendPageBody,
		GetNextPageCustomData: nextSpendPage,
		ResponseParser:        parseSpendPageResponse,
	})
	if err != nil {
		return err
	}

	return collector.Execute()
}
