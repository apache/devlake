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
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

var ExtractIssuesMeta = plugin.SubTaskMeta{
	Name:             "Extract Issues",
	EntryPoint:       ExtractIssues,
	EnabledByDefault: true,
	Description:      "Extract raw issue data into _tool_youtrack_issues, _tool_youtrack_issue_labels and _tool_youtrack_accounts; applies the scope config's field-name slots and type/status mappings at extraction time",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{RAW_ISSUES_TABLE},
	ProductTables: []string{
		models.YoutrackIssue{}.TableName(),
		models.YoutrackIssueLabel{}.TableName(),
		models.YoutrackAccount{}.TableName(),
	},
}

var _ plugin.SubTaskEntryPoint = ExtractIssues

// apiIssue mirrors the issue collector's fields param (issueFields).
type apiIssue struct {
	Id              string `json:"id"`
	IdReadable      string `json:"idReadable"`
	NumberInProject int64  `json:"numberInProject"`
	Created         int64  `json:"created"`
	Updated         int64  `json:"updated"`
	Resolved        *int64 `json:"resolved"`
	CommentsCount   int    `json:"commentsCount"`
	Summary         string `json:"summary"`
	Description     string `json:"description"`
	Project         *struct {
		Id        string `json:"id"`
		ShortName string `json:"shortName"`
		Name      string `json:"name"`
	} `json:"project"`
	Reporter *apiUser `json:"reporter"`
	Updater  *apiUser `json:"updater"`
	Tags     []struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"tags"`
	CustomFields []apiCustomField `json:"customFields"`
}

func ExtractIssues(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*YoutrackTaskData)
	db := taskCtx.GetDal()
	connectionId := data.Options.ConnectionId
	projectId := data.Options.ProjectId
	// accounts derive inline from user refs only when CROSS is collected;
	// the guest account is skipped at conversion
	deriveAccounts := utils.StringsContains(data.ScopeConfig.Entities, plugin.DOMAIN_TYPE_CROSS)
	resolver := newCustomFieldResolver(data.ScopeConfig, taskCtx.GetLogger())

	extractor, err := helper.NewStatefulApiExtractor(&helper.StatefulApiExtractorArgs[apiIssue]{
		SubtaskCommonArgs: &helper.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_ISSUES_TABLE,
			Params: models.YoutrackApiParams{
				ConnectionId: connectionId,
				ProjectId:    projectId,
			},
			// the whole scope config: any change (field slots or mappings)
			// switches the extractor to full sync, re-applying the new config
			// from the raw layer without new YouTrack API calls
			SubtaskConfig: data.ScopeConfig,
		},
		BeforeExtract: func(issue *apiIssue, stateManager *helper.SubtaskStateManager) errors.Error {
			// replace the issue's labels on incremental re-extraction; the
			// issue/account rows themselves upsert by PK. On full sync the
			// batch-save divider wipes the params' rows instead.
			if stateManager.IsIncremental() {
				return db.Delete(
					&models.YoutrackIssueLabel{},
					dal.Where("connection_id = ? AND issue_id = ?", connectionId, issue.Id),
				)
			}
			return nil
		},
		Extract: func(apiIssue *apiIssue, row *helper.RawData) ([]interface{}, errors.Error) {
			fields, err := resolver.resolve(apiIssue.CustomFields)
			if err != nil {
				return nil, err
			}
			customFieldsJson, err := errors.Convert01(json.Marshal(apiIssue.CustomFields))
			if err != nil {
				return nil, err
			}
			created := time.UnixMilli(apiIssue.Created).UTC()
			var resolved *time.Time
			if apiIssue.Resolved != nil {
				r := time.UnixMilli(*apiIssue.Resolved).UTC()
				resolved = &r
			}
			issue := &models.YoutrackIssue{
				ConnectionId:     connectionId,
				Id:               apiIssue.Id,
				IdReadable:       apiIssue.IdReadable,
				NumberInProject:  apiIssue.NumberInProject,
				ProjectId:        projectId,
				Summary:          apiIssue.Summary,
				Description:      apiIssue.Description,
				Created:          created,
				Updated:          time.UnixMilli(apiIssue.Updated).UTC(),
				Resolved:         resolved,
				CommentsCount:    apiIssue.CommentsCount,
				StateName:        fields.StateName,
				StateIsResolved:  fields.StateIsResolved,
				TypeName:         fields.TypeName,
				PriorityName:     fields.PriorityName,
				StoryPoint:       fields.StoryPoint,
				DueDate:          fields.DueDate,
				StdType:          StdTypeFor(data.ScopeConfig.TypeMappings, fields.TypeName),
				StdStatus:        StdStatusFor(data.ScopeConfig.StatusMappings, fields.StateName, fields.StateIsResolved),
				LeadTimeMinutes:  leadTimeMinutesFor(created, resolved),
				CustomFieldsJson: string(customFieldsJson),
			}
			if apiIssue.Reporter != nil {
				issue.ReporterId = apiIssue.Reporter.Id
				issue.ReporterName = apiIssue.Reporter.FullName
			}
			if fields.Assignee != nil {
				issue.AssigneeId = fields.Assignee.Id
				issue.AssigneeName = fields.Assignee.FullName
			}

			results := make([]interface{}, 0, len(apiIssue.Tags)+4)
			results = append(results, issue)
			for _, tag := range apiIssue.Tags {
				if tag.Name == "" {
					continue
				}
				results = append(results, &models.YoutrackIssueLabel{
					ConnectionId: connectionId,
					IssueId:      apiIssue.Id,
					LabelName:    tag.Name,
				})
			}
			if deriveAccounts {
				for _, user := range []*apiUser{apiIssue.Reporter, apiIssue.Updater, fields.Assignee} {
					if account := deriveAccount(connectionId, user); account != nil {
						results = append(results, account)
					}
				}
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

// deriveAccount maps a user reference to a tool-layer account row, or nil
// when the ref is absent or id-less (upserts by PK dedupe repeats).
func deriveAccount(connectionId uint64, user *apiUser) *models.YoutrackAccount {
	if user == nil || user.Id == "" {
		return nil
	}
	return &models.YoutrackAccount{
		ConnectionId: connectionId,
		Id:           user.Id,
		Login:        user.Login,
		FullName:     user.FullName,
		Email:        user.Email,
		AvatarUrl:    user.AvatarUrl,
		Guest:        user.Guest,
	}
}
