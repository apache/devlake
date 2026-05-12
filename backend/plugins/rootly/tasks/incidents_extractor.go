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

// extractRootlyIncident is the pure-function core of the extractor:
// take a raw JSON:API incident envelope + the task options, return the
// tool-layer rows to persist. Factored out of the closure so it can be
// unit-tested without a SubTaskContext.
//
// Output shape per incident:
//   - exactly one *models.Incident (always), with role-specific user-id
//     fields populated from the nested attributes.user / started_by /
//     mitigated_by / resolved_by / closed_by blocks
//   - zero-to-N *models.User rows, one per distinct user id seen across
//     those role fields (deduplicated within a single incident — if the
//     same user is both creator and resolver, only one User row is emitted)
//
// The nested user objects are plain JSON on the incident's attributes,
// NOT JSON:API-wrapped and NOT surfaced through a relationships
// `included` array. That is the whole reason this extractor can emit
// users directly without a separate users-collector.
func extractRootlyIncident(rawData []byte, op *RootlyOptions) ([]interface{}, errors.Error) {
	rawIncident := &raw.Incident{}
	if err := errors.Convert(json.Unmarshal(rawData, rawIncident)); err != nil {
		return nil, err
	}

	// Safety-net scope filter. The collector already sends
	// filter[services]=<op.ServiceId>, but if Rootly ever returns an
	// incident that touches multiple services (or the filter regresses),
	// dropping anything whose relationships.services.data does not
	// include our scoped service keeps the tool table clean. When the
	// envelope has no relationships at all we accept the incident — the
	// API-side filter is the only scoping signal we have.
	if len(rawIncident.Relationships) > 0 {
		relationships := raw.IncidentRelationships{}
		// Ignore unmarshal errors here: a malformed relationships block
		// should not fail the entire row — fall through to accept.
		if err := json.Unmarshal(rawIncident.Relationships, &relationships); err == nil {
			if len(relationships.Services.Data) > 0 && !containsService(relationships.Services.Data, op.ServiceId) {
				return nil, nil
			}
		}
	}

	if rawIncident.Attributes.StartedAt.IsZero() {
		return nil, errors.Default.New("rootly incident missing started_at")
	}

	incident := &models.Incident{
		ConnectionId:     op.ConnectionId,
		Id:               rawIncident.Id,
		Number:           resolveInt(rawIncident.Attributes.SequentialId),
		ServiceId:        op.ServiceId,
		Url:              resolve(rawIncident.Attributes.Url),
		Title:            rawIncident.Attributes.Title,
		Summary:          resolve(rawIncident.Attributes.Summary),
		Status:           rawIncident.Attributes.Status,
		Severity:         resolveSeverity(rawIncident.Attributes),
		Urgency:          resolve(rawIncident.Attributes.Urgency),
		StartedDate:      rawIncident.Attributes.StartedAt,
		AcknowledgedDate: rawIncident.Attributes.AcknowledgedAt,
		MitigatedDate:    rawIncident.Attributes.MitigatedAt,
		ResolvedDate:     rawIncident.Attributes.ResolvedAt,
		UpdatedDate:      rawIncident.Attributes.UpdatedAt,
	}

	results := []interface{}{incident}
	seen := map[string]bool{}
	addUser := func(u *raw.NestedUser, setRoleId func(string)) {
		if u == nil || u.Id == "" {
			return
		}
		setRoleId(u.Id)
		if seen[u.Id] {
			return
		}
		seen[u.Id] = true
		results = append(results, &models.User{
			ConnectionId: op.ConnectionId,
			Id:           u.Id,
			Email:        resolve(u.Email),
			Name:         pickUserName(u),
			Url:          resolve(u.Url),
		})
	}
	addUser(rawIncident.Attributes.User, func(id string) { incident.CreatorUserId = id })
	addUser(rawIncident.Attributes.StartedBy, func(id string) { incident.StartedByUserId = id })
	addUser(rawIncident.Attributes.MitigatedBy, func(id string) { incident.MitigatedByUserId = id })
	addUser(rawIncident.Attributes.ResolvedBy, func(id string) { incident.ResolvedByUserId = id })
	addUser(rawIncident.Attributes.ClosedBy, func(id string) { incident.ClosedByUserId = id })

	return results, nil
}

// pickUserName chooses the best display name for a nested user:
// FullName when set, otherwise Name, otherwise Email, otherwise empty.
// Email is a last-resort fallback so the User row is never nameless.
func pickUserName(u *raw.NestedUser) string {
	if u.FullName != nil && *u.FullName != "" {
		return *u.FullName
	}
	if u.Name != nil && *u.Name != "" {
		return *u.Name
	}
	if u.Email != nil {
		return *u.Email
	}
	return ""
}

// containsService checks whether the given service id appears in the
// JSON:API relationships.services.data array.
func containsService(data []struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}, serviceId string) bool {
	for _, s := range data {
		if s.Id == serviceId {
			return true
		}
	}
	return false
}

// resolveSeverity picks whichever severity shape Rootly returned:
// a nested severity_attributes.slug if present, else the flat
// `severity` field. See raw.IncidentAttributes for the shape
// decision deferred to implementation.
func resolveSeverity(attrs raw.IncidentAttributes) string {
	if attrs.SeverityObj != nil && attrs.SeverityObj.Slug != "" {
		return attrs.SeverityObj.Slug
	}
	return resolve(attrs.SeveritySlug)
}

func resolve[T any](t *T) T {
	if t == nil {
		return *new(T)
	}
	return *t
}

func resolveInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
