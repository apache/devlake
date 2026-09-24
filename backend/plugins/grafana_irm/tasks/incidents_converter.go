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
	"reflect"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/grafana_irm/models"
)

var _ plugin.SubTaskEntryPoint = ConvertIncidents

var ConvertIncidentsMeta = plugin.SubTaskMeta{
	Name:             "convertIncidents",
	EntryPoint:       ConvertIncidents,
	EnabledByDefault: true,
	Description:      "Convert Grafana IRM incidents into domain-layer ticket issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

// ConvertIncidents maps tool-layer Incident rows to domain ticket.Issue rows
// (Type: INCIDENT), plus their labels and real assignments as IssueLabel/
// IssueAssignee rows. A connection has exactly one scope covering its whole
// incident stream (§4.2 — Grafana IRM has no listable service/team resource,
// and the originating feature request never asked for per-team filtering),
// so every incident collected for this connection converts unconditionally.
func ConvertIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GrafanaIrmTaskData)
	logger := taskCtx.GetLogger()

	cursor, err := db.Cursor(
		dal.From(&models.Incident{}),
		dal.Where("connection_id = ?", data.Options.ConnectionId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	idGen := didgen.NewDomainIdGenerator(&models.Incident{})
	boardId := didgen.NewDomainIdGenerator(&models.GrafanaIrmScope{}).
		Generate(data.Options.ConnectionId, data.Options.ScopeId)

	scope := &models.GrafanaIrmScope{}
	scopeName := "All Incidents"
	if err := db.First(scope, dal.Where("connection_id = ? AND id = ?", data.Options.ConnectionId, data.Options.ScopeId)); err == nil && scope.Name != "" {
		scopeName = scope.Name
	}
	domainBoard := &ticket.Board{
		DomainEntity: domainlayer.DomainEntity{Id: boardId},
		Name:         scopeName,
	}
	if err := db.CreateOrUpdate(domainBoard); err != nil {
		return err
	}

	converter, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.Incident{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			incident := inputRow.(*models.Incident)

			var labels []models.IncidentLabel
			if err := db.All(&labels,
				dal.From(&models.IncidentLabel{}),
				dal.Where("connection_id = ? AND incident_id = ?", data.Options.ConnectionId, incident.Id),
			); err != nil {
				return nil, err
			}

			status, known := mapIncidentStatus(incident.Status)
			if !known {
				logger.Warn(nil, "unknown grafana irm incident status: %s", incident.Status)
			}

			domainIssueId := idGen.Generate(data.Options.ConnectionId, incident.Id)
			createdDate := incident.CreatedDate
			updatedDate := incident.UpdatedDate

			domainIssue := &ticket.Issue{
				DomainEntity:    domainlayer.DomainEntity{Id: domainIssueId},
				Url:             incident.Url,
				IssueKey:        incident.Id,
				Title:           incident.Title,
				Type:            ticket.INCIDENT,
				Status:          status,
				OriginalStatus:  incident.Status,
				Severity:        incident.Severity,
				ResolutionDate:  incident.ResolvedDate,
				CreatedDate:     &createdDate,
				UpdatedDate:     &updatedDate,
				LeadTimeMinutes: computeLeadTimeMinutes(incident.CreatedDate, incident.ResolvedDate),
			}

			result := []interface{}{
				domainIssue,
				&ticket.BoardIssue{BoardId: boardId, IssueId: domainIssueId},
			}

			for _, label := range labels {
				result = append(result, &ticket.IssueLabel{
					IssueId:   domainIssueId,
					LabelName: label.Key + ":" + label.Label,
				})
			}

			var assignments []models.IncidentAssignment
			if err := db.All(&assignments,
				dal.From(&models.IncidentAssignment{}),
				dal.Where("connection_id = ? AND incident_id = ?", data.Options.ConnectionId, incident.Id),
			); err != nil {
				return nil, err
			}
			for _, assignment := range assignments {
				result = append(result, &ticket.IssueAssignee{
					IssueId:      domainIssueId,
					AssigneeId:   assignment.UserId,
					AssigneeName: assignment.UserName,
				})
			}

			return result, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

// mapIncidentStatus mirrors incidentio's soft-fallback approach: an unknown
// status logs a warning instead of failing the pipeline, since Grafana IRM
// statuses could in principle be extended beyond the two values ("active",
// "resolved") verified live so far (see grafana_irm_plan.md §3.4/§9).
func mapIncidentStatus(status string) (mapped string, known bool) {
	switch status {
	case "active":
		return ticket.IN_PROGRESS, true
	case "resolved":
		return ticket.DONE, true
	default:
		return ticket.IN_PROGRESS, false
	}
}

// computeLeadTimeMinutes guards against a resolved timestamp preceding the
// created timestamp the same way incidentio's converter does: keep the
// resolution date but drop the (meaningless) lead time rather than let a
// negative duration wrap to a huge garbage uint.
func computeLeadTimeMinutes(created time.Time, resolved *time.Time) *uint {
	if resolved == nil || resolved.Before(created) {
		return nil
	}
	minutes := uint(resolved.Sub(created).Minutes())
	return &minutes
}
