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
	"github.com/apache/devlake/plugins/datadog/models"
	"github.com/apache/devlake/plugins/datadog/models/raw"
)

var _ plugin.SubTaskEntryPoint = ExtractIncidentTypes

var ExtractIncidentTypesMeta = plugin.SubTaskMeta{
	Name:             "extractIncidentTypes",
	EntryPoint:       ExtractIncidentTypes,
	EnabledByDefault: true,
	Description:      "Extract Datadog incident types",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables:    []string{models.IncidentType{}.TableName()},
}

func ExtractIncidentTypes(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*DatadogTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENT_TYPES_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			rawType := &raw.IncidentType{}
			if err := errors.Convert(json.Unmarshal(row.Data, rawType)); err != nil {
				return nil, err
			}
			if rawType.Attributes == nil {
				return nil, nil
			}
			// One task runs per scope, so drop the types this scope is not
			// about; an empty IncidentTypeId means "collect everything".
			if data.Options.IncidentTypeId != "" && rawType.Id != data.Options.IncidentTypeId {
				return nil, nil
			}
			incidentType := &models.IncidentType{
				Id:        rawType.Id,
				Name:      rawType.Attributes.Name,
				Prefix:    derefString(rawType.Attributes.Prefix),
				IsDefault: derefBool(rawType.Attributes.IsDefault),
			}
			incidentType.ConnectionId = data.Options.ConnectionId
			return []interface{}{incidentType}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
