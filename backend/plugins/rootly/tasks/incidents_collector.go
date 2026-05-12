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
	args := api.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Options: data.Options,
		Table:   RAW_INCIDENTS_TABLE,
	}
	collector, err := api.NewStatefulApiCollectorForFinalizableEntity(api.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: args,
		ApiClient:          data.Client,
		CollectNewRecordsByList: api.FinalizableApiCollectorListArgs{
			PageSize: 100,
			// GetNextPageCustomData terminates pagination by reading the
			// JSON:API links.next / meta.current_page / meta.total_pages
			// fields from the previous response. If either signal says
			// "no more pages", return ErrFinishCollect.
			GetNextPageCustomData: func(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
				if prevPageResponse == nil {
					return nil, nil
				}
				parsed := collectedIncidents{}
				if perr := api.UnmarshalResponse(prevPageResponse, &parsed); perr != nil {
					return nil, perr
				}
				if parsed.Links != nil && parsed.Links.Next != nil && *parsed.Links.Next != "" {
					return nil, nil
				}
				if parsed.Meta != nil && parsed.Meta.CurrentPage != nil && parsed.Meta.TotalPages != nil {
					if *parsed.Meta.CurrentPage >= *parsed.Meta.TotalPages {
						return nil, api.ErrFinishCollect
					}
					return nil, nil
				}
				// No signal either way — if the page came back empty,
				// stop. Otherwise continue and let the next empty page
				// terminate us.
				if len(parsed.Data) == 0 {
					return nil, api.ErrFinishCollect
				}
				return nil, nil
			},
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate: "incidents",
				Query: func(reqData *api.RequestData, createdAfter *time.Time) (url.Values, errors.Error) {
					query := url.Values{}
					query.Set("filter[services]", data.Options.ServiceId)
					query.Set("page[size]", fmt.Sprintf("%d", reqData.Pager.Size))
					// Rootly's JSON:API pagination is 1-based.
					pageNumber := reqData.Pager.Skip/reqData.Pager.Size + 1
					query.Set("page[number]", fmt.Sprintf("%d", pageNumber))
					query.Set("sort", "-updated_at")
					if createdAfter != nil {
						query.Set("filter[updated_at][gt]", createdAfter.UTC().Format(time.RFC3339))
					}
					return query, nil
				},
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					rawResult := collectedIncidents{}
					if err := api.UnmarshalResponse(res, &rawResult); err != nil {
						return nil, err
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
