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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
)

const RAW_INCIDENT_TYPES_TABLE = "datadog_incident_types"

var _ plugin.SubTaskEntryPoint = CollectIncidentTypes

var CollectIncidentTypesMeta = plugin.SubTaskMeta{
	Name:             "collectIncidentTypes",
	EntryPoint:       CollectIncidentTypes,
	EnabledByDefault: true,
	Description:      "Collect Datadog incident types",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables:    []string{RAW_INCIDENT_TYPES_TABLE},
}

func CollectIncidentTypes(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*DatadogTaskData)
	collector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENT_TYPES_TABLE,
		},
		ApiClient: data.Client,
		// The incident-types config endpoint returns the full list in one
		// response, so there is nothing to paginate.
		UrlTemplate: "incidents/config/types",
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			return parseDataEnvelope(res)
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

// parseDataEnvelope unwraps the JSON:API `data` array that every Datadog
// v2 collection response carries.
func parseDataEnvelope(res *http.Response) ([]json.RawMessage, errors.Error) {
	var body struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := api.UnmarshalResponse(res, &body); err != nil {
		return nil, err
	}
	return body.Data, nil
}
