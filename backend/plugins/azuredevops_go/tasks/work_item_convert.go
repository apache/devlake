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
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
	"gorm.io/datatypes"
)

func init() {
	RegisterSubtaskMeta(&ExtractApiWorkItemsMeta)
	RegisterSubtaskMeta(&ConvertWorkItemsMeta)
}

var workItemIDFromURL = regexp.MustCompile(`(?i)/workitems/(\d+)`)

var mappedWorkItemFields = map[string]struct{}{
	"System.Title": {}, "System.WorkItemType": {}, "System.State": {}, "System.Reason": {},
	"System.Description": {}, "System.CreatedDate": {}, "System.ChangedDate": {},
	"System.CreatedBy": {}, "System.AssignedTo": {}, "System.Parent": {},
	"System.IterationPath": {}, "System.AreaPath": {}, "System.Tags": {}, "System.TeamProject": {},
	"Microsoft.VSTS.Common.Priority": {}, "Microsoft.VSTS.Common.Severity": {}, "Microsoft.VSTS.Common.ClosedDate": {},
	"Microsoft.VSTS.Scheduling.TargetDate": {}, "Microsoft.VSTS.Scheduling.DueDate": {},
	"Microsoft.VSTS.Scheduling.OriginalEstimate": {}, "Microsoft.VSTS.Scheduling.CompletedWork": {},
	"Microsoft.VSTS.Scheduling.RemainingWork": {}, "Microsoft.VSTS.Scheduling.StoryPoints": {},
	"Microsoft.VSTS.Scheduling.Effort": {}, "Microsoft.VSTS.Scheduling.Size": {},
}

var ExtractApiWorkItemsMeta = plugin.SubTaskMeta{
	Name:             "extractApiWorkItems",
	EntryPoint:       ExtractApiWorkItems,
	EnabledByDefault: true,
	Description:      "Extract raw Azure Boards work items into the tool layer.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{RawWorkItemTable},
	ProductTables:    []string{models.AzuredevopsWorkItem{}.TableName()},
}

var ConvertWorkItemsMeta = plugin.SubTaskMeta{
	Name:             "convertWorkItems",
	EntryPoint:       ConvertWorkItems,
	EnabledByDefault: true,
	Description:      "Convert Azure Boards work items into domain issues, labels, and sprint links.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{
		models.AzuredevopsWorkItem{}.TableName(),
		models.AzuredevopsIteration{}.TableName(),
	},
	ProductTables: []string{
		ticket.Issue{}.TableName(),
		ticket.BoardIssue{}.TableName(),
		ticket.SprintIssue{}.TableName(),
		ticket.IssueAssignee{}.TableName(),
		ticket.IssueLabel{}.TableName(),
		ticket.IssueRelationship{}.TableName(),
	},
}

type adoWorkItem struct {
	ID        int                      `json:"id"`
	URL       string                   `json:"url"`
	Fields    map[string]interface{}   `json:"fields"`
	Relations []map[string]interface{} `json:"relations"`
}

