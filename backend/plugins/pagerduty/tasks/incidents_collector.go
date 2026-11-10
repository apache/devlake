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
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/pagerduty/models"
)

const RAW_INCIDENTS_TABLE = "pagerduty_incidents"

var _ plugin.SubTaskEntryPoint = CollectIncidents

type (
	pagingInfo struct {
		Limit  *int  `json:"limit"`
		Offset *int  `json:"offset"`
		Total  *int  `json:"total"`
		More   *bool `json:"more"`
	}
	collectedIncidents struct {
		pagingInfo
		Incidents []json.RawMessage `json:"incidents"`
	}

	collectedIncident struct {
		pagingInfo
		Incident json.RawMessage `json:"incident"`
	}
	simplifiedRawIncident struct {
		Number    int       `json:"number"`
		CreatedAt time.Time `json:"created_at"`
	}
)

func CollectIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*PagerDutyTaskData)
	db := taskCtx.GetDal()
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
			GetNextPageCustomData: func(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
				pager := prevReqData.Pager
				if pager.Skip+pager.Size >= 10_000 { // API limit. Can't exceed this or it'll error out
					return nil, api.ErrFinishCollect
				}
				return nil, nil
			},
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate: "incidents",
				Query: func(reqData *api.RequestData, createdAfter *time.Time) (url.Values, errors.Error) {
					query := url.Values{}
					if createdAfter != nil {
						now := time.Now()
						if now.Sub(*createdAfter).Seconds() > 180*24*time.Hour.Seconds() {
							// beyond 6 months Pagerduty API will just return nothing, so need to query for 'all' instead
							query.Set("date_range", "all")
						} else {
							// since for PagerDuty is actually the created_at time of the incident (this is not well documented in their APIs)
							query.Set("since", createdAfter.String())
						}
					} else {
						query.Set("date_range", "all")
					}
					query.Set("service_ids[]", data.Options.ServiceId)
					query.Set("sort_by", "created_at:desc") // the newest entries will be fetched first
					query.Set("limit", fmt.Sprintf("%d", reqData.Pager.Size))
					query.Set("offset", fmt.Sprintf("%d", reqData.Pager.Skip))
					query.Set("total", "true")
					return query, nil
				},
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					rawResult := collectedIncidents{}
					err := api.UnmarshalResponse(res, &rawResult)
					return rawResult.Incidents, err
				},
			},
		},
		CollectUnfinishedDetails: &api.FinalizableApiCollectorDetailArgs{
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				// 2. "Input" here is the type: simplifiedRawIncident which is the element type of the returned iterator from BuildInputIterator
				UrlTemplate: "incidents/{{ .Input.Number }}",
				// 3. No custom query params/headers needed for this endpoint
				Query: nil,
				// 4. Parse the response for this endpoint call into a json.RawMessage
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					rawResult := collectedIncident{}
					err := api.UnmarshalResponse(res, &rawResult)
					return []json.RawMessage{rawResult.Incident}, err
				},
			},
			BuildInputIterator: func() (api.Iterator, errors.Error) {
				// 1. fetch individual "active/non-final" incidents from previous collections+extractions
				cursor, err := db.Cursor(
					dal.Select("number, created_date"),
					dal.From(&models.Incident{}),
					dal.Where(
						"service_id = ? AND connection_id = ? AND status != ?",
						data.Options.ServiceId, data.Options.ConnectionId, "resolved",
					),
				)
				if err != nil {
					return nil, err
				}
				return api.NewDalCursorIterator(db, cursor, reflect.TypeOf(simplifiedRawIncident{}))
			},
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

var CollectIncidentsMeta = plugin.SubTaskMeta{
	Name:             "collectIncidents",
	EntryPoint:       CollectIncidents,
	EnabledByDefault: true,
	Description:      "Collect PagerDuty incidents",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}
