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
	collector, err := api.NewStatefulApiCollectorForFinalizableEntity(api.FinalizableApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		ApiClient: data.Client,
		CollectNewRecordsByList: api.FinalizableApiCollectorListArgs{
			PageSize: 100,
			FinalizableApiCollectorCommonArgs: api.FinalizableApiCollectorCommonArgs{
				UrlTemplate: "incidents",
				// The list endpoint offers neither an incident-type filter nor
				// an updated-since filter, so every scope collects all
				// incidents and the extractor filters. createdAfter is
				// deliberately ignored for the same reason.
				Query: func(reqData *api.RequestData, createdAfter *time.Time) (url.Values, errors.Error) {
					return buildIncidentsQuery(reqData.Pager.Size, reqData.Pager.Skip), nil
				},
				ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
					return parseDataEnvelope(res)
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
// page number.
func buildIncidentsQuery(pageSize int, offset int) url.Values {
	query := url.Values{}
	query.Set("page[size]", fmt.Sprintf("%d", pageSize))
	query.Set("page[offset]", fmt.Sprintf("%d", offset))
	return query
}
