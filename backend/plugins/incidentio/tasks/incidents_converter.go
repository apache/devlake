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
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/incidentio/models"
)

var _ plugin.SubTaskEntryPoint = ConvertIncidents

var ConvertIncidentsMeta = plugin.SubTaskMeta{
	Name:             "convertIncidents",
	EntryPoint:       ConvertIncidents,
	EnabledByDefault: true,
	Description:      "Convert incident.io incidents into domain-layer ticket issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*IncidentioTaskData)
	logger := taskCtx.GetLogger()

	cursor, err := db.Cursor(
		dal.From(&models.Incident{}),
		dal.Where("connection_id = ? AND incident_type_id = ?", data.Options.ConnectionId, data.Options.IncidentTypeId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	idGen := didgen.NewDomainIdGenerator(&models.Incident{})
	incidentTypeIdGen := didgen.NewDomainIdGenerator(&models.IncidentType{})
	boardId := incidentTypeIdGen.Generate(data.Options.ConnectionId, data.Options.IncidentTypeId)

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.Incident{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			incident := inputRow.(*models.Incident)

			status, known := mapStatusCategory(incident.StatusCategory)
			if !known {
				logger.Warn(nil, "unknown incident.io status category: %s", incident.StatusCategory)
			}

			leadTime, resolutionDate := computeLeadTime(incident.DeclaredDate, incident.ResolvedDate)

			domainIssueId := idGen.Generate(data.Options.ConnectionId, incident.Id)

			domainIssue := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: domainIssueId,
				},
				Url:             incident.Url,
				IssueKey:        issueKeyFor(incident),
				Title:           incident.Name,
				Description:     incident.Summary,
				Type:            ticket.INCIDENT,
				Status:          status,
				OriginalStatus:  incident.StatusName,
				ResolutionDate:  resolutionDate,
				CreatedDate:     &incident.DeclaredDate,
				UpdatedDate:     &incident.UpdatedDate,
				LeadTimeMinutes: leadTime,
				Severity:        incident.SeverityName,
			}

			return []interface{}{
				domainIssue,
				&ticket.BoardIssue{
					BoardId: boardId,
					IssueId: domainIssueId,
				},
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

// Unknown categories fall through to IN_PROGRESS rather than
// panicking; incident.io statuses are operator-defined, but their
// categories form a small fixed enum, so anything new from upstream
// shouldn't crash a production pipeline.
//
// The API values are triage, declined, merged, canceled, live,
// learning, closed and paused. The app shows live as "Active" and
// learning as "Post-incident". declined, merged and canceled are
// terminal: the incident was declined in triage, merged into another
// incident, or canceled.
func mapStatusCategory(category string) (mapped string, known bool) {
	switch category {
	case "triage", "live", "learning", "paused":
		return ticket.IN_PROGRESS, true
	case "closed", "declined", "merged", "canceled":
		return ticket.DONE, true
	default:
		return ticket.IN_PROGRESS, false
	}
}

func computeLeadTime(declared time.Time, resolved *time.Time) (*uint, *time.Time) {
	if resolved == nil {
		return nil, nil
	}
	// Retrospective incidents are declared after the fact, so resolved
	// legitimately precedes declared. The resolution date is still real;
	// only the declared→resolved duration is meaningless (and a naive
	// uint() cast on a negative duration would wrap to huge garbage and
	// silently corrupt MTTR), so keep the date and drop the lead time.
	if resolved.Before(declared) {
		resolutionDate := *resolved
		return nil, &resolutionDate
	}
	minutes := uint(resolved.Sub(declared).Minutes())
	resolutionDate := *resolved
	return &minutes, &resolutionDate
}

func issueKeyFor(incident *models.Incident) string {
	if incident.Reference != "" {
		return incident.Reference
	}
	return incident.Id
}
