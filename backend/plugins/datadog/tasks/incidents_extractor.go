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

var _ plugin.SubTaskEntryPoint = ExtractIncidents

var ExtractIncidentsMeta = plugin.SubTaskMeta{
	Name:             "extractIncidents",
	EntryPoint:       ExtractIncidents,
	EnabledByDefault: true,
	Description:      "Extract Datadog incidents",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables:    []string{models.Incident{}.TableName()},
}

func ExtractIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*DatadogTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			return extractIncident(row.Data, data)
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func extractIncident(rawData []byte, data *DatadogTaskData) ([]interface{}, errors.Error) {
	rawIncident := &raw.Incident{}
	if err := errors.Convert(json.Unmarshal(rawData, rawIncident)); err != nil {
		return nil, err
	}
	attributes := rawIncident.Attributes
	if attributes == nil {
		return nil, nil
	}

	// Test incidents are drills. Counting them would inflate every
	// incident metric they touch.
	if derefBool(attributes.IsTest) {
		return nil, nil
	}

	op := data.Options
	// The collector fetches all incidents, so the scope filter lives here.
	// An empty IncidentTypeId means this task collects every type.
	if op.IncidentTypeId != "" && derefString(attributes.IncidentTypeUuid) != op.IncidentTypeId {
		return nil, nil
	}

	if attributes.Created == nil {
		return nil, errors.Default.New("datadog incident missing created timestamp")
	}

	incident := &models.Incident{
		ConnectionId:     op.ConnectionId,
		Id:               rawIncident.Id,
		PublicId:         attributes.PublicId,
		IncidentTypeId:   derefString(attributes.IncidentTypeUuid),
		Title:            attributes.Title,
		Url:              incidentUrl(data.WebUrl, attributes.PublicId),
		State:            derefString(attributes.State),
		Severity:         derefString(attributes.Severity),
		IsTest:           false,
		CustomerImpacted: derefBool(attributes.CustomerImpacted),
		CreatedDate:      *attributes.Created,
		DeclaredDate:     attributes.Declared,
		DetectedDate:     attributes.Detected,
		ResolvedDate:     attributes.Resolved,
		ModifiedDate:     attributes.Modified,
		TimeToDetect:     attributes.TimeToDetect,
		TimeToRepair:     attributes.TimeToRepair,
		TimeToResolve:    attributes.TimeToResolve,
	}

	if len(attributes.Fields) > 0 {
		encoded, err := errors.Convert01(json.Marshal(attributes.Fields))
		if err != nil {
			return nil, err
		}
		incident.CustomFields = string(encoded)
	}

	// Custom fields carry organization-specific vocabulary, so which field
	// means what is configuration rather than plugin knowledge.
	if field := op.ComponentField(); field != "" {
		incident.Component = firstFieldValue(attributes.Fields, field)
	}
	if field := op.SeverityField(); field != "" {
		if value := firstFieldValue(attributes.Fields, field); value != "" {
			incident.Severity = value
		}
	}

	return []interface{}{incident}, nil
}

// firstFieldValue takes the first value of a custom field. Multi-select
// fields hold several; the domain layer has room for one, and the raw
// object is kept so nothing is lost.
func firstFieldValue(fields map[string]raw.FieldAttributes, name string) string {
	field, ok := fields[name]
	if !ok {
		return ""
	}
	values := field.Values()
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
