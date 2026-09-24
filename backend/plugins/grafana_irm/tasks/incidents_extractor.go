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
	"strings"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/grafana_irm/models"
	"github.com/apache/devlake/plugins/grafana_irm/models/raw"
)

var _ plugin.SubTaskEntryPoint = ExtractIncidents

var ExtractIncidentsMeta = plugin.SubTaskMeta{
	Name:             "extractIncidents",
	EntryPoint:       ExtractIncidents,
	EnabledByDefault: true,
	Description:      "Extract Grafana IRM incidents",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables: []string{
		models.Incident{}.TableName(),
		models.IncidentLabel{}.TableName(),
		models.IncidentAssignment{}.TableName(),
	},
}

func ExtractIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GrafanaIrmTaskData)
	endpoint := strings.TrimSuffix(data.Connection.GetEndpoint(), "/")

	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			return extractIncident(row.Data, data.Options, endpoint)
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func extractIncident(rawData []byte, op *GrafanaIrmOptions, endpoint string) ([]interface{}, errors.Error) {
	rawIncident := &raw.Incident{}
	if err := errors.Convert(json.Unmarshal(rawData, rawIncident)); err != nil {
		return nil, err
	}

	createdDate, err := parseIncidentTime(rawIncident.CreatedTime)
	if err != nil {
		return nil, err
	}
	updatedDate, err := parseIncidentTime(rawIncident.ModifiedTime)
	if err != nil {
		return nil, err
	}
	resolvedDate, err := parseOptionalIncidentTime(rawIncident.ClosedTime)
	if err != nil {
		return nil, err
	}

	incident := &models.Incident{
		ConnectionId: op.ConnectionId,
		Id:           rawIncident.IncidentID,
		Title:        rawIncident.Title,
		Url:          resolveIncidentUrl(endpoint, rawIncident.OverviewURL),
		Status:       rawIncident.Status,
		Severity:     rawIncident.Severity,
		CreatedDate:  createdDate,
		UpdatedDate:  updatedDate,
		ResolvedDate: resolvedDate,
	}

	result := []interface{}{incident}

	for _, label := range rawIncident.Labels {
		result = append(result, &models.IncidentLabel{
			ConnectionId: op.ConnectionId,
			IncidentId:   rawIncident.IncidentID,
			Key:          label.Key,
			Label:        label.Label,
		})
	}

	// IncidentMembership.Assignments is a fixed-size array of role *slots*,
	// most of them empty placeholders (verified live, see
	// grafana_irm_plan.md §3.4/§9) — only keep entries with a real user.
	for _, assignment := range rawIncident.IncidentMembership.Assignments {
		if assignment.User.UserID == "" {
			continue
		}
		result = append(result, &models.IncidentAssignment{
			ConnectionId: op.ConnectionId,
			IncidentId:   rawIncident.IncidentID,
			UserId:       assignment.User.UserID,
			UserName:     assignment.User.Name,
			RoleName:     assignment.Role.Name,
		})
	}

	return result, nil
}

// resolveIncidentUrl prefixes the wire's relative overviewURL with the
// connection's endpoint (verified live: overviewURL is relative, e.g.
// "/a/grafana-irm-app/incidents/1/some-slug" — see grafana_irm_plan.md §3.4).
func resolveIncidentUrl(endpoint, overviewURL string) string {
	if overviewURL == "" {
		return ""
	}
	return endpoint + overviewURL
}

// parseIncidentTime parses a required RFC3339 timestamp field. Go's
// time.Parse accepts arbitrary-precision fractional seconds even though the
// RFC3339 layout constant doesn't declare one, which real responses use
// (e.g. "2026-09-19T11:07:18.515762775Z", verified live).
func parseIncidentTime(value string) (time.Time, errors.Error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.Convert(err)
	}
	return t.Truncate(time.Millisecond), nil
}

// parseOptionalIncidentTime handles fields that come back as a literal empty
// string when unset (verified live: ClosedTime on an open incident is `""`,
// not null or omitted — see grafana_irm_plan.md §3.4).
func parseOptionalIncidentTime(value string) (*time.Time, errors.Error) {
	if value == "" {
		return nil, nil
	}
	t, err := parseIncidentTime(value)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
