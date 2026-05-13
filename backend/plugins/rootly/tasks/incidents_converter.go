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
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/rootly/models"
)

var _ plugin.SubTaskEntryPoint = ConvertIncidents

var ConvertIncidentsMeta = plugin.SubTaskMeta{
	Name:             "convertIncidents",
	EntryPoint:       ConvertIncidents,
	EnabledByDefault: true,
	Description:      "Convert Rootly incidents into domain-layer ticket issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*RootlyTaskData)
	logger := taskCtx.GetLogger()

	var userRows []models.User
	if err := db.All(
		&userRows,
		dal.Where("connection_id = ?", data.Options.ConnectionId),
	); err != nil {
		return err
	}
	userNames := make(map[string]string, len(userRows))
	for _, u := range userRows {
		userNames[u.Id] = u.Name
	}

	cursor, err := db.Cursor(
		dal.From(&models.Incident{}),
		dal.Where("connection_id = ? AND service_id = ?", data.Options.ConnectionId, data.Options.ServiceId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	idGen := didgen.NewDomainIdGenerator(&models.Incident{})
	serviceIdGen := didgen.NewDomainIdGenerator(&models.Service{})
	userIdGen := didgen.NewDomainIdGenerator(&models.User{})
	boardId := serviceIdGen.Generate(data.Options.ConnectionId, data.Options.ServiceId)

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

			status := mapStatus(incident.Status)
			if status == ticket.IN_PROGRESS && !isKnownStatus(incident.Status) {
				logger.Warn(nil, "unknown rootly incident status: %s", incident.Status)
			}

			leadTime, resolutionDate := computeLeadTime(incident.StartedDate, incident.ResolvedDate)

			domainIssueId := idGen.Generate(data.Options.ConnectionId, incident.Id)

			var creatorDomainId, creatorName string
			if incident.CreatorUserId != "" {
				creatorDomainId = userIdGen.Generate(data.Options.ConnectionId, incident.CreatorUserId)
				creatorName = userNames[incident.CreatorUserId]
			}

			domainIssue := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: domainIssueId,
				},
				Url:             incident.Url,
				IssueKey:        issueKeyFor(incident),
				Title:           incident.Title,
				Description:     incident.Summary,
				Type:            ticket.INCIDENT,
				Status:          status,
				OriginalStatus:  incident.Status,
				ResolutionDate:  resolutionDate,
				CreatedDate:     &incident.StartedDate,
				UpdatedDate:     &incident.UpdatedDate,
				LeadTimeMinutes: leadTime,
				Priority:        mapSeverityToPriority(incident.Severity),
				Severity:        incident.Severity,
				CreatorId:       creatorDomainId,
				CreatorName:     creatorName,
				AssigneeId:      creatorDomainId,
				AssigneeName:    creatorName,
			}

			results := []interface{}{domainIssue}

			seenAssignees := map[string]bool{}
			roleUserIds := []string{
				incident.CreatorUserId,
				incident.StartedByUserId,
				incident.MitigatedByUserId,
				incident.ResolvedByUserId,
				incident.ClosedByUserId,
			}
			for _, toolUserId := range roleUserIds {
				if toolUserId == "" || seenAssignees[toolUserId] {
					continue
				}
				seenAssignees[toolUserId] = true
				results = append(results, &ticket.IssueAssignee{
					IssueId:      domainIssueId,
					AssigneeId:   userIdGen.Generate(data.Options.ConnectionId, toolUserId),
					AssigneeName: userNames[toolUserId],
				})
			}

			results = append(results, &ticket.BoardIssue{
				BoardId: boardId,
				IssueId: domainIssueId,
			})

			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

// Unknown statuses fall through to IN_PROGRESS rather than panicking
// (PagerDuty panics). Rootly's status enum is more volatile, so a new
// value from upstream shouldn't crash a production pipeline.
func mapStatus(status string) string {
	switch status {
	case "triage", "started":
		return ticket.TODO
	case "mitigated":
		return ticket.IN_PROGRESS
	case "resolved", "closed", "cancelled":
		return ticket.DONE
	default:
		return ticket.IN_PROGRESS
	}
}

func isKnownStatus(status string) bool {
	switch status {
	case "triage", "started", "mitigated", "resolved", "closed", "cancelled":
		return true
	default:
		return false
	}
}

func mapSeverityToPriority(severity string) string {
	switch strings.ToLower(severity) {
	case "sev0":
		return "CRITICAL"
	case "sev1":
		return "HIGH"
	case "sev2":
		return "MEDIUM"
	case "sev3", "sev4":
		return "LOW"
	default:
		return severity
	}
}

func computeLeadTime(started time.Time, resolved *time.Time) (*uint, *time.Time) {
	if resolved == nil {
		return nil, nil
	}
	// Clock skew / backfill can place resolved before started. A naive
	// uint() cast on a negative duration wraps to huge garbage and
	// silently corrupts MTTR; treat as unresolved instead.
	if resolved.Before(started) {
		return nil, nil
	}
	minutes := uint(resolved.Sub(started).Minutes())
	resolutionDate := *resolved
	return &minutes, &resolutionDate
}

func issueKeyFor(incident *models.Incident) string {
	if incident.Number > 0 {
		return fmt.Sprintf("%d", incident.Number)
	}
	return incident.Id
}