func ExtractApiWorkItems(taskCtx plugin.SubTaskContext) errors.Error {
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawWorkItemTable)
	displayNames := loadWorkItemFieldDisplayNames(taskCtx, data)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			raw := &adoWorkItem{}
			if err := errors.Convert(json.Unmarshal(row.Data, raw)); err != nil {
				return nil, err
			}
			fields := raw.Fields
			if fields == nil {
				fields = map[string]interface{}{}
			}
			assigneeID, assigneeName := identityFromRef(fields["System.AssignedTo"])
			creatorID, creatorName := identityFromRef(fields["System.CreatedBy"])
			var parentID *int
			if fields["System.Parent"] != nil {
				parent := asInt(fields["System.Parent"])
				if parent != 0 {
					parentID = &parent
				}
			}
			relations, _ := json.Marshal(raw.Relations)
			relationsJSON := ""
			if len(raw.Relations) > 0 {
				relationsJSON = string(relations)
			}
			url := raw.URL
			if url == "" {
				url = "https://dev.azure.com/" + data.Options.OrganizationId + "/_workitems/edit/" + asString(raw.ID)
			}
			return []interface{}{&models.AzuredevopsWorkItem{
				ConnectionId:            data.Options.ConnectionId,
				AzuredevopsId:           raw.ID,
				OrgId:                   data.Options.OrganizationId,
				ProjectId:               firstNonEmpty(asString(fields["System.TeamProject"]), data.Options.ProjectId),
				Title:                   asString(fields["System.Title"]),
				WorkItemType:            asString(fields["System.WorkItemType"]),
				State:                   asString(fields["System.State"]),
				Reason:                  asString(fields["System.Reason"]),
				Priority:                asString(fields["Microsoft.VSTS.Common.Priority"]),
				Severity:                asString(fields["Microsoft.VSTS.Common.Severity"]),
				Description:             asString(fields["System.Description"]),
				Url:                     url,
				CreatedDate:             parseAdoTime(fields["System.CreatedDate"]),
				UpdatedDate:             parseAdoTime(fields["System.ChangedDate"]),
				ClosedDate:              parseAdoTime(fields["Microsoft.VSTS.Common.ClosedDate"]),
				DueDate:                 firstTime(fields["Microsoft.VSTS.Scheduling.TargetDate"], fields["Microsoft.VSTS.Scheduling.DueDate"]),
				OriginalEstimateMinutes: hoursToMinutes(fields["Microsoft.VSTS.Scheduling.OriginalEstimate"]),
				TimeSpentMinutes:        hoursToMinutes(fields["Microsoft.VSTS.Scheduling.CompletedWork"]),
				TimeRemainingMinutes:    hoursToMinutes(fields["Microsoft.VSTS.Scheduling.RemainingWork"]),
				CreatorId:               creatorID,
				CreatorName:             creatorName,
				AssigneeId:              assigneeID,
				AssigneeName:            assigneeName,
				ParentId:                parentID,
				StoryPoints:             firstFloat(fields, "Microsoft.VSTS.Scheduling.StoryPoints", "Microsoft.VSTS.Scheduling.Effort", "Microsoft.VSTS.Scheduling.Size"),
				IterationPath:           asString(fields["System.IterationPath"]),
				AreaPath:                asString(fields["System.AreaPath"]),
				Tags:                    asString(fields["System.Tags"]),
				RelationsJson:           relationsJSON,
				CustomFieldsJson:        customFieldsJSON(fields, displayNames),
			}}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func ConvertWorkItems(taskCtx plugin.SubTaskContext) errors.Error {
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, models.AzuredevopsWorkItem{}.TableName())
	mapper := newWorkItemMapper(taskCtx, data)
	var typeMappings datatypes.JSONMap
	var statusMappings datatypes.JSONMap
	if data.Options.ScopeConfig != nil {
		typeMappings = data.Options.ScopeConfig.TypeMappings
		statusMappings = data.Options.ScopeConfig.StatusMappings
	}
	workItemIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsWorkItem{})
	iterationIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsIteration{})
	boardID := projectBoardID(data)
	db := taskCtx.GetDal()
	cursor, err := db.Cursor(
		dal.From(&models.AzuredevopsWorkItem{}),
		dal.Where("connection_id = ? AND project_id = ?", data.Options.ConnectionId, data.Options.ProjectId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := api.NewDataConverter(api.DataConverterArgs{
		InputRowType:       reflect.TypeOf(models.AzuredevopsWorkItem{}),
		Input:              cursor,
		RawDataSubTaskArgs: *rawArgs,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			workItem := inputRow.(*models.AzuredevopsWorkItem)
			if !shouldConvertWorkItem(workItem.WorkItemType) {
				return nil, nil
			}
			mapper.loadStateCategory(data, workItem.WorkItemType)
			hasParent := workItem.ParentId != nil
			stdType := resolveWorkItemType(mapper.processTemplate, workItem.WorkItemType, hasParent, typeMappings)
			stdStatus := resolveWorkItemStatus(workItem.WorkItemType, workItem.State, statusMappings, mapper.categories)
			var resolution *time.Time
			if stdStatus == ticket.DONE {
				resolution = workItem.ClosedDate
				if resolution == nil {
					resolution = workItem.UpdatedDate
				}
			}
			var leadTime *uint
			if workItem.CreatedDate != nil && resolution != nil {
				seconds := int64(resolution.Sub(*workItem.CreatedDate).Seconds())
				if seconds >= 0 {
					minutes := uint(seconds / 60)
					leadTime = &minutes
				}
			}
			issueID := workItemIdGen.Generate(data.Options.ConnectionId, workItem.AzuredevopsId)
			var parentIssueID string
			if hasParent {
				parentIssueID = workItemIdGen.Generate(data.Options.ConnectionId, *workItem.ParentId)
			}
			issue := &ticket.Issue{
				DomainEntity:            domainlayer.NewDomainEntity(issueID),
				Url:                     workItem.Url,
				IssueKey:                asString(workItem.AzuredevopsId),
				Title:                   workItem.Title,
				Description:             workItem.Description,
				Type:                    stdType,
				OriginalType:            workItem.WorkItemType,
				Status:                  stdStatus,
				OriginalStatus:          workItem.State,
				CreatedDate:             workItem.CreatedDate,
				UpdatedDate:             workItem.UpdatedDate,
				ResolutionDate:          resolution,
				LeadTimeMinutes:         leadTime,
				OriginalEstimateMinutes: workItem.OriginalEstimateMinutes,
				TimeSpentMinutes:        workItem.TimeSpentMinutes,
				TimeRemainingMinutes:    workItem.TimeRemainingMinutes,
				CreatorId:               workItem.CreatorId,
				CreatorName:             workItem.CreatorName,
				AssigneeId:              workItem.AssigneeId,
				AssigneeName:            workItem.AssigneeName,
				ParentIssueId:           parentIssueID,
				Priority:                workItem.Priority,
				Severity:                workItem.Severity,
				Component:               areaPathToComponent(workItem.AreaPath),
				OriginalProject:         workItem.ProjectId,
				StoryPoint:              workItem.StoryPoints,
				IsSubtask:               hasParent,
				DueDate:                 workItem.DueDate,
			}
			results := []interface{}{
				issue,
				&ticket.BoardIssue{BoardId: boardID, IssueId: issueID},
			}
			if pathKey := iterationPathKey(workItem.IterationPath, workItem.ProjectId); pathKey != "" {
				results = append(results, &ticket.SprintIssue{
					SprintId: iterationIdGen.Generate(data.Options.ConnectionId, pathKey),
					IssueId:  issueID,
				})
			}
			if workItem.AssigneeId != "" {
				results = append(results, &ticket.IssueAssignee{
					IssueId:      issueID,
					AssigneeId:   workItem.AssigneeId,
					AssigneeName: workItem.AssigneeName,
				})
			}
			for _, tag := range splitTags(workItem.Tags) {
				results = append(results, &ticket.IssueLabel{IssueId: issueID, LabelName: tag})
			}
			results = append(results, workItemRelationships(workItem, issueID, workItemIdGen, data.Options.ConnectionId)...)
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstTime(values ...interface{}) *time.Time {
	for _, value := range values {
		if parsed := parseAdoTime(value); parsed != nil {
			return parsed
		}
	}
	return nil
}

func firstFloat(fields map[string]interface{}, names ...string) *float64 {
	for _, name := range names {
		if parsed := asFloatPtr(fields[name]); parsed != nil {
			return parsed
		}
	}
	return nil
}

func loadWorkItemFieldDisplayNames(taskCtx plugin.SubTaskContext, data *AzuredevopsTaskData) map[string]string {
	names := map[string]string{}
	res, err := data.ApiClient.Get(fmt.Sprintf("%s/_apis/wit/fields?api-version=7.1", adoScopePrefix(data)), nil, nil)
	if err != nil {
		taskCtx.GetLogger().Warn(err, "failed to load Azure DevOps work item field names")
		return names
	}
	var body struct {
		Value []struct {
			ReferenceName string `json:"referenceName"`
			Name          string `json:"name"`
		} `json:"value"`
	}
	if err = readJSON(res, &body); err != nil {
		taskCtx.GetLogger().Warn(err, "failed to load Azure DevOps work item field names")
		return names
	}
	for _, item := range body.Value {
		if item.ReferenceName != "" && item.Name != "" && item.Name != item.ReferenceName {
			names[item.ReferenceName] = item.Name
		}
	}
	return names
}

func customFieldsJSON(fields map[string]interface{}, displayNames map[string]string) string {
	custom := map[string]interface{}{}
	for ref, value := range fields {
		if value == nil || strings.HasPrefix(ref, "System.") {
			continue
		}
		if _, mapped := mappedWorkItemFields[ref]; mapped {
			continue
		}
		custom[ref] = value
		if name := displayNames[ref]; name != "" {
			if _, exists := custom[name]; !exists {
				custom[name] = value
			}
		}
	}
	if len(custom) == 0 {
		return ""
	}
	encoded, err := json.Marshal(custom)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func splitTags(tags string) []string {
	if tags == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, tag := range regexp.MustCompile(`\s*;\s*`).Split(tags, -1) {
		if tag != "" {
			parts = append(parts, tag)
		}
	}
	return parts
}

func workItemRelationships(workItem *models.AzuredevopsWorkItem, issueID string, idGen *didgen.DomainIdGenerator, connectionID uint64) []interface{} {
	var relations []map[string]interface{}
	if workItem.RelationsJson != "" {
		_ = json.Unmarshal([]byte(workItem.RelationsJson), &relations)
	}
	linked := map[int]struct{}{}
	results := make([]interface{}, 0)
	for _, relation := range relations {
		match := workItemIDFromURL.FindStringSubmatch(asString(relation["url"]))
		relType := asString(relation["rel"])
		if len(match) < 2 || relType == "" {
			continue
		}
		targetID := asInt(match[1])
		if targetID == 0 || targetID == workItem.AzuredevopsId {
			continue
		}
		linked[targetID] = struct{}{}
		results = append(results, &ticket.IssueRelationship{
			SourceIssueId: issueID,
			TargetIssueId: idGen.Generate(connectionID, targetID),
			OriginalType:  relType,
		})
	}
	if workItem.ParentId != nil {
		if _, ok := linked[*workItem.ParentId]; !ok {
			results = append(results, &ticket.IssueRelationship{
				SourceIssueId: issueID,
				TargetIssueId: idGen.Generate(connectionID, *workItem.ParentId),
				OriginalType:  "System.LinkTypes.Hierarchy-Reverse",
			})
		}
	}
	return results
}
