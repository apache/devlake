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

type collectedIncidents struct {
	Data  []json.RawMessage   `json:"data"`
	Meta  *collectedListMeta  `json:"meta"`
	Links *collectedListLinks `json:"links"`
}

type collectedListMeta struct {
	CurrentPage *int `json:"current_page"`
	TotalPages  *int `json:"total_pages"`
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
	args := api.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Options: data.Options,
		Table:   RAW_INCIDENTS_TABLE,
	}
	// Pagination state captured during ResponseParser and consulted in
	// GetNextPageCustomData. Required because prevPageResponse.Body is
	// a single-read stream and is already drained by the time the
	// next-page hook fires.
	var lastPage *collectedListMeta
	var lastLinksNext *string

	collector, err := api.NewStatefulApiCollectorForFinalizableEntity(api.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: args,
		ApiClient:          data.Client,
		CollectNewRecordsByList: api.FinalizableApiCollectorListArgs{
			PageSize: 100,
			GetNextPageCustomData: func(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
				// Safety cap against an upstream that returns full pages forever
				// without populating either meta.total_pages or links.next.
				const maxPages = 10000
				if prevReqData.Pager.Page >= maxPages {
					return nil, api.ErrFinishCollect
				}
				if lastLinksNext != nil && *lastLinksNext != "" {
					return nil, nil
				}
				if lastPage != nil && lastPage.CurrentPage != nil && lastPage.TotalPages != nil {
					if *lastPage.CurrentPage >= *lastPage.TotalPages {
						return nil, api.ErrFinishCollect
					}
				}
				return nil, nil
			},
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate: "incidents",
				Query: func(reqData *api.RequestData, createdAfter *time.Time) (url.Values, errors.Error) {
					return buildIncidentsQuery(data.Options.ServiceId, reqData.Pager.Size, reqData.Pager.Page, createdAfter), nil
				},
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					rawResult := collectedIncidents{}
					if err := api.UnmarshalResponse(res, &rawResult); err != nil {
						return nil, err
					}
					lastPage = rawResult.Meta
					if rawResult.Links != nil {
						lastLinksNext = rawResult.Links.Next
					} else {
						lastLinksNext = nil
					}
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

// buildIncidentsQuery is the pure-function core of the Query closure
// above so a regression in the filter parameter name (we shipped with
// `filter[services]` once and got 0 results back; the correct param is
// `filter[service_ids]`) is caught by a unit test.
func buildIncidentsQuery(serviceId string, pageSize, pageNumber int, createdAfter *time.Time) url.Values {
	query := url.Values{}
	query.Set("filter[service_ids]", serviceId)
	query.Set("page[size]", fmt.Sprintf("%d", pageSize))
	// Rootly's JSON:API pagination is 1-based.
	query.Set("page[number]", fmt.Sprintf("%d", pageNumber))
	query.Set("sort", "-updated_at")
	if createdAfter != nil {
		query.Set("filter[updated_at][gt]", createdAfter.UTC().Format(time.RFC3339))
	}
	return query
}
