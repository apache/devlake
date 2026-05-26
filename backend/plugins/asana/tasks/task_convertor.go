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
	"regexp"
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

var _ plugin.SubTaskEntryPoint = ConvertTask

var ConvertTaskMeta = plugin.SubTaskMeta{
	Name:             "ConvertTask",
	EntryPoint:       ConvertTask,
	EnabledByDefault: true,
	Description:      "Convert tool layer Asana tasks into domain layer issues and board_issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertTask(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, rawTaskTable)
	db := taskCtx.GetDal()
	connectionId := data.Options.ConnectionId
	projectId := data.Options.ProjectId

	// Get scope config for transformation rules
	scopeConfig := getScopeConfig(taskCtx)

	// Get tags for tasks
	taskTags := getTaskTags(db, connectionId)

	clauses := []dal.Clause{
		dal.From(&models.AsanaTask{}),
		dal.Where("connection_id = ? AND project_gid = ?", connectionId, projectId),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	taskIdGen := didgen.NewDomainIdGenerator(&models.AsanaTask{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.AsanaProject{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.AsanaUser{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(models.AsanaTask{}),
		Input:              cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			toolTask := inputRow.(*models.AsanaTask)

			// Get tags for this task
			tags := taskTags[toolTask.Gid]

			// Map type and status using scope config and tags
			stdType, stdStatus := getStdTypeAndStatus(toolTask, scopeConfig, tags)

			domainIssue := &ticket.Issue{
				DomainEntity:    domainlayer.DomainEntity{Id: taskIdGen.Generate(toolTask.ConnectionId, toolTask.Gid)},
				IssueKey:        toolTask.Gid,
				Title:           toolTask.Name,
				Description:     toolTask.Notes,
				Url:             toolTask.PermalinkUrl,
				Type:            stdType,
				OriginalType:    toolTask.ResourceSubtype,
				Status:          stdStatus,
				OriginalStatus:  getOriginalStatus(toolTask),
				StoryPoint:      toolTask.StoryPoint,
				CreatedDate:     &toolTask.CreatedAt,
				UpdatedDate:     toolTask.ModifiedAt,
				ResolutionDate:  toolTask.CompletedAt,
				DueDate:         toolTask.DueOn,
				CreatorName:     toolTask.CreatorName,
				AssigneeName:    toolTask.AssigneeName,
				LeadTimeMinutes: toolTask.LeadTimeMinutes,
			}

			// Set creator and assignee IDs
			if toolTask.CreatorGid != "" {
				domainIssue.CreatorId = accountIdGen.Generate(connectionId, toolTask.CreatorGid)
			}
			if toolTask.AssigneeGid != "" {
				domainIssue.AssigneeId = accountIdGen.Generate(connectionId, toolTask.AssigneeGid)
			}

			// Set parent issue ID if this is a subtask
			if toolTask.ParentGid != "" {
				domainIssue.ParentIssueId = taskIdGen.Generate(connectionId, toolTask.ParentGid)
				// If no type determined and has parent, it's a subtask
				if stdType == "" || stdType == ticket.TASK {
					domainIssue.Type = ticket.SUBTASK
				}
			}

			// Set subtask flag
			domainIssue.IsSubtask = toolTask.ParentGid != ""

			var result []interface{}
			result = append(result, domainIssue)

			// Create board issue relationship
			boardId := boardIdGen.Generate(connectionId, toolTask.ProjectGid)
			boardIssue := &ticket.BoardIssue{
				BoardId: boardId,
				IssueId: domainIssue.Id,
			}
			result = append(result, boardIssue)

			// Create issue assignee if assignee exists
			if toolTask.AssigneeGid != "" {
				issueAssignee := &ticket.IssueAssignee{
					IssueId:      domainIssue.Id,
					AssigneeId:   domainIssue.AssigneeId,
					AssigneeName: toolTask.AssigneeName,
				}
				result = append(result, issueAssignee)
			}

			return result, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

// getScopeConfig retrieves the scope config for transformation rules
func getScopeConfig(taskCtx plugin.SubTaskContext) *models.AsanaScopeConfig {
	logger := taskCtx.GetLogger()
	if taskCtx.GetData() == nil {
		logger.Info("getScopeConfig: taskCtx.GetData() is nil")
		return nil
	}
	data := taskCtx.GetData().(*AsanaTaskData)
	db := taskCtx.GetDal()

	// First try to get by ScopeConfigId from options
	if data.Options.ScopeConfigId != 0 {
		var scopeConfig models.AsanaScopeConfig
		err := db.First(&scopeConfig, dal.Where("id = ?", data.Options.ScopeConfigId))
		if err == nil {
			logger.Info("getScopeConfig: Found scope config by ID %d, IssueTypeRequirement=%s, IssueTypeBug=%s, IssueTypeIncident=%s",
				data.Options.ScopeConfigId, scopeConfig.IssueTypeRequirement, scopeConfig.IssueTypeBug, scopeConfig.IssueTypeIncident)
			return &scopeConfig
		}
		logger.Info("getScopeConfig: Failed to get scope config by ID %d: %v", data.Options.ScopeConfigId, err)
	} else {
		logger.Info("getScopeConfig: ScopeConfigId is 0, trying to get from project")
	}

	// Try to get scope config from project's scope_config_id
	var project models.AsanaProject
	err := db.First(&project, dal.Where("connection_id = ? AND gid = ?", data.Options.ConnectionId, data.Options.ProjectId))
	if err != nil {
		logger.Info("getScopeConfig: Failed to get project: %v", err)
		return nil
	}

	if project.ScopeConfigId != 0 {
		var scopeConfig models.AsanaScopeConfig
		err := db.First(&scopeConfig, dal.Where("id = ?", project.ScopeConfigId))
		if err == nil {
			logger.Info("getScopeConfig: Found scope config from project, IssueTypeRequirement=%s, IssueTypeBug=%s, IssueTypeIncident=%s",
				scopeConfig.IssueTypeRequirement, scopeConfig.IssueTypeBug, scopeConfig.IssueTypeIncident)
			return &scopeConfig
		}
		logger.Info("getScopeConfig: Failed to get scope config from project: %v", err)
	} else {
		logger.Info("getScopeConfig: Project has no scope_config_id")
	}

	return nil
}

// getTaskTags retrieves all tags for tasks and returns a map of taskGid -> []tagName
func getTaskTags(db dal.Dal, connectionId uint64) map[string][]string {
	result := make(map[string][]string)

	var taskTags []models.AsanaTaskTag
	err := db.All(&taskTags, dal.Where("connection_id = ?", connectionId))
	if err != nil {
		return result
	}

	// Get all tag names
	tagNames := make(map[string]string)
	var tags []models.AsanaTag
	err = db.All(&tags, dal.Where("connection_id = ?", connectionId))
	if err == nil {
		for _, tag := range tags {
			tagNames[tag.Gid] = tag.Name
		}
	}

	// Build taskGid -> []tagName map
	for _, tt := range taskTags {
		if tagName, ok := tagNames[tt.TagGid]; ok {
			result[tt.TaskGid] = append(result[tt.TaskGid], tagName)
		}
	}

	return result
}

// getStdTypeAndStatus maps Asana task to standard type and status using regex patterns (like GitHub)
func getStdTypeAndStatus(task *models.AsanaTask, scopeConfig *models.AsanaScopeConfig, tags []string) (string, string) {
	stdType := ticket.TASK
	stdStatus := ticket.TODO

	// Default status based on completion
	if task.Completed {
		stdStatus = ticket.DONE
	}

	// If no scope config, return defaults
	if scopeConfig == nil {
		return getDefaultType(task), stdStatus
	}

	// Combine all tags into a single string for matching
	tagString := strings.ToLower(strings.Join(tags, " "))

	// Match issue type using regex patterns (like GitHub)
	if scopeConfig.IssueTypeRequirement != "" && matchPattern(tagString, scopeConfig.IssueTypeRequirement) {
		stdType = ticket.REQUIREMENT
	}
	if scopeConfig.IssueTypeBug != "" && matchPattern(tagString, scopeConfig.IssueTypeBug) {
		stdType = ticket.BUG
	}
	if scopeConfig.IssueTypeIncident != "" && matchPattern(tagString, scopeConfig.IssueTypeIncident) {
		stdType = ticket.INCIDENT
	}

	// If no type matched and task is a subtask, mark it as subtask
	if stdType == ticket.TASK && task.ParentGid != "" {
		stdType = ticket.SUBTASK
	}

	return stdType, stdStatus
}

// getDefaultType returns the default type based on task properties
func getDefaultType(task *models.AsanaTask) string {
	if task.ParentGid != "" {
		return ticket.SUBTASK
	}
	return ticket.TASK
}

// matchPattern checks if the input string matches the regex pattern
func matchPattern(input, pattern string) bool {
	if pattern == "" {
		return false
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return false
	}
	return re.MatchString(input)
}

// getOriginalStatus returns the original status string
func getOriginalStatus(task *models.AsanaTask) string {
	if task.Completed {
		return "completed"
	}
	if task.SectionName != "" {
		return task.SectionName
	}
	return "incomplete"
}
