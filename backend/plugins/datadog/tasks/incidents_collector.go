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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
)

const RAW_INCIDENTS_TABLE = "datadog_incidents"

var _ plugin.SubTaskEntryPoint = CollectIncidents

var CollectIncidentsMeta = plugin.SubTaskMeta{
	Name:             "collectIncidents",
	EntryPoint:       CollectIncidents,
	EnabledByDefault: true,
	Description:      "Collect Datadog incidents",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables:    []string{RAW_INCIDENTS_TABLE},
}

func CollectIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*DatadogTaskData)
	var lastNextOffset *int
	collector, err := api.NewStatefulApiCollectorForFinalizableEntity(api.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		ApiClient: data.Client,
		CollectNewRecordsByList: api.FinalizableApiCollectorListArgs{
			PageSize: 100,
			// Pagination state captured in ResponseParser and read back in
			// GetNextPageCustomData: the response body is a single-read
			// stream and is already drained when the next-page hook fires.
			GetNextPageCustomData: func(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
				offset, ok := nextOffsetFrom(prevReqData.Pager.Skip, lastNextOffset)
				if !ok {
					return nil, api.ErrFinishCollect
				}
				return offset, nil
			},
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate: "incidents",
				// The list endpoint offers neither an incident-type filter nor
				// an updated-since filter, so every scope collects all
				// incidents and the extractor filters. createdAfter is
				// deliberately ignored for the same reason.
				Query: func(reqData *api.RequestData, createdAfter *time.Time) (url.Values, errors.Error) {
					offset := reqData.Pager.Skip
					if custom, ok := reqData.CustomData.(int); ok {
						offset = custom
					}
					return buildIncidentsQuery(reqData.Pager.Size, offset), nil
				},
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					data, next, err := parseIncidentsPage(res)
					lastNextOffset = next
					return data, err
				},
			},
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

// buildIncidentsQuery is the pure core of the Query closure above.
// Datadog paginates JSON:API collections with an offset in records, not a
// page number. It ignores an unknown `page[limit]`, so the name matters.
func buildIncidentsQuery(pageSize int, offset int) url.Values {
	query := url.Values{}
	query.Set("page[size]", fmt.Sprintf("%d", pageSize))
	query.Set("page[offset]", fmt.Sprintf("%d", offset))
	return query
}

// parseIncidentsPage unwraps the JSON:API envelope and reports where the
// next page starts, as Datadog itself declares it.
func parseIncidentsPage(res *http.Response) ([]json.RawMessage, *int, errors.Error) {
	var body struct {
		Data []json.RawMessage `json:"data"`
		Meta *struct {
			Pagination *struct {
				Offset     int  `json:"offset"`
				Size       int  `json:"size"`
				NextOffset *int `json:"next_offset"`
			} `json:"pagination"`
		} `json:"meta"`
	}
	if err := api.UnmarshalResponse(res, &body); err != nil {
		return nil, nil, err
	}
	if body.Meta != nil && body.Meta.Pagination != nil {
		return body.Data, body.Meta.Pagination.NextOffset, nil
	}
	return body.Data, nil, nil
}

// nextOffsetFrom decides whether to ask for another page. The last page
// carries no next_offset; an offset that fails to advance would loop
// forever, so treat that as the end too.
func nextOffsetFrom(prevOffset int, next *int) (int, bool) {
	if next == nil || *next <= prevOffset {
		return 0, false
	}
	return *next, true
}
