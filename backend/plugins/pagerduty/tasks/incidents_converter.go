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
	"fmt"
	"reflect"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/pagerduty/models"
)

var ConvertIncidentsMeta = plugin.SubTaskMeta{
	Name:             "convertIncidents",
	EntryPoint:       ConvertIncidents,
	EnabledByDefault: true,
	Description:      "Convert incidents into domain layer table issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type (
	// IncidentWithUser struct that represents the joined query result
	IncidentWithUser struct {
		common.NoPKModel
		models.Incident
		*models.User
		AssignedAt time.Time
	}
)

func ConvertIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*PagerDutyTaskData)
	cursor, err := db.Cursor(
		dal.Select("pi.*, pu.*, pa.assigned_at"),
		dal.From("_tool_pagerduty_incidents AS pi"),
		// incident numbers are only unique within a PagerDuty account, and
		// assignments and users are stored per connection, so keep both joins
		// inside the incident's connection
		dal.Join(`LEFT JOIN _tool_pagerduty_assignments AS pa ON pa.connection_id = pi.connection_id AND pa.incident_number = pi.number`),
		dal.Join(`LEFT JOIN _tool_pagerduty_users AS pu ON pu.connection_id = pa.connection_id AND pu.id = pa.user_id`),
		dal.Where("pi.connection_id = ? AND pi.service_id = ?", data.Options.ConnectionId, data.Options.ServiceId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()
	seenIncidents := map[int]*IncidentWithUser{}
	idGen := didgen.NewDomainIdGenerator(&models.Incident{})
	serviceIdGen := didgen.NewDomainIdGenerator(&models.Service{})
	converter, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		InputRowType: reflect.TypeOf(IncidentWithUser{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			combined := inputRow.(*IncidentWithUser)
			incident := combined.Incident
			if seen, ok := seenIncidents[incident.Number]; ok {
				if combined.AssignedAt.Before(seen.AssignedAt) {
					// skip this one (it's an older assignee)
					return nil, nil
				}
			}
			status := getStatus(&incident)
			leadTime, resolutionDate := getTimes(&incident)
			domainIssue := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: idGen.Generate(data.Options.ConnectionId, incident.Number),
				},
				Url:             incident.Url,
				IssueKey:        fmt.Sprintf("%d", incident.Number),
				Description:     incident.Summary,
				Type:            ticket.INCIDENT,
				Status:          status,
				OriginalStatus:  string(incident.Status),
				ResolutionDate:  resolutionDate,
				CreatedDate:     &incident.CreatedDate,
				UpdatedDate:     &incident.UpdatedDate,
				LeadTimeMinutes: leadTime,
				Priority:        incident.Priority,
				Urgency:         string(incident.Urgency),
			}
			var result []interface{}
			if combined.User != nil {
				domainIssue.AssigneeId = combined.User.Id
				domainIssue.AssigneeName = combined.User.Name
				issueAssignee := &ticket.IssueAssignee{
					IssueId:      domainIssue.Id,
					AssigneeId:   domainIssue.AssigneeId,
					AssigneeName: domainIssue.AssigneeName,
				}
				result = append(result, issueAssignee)
			}
			result = append(result, domainIssue)
			seenIncidents[incident.Number] = combined
			boardIssue := &ticket.BoardIssue{
				BoardId: serviceIdGen.Generate(data.Options.ConnectionId, data.Options.ServiceId),
				IssueId: domainIssue.Id,
			}
			result = append(result, boardIssue)
			return result, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

func getStatus(incident *models.Incident) string {
	if incident.Status == models.IncidentStatusTriggered {
		return ticket.TODO
	}
	if incident.Status == models.IncidentStatusAcknowledged {
		return ticket.IN_PROGRESS
	}
	if incident.Status == models.IncidentStatusResolved {
		return ticket.DONE
	}
	panic("unknown incident status encountered")
}

func getTimes(incident *models.Incident) (*uint, *time.Time) {
	var leadTime *uint
	var resolutionDate *time.Time
	if incident.Status == models.IncidentStatusResolved {
		resolutionDate = &incident.UpdatedDate
		temp := uint(resolutionDate.Sub(incident.CreatedDate).Minutes())
		leadTime = &temp
	}
	return leadTime, resolutionDate
}
