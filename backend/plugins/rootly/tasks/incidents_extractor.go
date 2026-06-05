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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/rootly/models"
	"github.com/apache/incubator-devlake/plugins/rootly/models/raw"
)

var _ plugin.SubTaskEntryPoint = ExtractIncidents

var ExtractIncidentsMeta = plugin.SubTaskMeta{
	Name:             "extractIncidents",
	EntryPoint:       ExtractIncidents,
	EnabledByDefault: true,
	Description:      "Extract Rootly incidents",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables:    []string{models.Incident{}.TableName(), models.User{}.TableName()},
}

func ExtractIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*RootlyTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			return extractRootlyIncident(row.Data, data.Options)
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func extractRootlyIncident(rawData []byte, op *RootlyOptions) ([]interface{}, errors.Error) {
	rawIncident := &raw.Incident{}
	if err := errors.Convert(json.Unmarshal(rawData, rawIncident)); err != nil {
		return nil, err
	}

	// Safety net: filter[service_ids] in the collector is the primary
	// scope filter, but a regression there would let multi-service
	// incidents leak into a wrong scope's tool table. When ServiceId is
	// empty we are collecting all incidents globally, so skip this check.
	if op.ServiceId != "" {
		if services := rawIncident.Relationships.Services.Data; len(services) > 0 && !containsServiceId(services, op.ServiceId) {
			return nil, nil
		}
	}

	if rawIncident.Attributes.StartedAt.IsZero() {
		return nil, errors.Default.New("rootly incident missing started_at")
	}

	incident := &models.Incident{
		ConnectionId:     op.ConnectionId,
		Id:               rawIncident.Id,
		Number:           resolve(rawIncident.Attributes.SequentialId),
		ServiceId:        op.ServiceId,
		Url:              resolve(rawIncident.Attributes.Url),
		Title:            rawIncident.Attributes.Title,
		Summary:          resolve(rawIncident.Attributes.Summary),
		Status:           rawIncident.Attributes.Status,
		Severity:         resolveSeverity(rawIncident.Attributes.Severity),
		StartedDate:      rawIncident.Attributes.StartedAt,
		AcknowledgedDate: rawIncident.Attributes.AcknowledgedAt,
		MitigatedDate:    rawIncident.Attributes.MitigatedAt,
		ResolvedDate:     rawIncident.Attributes.ResolvedAt,
		UpdatedDate:      rawIncident.Attributes.UpdatedAt,
	}

	results := []interface{}{incident}
	seen := map[string]bool{}
	addUser := func(u *raw.UserEnvelope, setRoleId func(string)) {
		if u == nil || u.Data.Id == "" {
			return
		}
		setRoleId(u.Data.Id)
		if seen[u.Data.Id] {
			return
		}
		seen[u.Data.Id] = true
		name := pickUserName(u.Data.Attributes)
		// Skip rows with no useful data so a sibling scope task that has
		// fuller data for the same user doesn't get overwritten with blanks.
		if name == "" && u.Data.Attributes.Email == "" {
			return
		}
		results = append(results, &models.User{
			ConnectionId: op.ConnectionId,
			Id:           u.Data.Id,
			Email:        u.Data.Attributes.Email,
			Name:         name,
		})
	}
	addUser(rawIncident.Attributes.User, func(id string) { incident.CreatorUserId = id })
	addUser(rawIncident.Attributes.StartedBy, func(id string) { incident.StartedByUserId = id })
	addUser(rawIncident.Attributes.MitigatedBy, func(id string) { incident.MitigatedByUserId = id })
	addUser(rawIncident.Attributes.ResolvedBy, func(id string) { incident.ResolvedByUserId = id })
	addUser(rawIncident.Attributes.ClosedBy, func(id string) { incident.ClosedByUserId = id })

	return results, nil
}

func pickUserName(u raw.UserAttributes) string {
	if u.FullName != "" {
		return u.FullName
	}
	if u.Name != "" {
		return u.Name
	}
	return u.Email
}

func containsServiceId(services []raw.ServiceRef, serviceId string) bool {
	for _, s := range services {
		if s.Id == serviceId {
			return true
		}
	}
	return false
}

func resolveSeverity(s *raw.SeverityEnvelope) string {
	if s == nil {
		return ""
	}
	return s.Data.Attributes.Slug
}

func resolve[T any](t *T) T {
	if t == nil {
		return *new(T)
	}
	return *t
}
