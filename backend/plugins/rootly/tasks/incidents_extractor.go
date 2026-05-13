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
//     fields populated from the nested attributes.{user,started_by,
//     mitigated_by,resolved_by,closed_by} JSON:API-envelope blocks
//   - zero-to-N *models.User rows, one per distinct user id seen across
//     those role fields (deduplicated within a single incident — if the
//     same user is both creator and resolver, only one User row is
//     emitted)
//
// The role fields are nested JSON:API response envelopes on the
// incident's attributes — inner record at `<field>.data.attributes.*`.
// We pull users straight from those without needing a separate users
// collector or a JSON:API `included` sidecar parse.
func extractRootlyIncident(rawData []byte, op *RootlyOptions) ([]interface{}, errors.Error) {
	rawIncident := &raw.Incident{}
	if err := errors.Convert(json.Unmarshal(rawData, rawIncident)); err != nil {
		return nil, err
	}

	// Safety-net scope filter. Rootly exposes service membership on the
	// JSON:API relationships block (id+type pointers only, no embedded
	// attributes unless we pass `?include=services`). The collector
	// relies on `filter[service_ids]=<op.ServiceId>` for scoping; this
	// is defense in depth for multi-service incidents that would
	// otherwise leak into a wrong scope. When the relationship is
	// empty we accept the incident — API-side filtering is the only
	// signal we have.
	if services := rawIncident.Relationships.Services.Data; len(services) > 0 && !containsServiceId(services, op.ServiceId) {
		return nil, nil
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
		results = append(results, &models.User{
			ConnectionId: op.ConnectionId,
			Id:           u.Data.Id,
			Email:        u.Data.Attributes.Email,
			Name:         pickUserName(u.Data.Attributes),
		})
	}
	addUser(rawIncident.Attributes.User, func(id string) { incident.CreatorUserId = id })
	addUser(rawIncident.Attributes.StartedBy, func(id string) { incident.StartedByUserId = id })
	addUser(rawIncident.Attributes.MitigatedBy, func(id string) { incident.MitigatedByUserId = id })
	addUser(rawIncident.Attributes.ResolvedBy, func(id string) { incident.ResolvedByUserId = id })
	addUser(rawIncident.Attributes.ClosedBy, func(id string) { incident.ClosedByUserId = id })

	return results, nil
}

// pickUserName chooses the best display name: FullName when set,
// otherwise Name, otherwise Email, otherwise empty. Email is a
// last-resort fallback so the User row is never nameless.
func pickUserName(u raw.UserAttributes) string {
	if u.FullName != "" {
		return u.FullName
	}
	if u.Name != "" {
		return u.Name
	}
	return u.Email
}

// containsServiceId checks whether the given service id appears in
// the incident's relationships.services.data array. Each entry is
// just a JSON:API pointer (id + type), not a full service record.
func containsServiceId(services []struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}, serviceId string) bool {
	for _, s := range services {
		if s.Id == serviceId {
			return true
		}
	}
	return false
}

// resolveSeverity extracts the slug from a Rootly severity envelope.
// Rootly returns severity as a JSON:API envelope with inner attributes
// `slug` (org-defined, e.g. "sev2") and `severity` (domain-normalized:
// critical/high/medium/low). We preserve the org's own slug in the tool
// layer — the converter applies the domain-normalized mapping at
// conversion time.
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

func resolveInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
