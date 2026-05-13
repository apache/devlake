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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const RAW_INCIDENTS_TABLE = "rootly_incidents"

var _ plugin.SubTaskEntryPoint = CollectIncidents

// collectedIncidents is the JSON:API envelope for a paginated list of
// incidents. `data` is an array of raw resource objects (id, type,
// attributes, relationships) — one per incident — and `meta`/`links`
// drive pagination termination.
type collectedIncidents struct {
	Data  []json.RawMessage   `json:"data"`
	Meta  *collectedListMeta  `json:"meta"`
	Links *collectedListLinks `json:"links"`
}

type collectedListMeta struct {
	CurrentPage *int `json:"current_page"`
	TotalPages  *int `json:"total_pages"`
	TotalCount  *int `json:"total_count"`
}

type collectedListLinks struct {
	Next *string `json:"next"`
}

var CollectIncidentsMeta = plugin.SubTaskMeta{
	Name:             "collectIncidents",
	EntryPoint:       CollectIncidents,
	EnabledByDefault: true,
	Description:      "Collect Rootly incidents",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables:    []string{RAW_INCIDENTS_TABLE},
}

func CollectIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*RootlyTaskData)
	logger := taskCtx.GetLogger()
	logger.Info("[rootly] CollectIncidents: starting for serviceId=%s connectionId=%d", data.Options.ServiceId, data.Options.ConnectionId)
	args := api.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Options: data.Options,
		Table:   RAW_INCIDENTS_TABLE,
	}
	// lastPage captures the pagination signals from the most recent
	// ResponseParser invocation so GetNextPageCustomData can decide
	// whether to stop without re-reading prevPageResponse.Body, which
	// has already been drained by ResponseParser (http.Response.Body
	// is a single-read stream).
	var lastPage *collectedListMeta
	var lastLinksNext *string
	var lastPageEmpty bool

	collector, err := api.NewStatefulApiCollectorForFinalizableEntity(api.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: args,
		ApiClient:          data.Client,
		CollectNewRecordsByList: api.FinalizableApiCollectorListArgs{
			PageSize: 100,
			GetNextPageCustomData: func(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
				// The response body was already consumed by
				// ResponseParser; rely on the closure-captured
				// pagination state from that parse.
				if lastLinksNext != nil && *lastLinksNext != "" {
					return nil, nil
				}
				if lastPage != nil && lastPage.CurrentPage != nil && lastPage.TotalPages != nil {
					if *lastPage.CurrentPage >= *lastPage.TotalPages {
						return nil, api.ErrFinishCollect
					}
					return nil, nil
				}
				if lastPageEmpty {
					return nil, api.ErrFinishCollect
				}
				return nil, nil
			},
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate: "incidents",
				Query: func(reqData *api.RequestData, createdAfter *time.Time) (url.Values, errors.Error) {
					query := url.Values{}
					query.Set("filter[service_ids]", data.Options.ServiceId)
					query.Set("page[size]", fmt.Sprintf("%d", reqData.Pager.Size))
					// Rootly's JSON:API pagination is 1-based.
					pageNumber := reqData.Pager.Skip/reqData.Pager.Size + 1
					query.Set("page[number]", fmt.Sprintf("%d", pageNumber))
					query.Set("sort", "-updated_at")
					if createdAfter != nil {
						query.Set("filter[updated_at][gt]", createdAfter.UTC().Format(time.RFC3339))
					}
					logger.Debug("[rootly] incidents query: page=%d size=%d createdAfter=%v %s", pageNumber, reqData.Pager.Size, createdAfter, query.Encode())
					return query, nil
				},
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					rawResult := collectedIncidents{}
					if err := api.UnmarshalResponse(res, &rawResult); err != nil {
						logger.Error(err, "[rootly] incidents ResponseParser: unmarshal failed")
						return nil, err
					}
					metaStr := "nil"
					if rawResult.Meta != nil {
						metaStr = fmt.Sprintf("current=%s total_pages=%s total_count=%s",
							derefIntStr(rawResult.Meta.CurrentPage),
							derefIntStr(rawResult.Meta.TotalPages),
							derefIntStr(rawResult.Meta.TotalCount))
					}
					linksNextStr := "nil"
					if rawResult.Links != nil && rawResult.Links.Next != nil {
						linksNextStr = *rawResult.Links.Next
					}
					logger.Debug("[rootly] incidents response: status=%d count=%d meta=%s links.next=%s",
						res.StatusCode, len(rawResult.Data), metaStr, linksNextStr)
					lastPage = rawResult.Meta
					if rawResult.Links != nil {
						lastLinksNext = rawResult.Links.Next
					} else {
						lastLinksNext = nil
					}
					lastPageEmpty = len(rawResult.Data) == 0
					return rawResult.Data, nil
				},
			},
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

func derefIntStr(p *int) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *p)
}
