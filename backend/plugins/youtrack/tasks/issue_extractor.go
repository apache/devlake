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

	// latestVersionByIssue is resolved against the extractor's window after
	// construction (below) and read by Extract at Execute time; declare it
	// before the closure that captures it.
	var latestVersionByIssue map[string]uint64
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
			// Replace the issue's labels on EVERY extraction, both modes: the
			// issue/account rows themselves upsert by PK, but labels are a
			// per-issue child collection. Only the newest retained version of
			// an issue reaches Extract (see latestVersionByIssue below), so
			// this delete can never race an older version's buffered writes.
			// The divider's full-sync wipe alone would not do: it fires only
			// when a label row is actually emitted, so an all-tagless replay
			// would leave the previous run's labels in place.
			return db.Delete(
				&models.YoutrackIssueLabel{},
				dal.Where("connection_id = ? AND issue_id = ?", connectionId, issue.Id),
			)
		},
		Extract: func(apiIssue *apiIssue, row *helper.RawData) ([]interface{}, errors.Error) {
			// a stale retained version (superseded by a newer raw row within
			// this run's window) must not expand its child collections — it
			// would resurrect tags the newest version no longer carries
			if latestId, ok := latestVersionByIssue[apiIssue.Id]; ok && row.ID != latestId {
				return nil, nil
			}
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

	// Mapping replay re-extracts every retained raw version of an
	// issue, in raw-id order; an older version would resurrect labels a
	// newer version removed (its delete runs before the newer version's
	// buffered writes land). Resolve the newest raw row per issue over
	// exactly the window the extractor is about to walk, so Extract can
	// expand only the current snapshot of each issue.
	latestVersionByIssue, err = latestIssueRawVersions(db, extractor)
	if err != nil {
		return err
	}
	return extractor.Execute()
}

// latestIssueRawVersions maps issue id -> newest raw row id over the same
// window the stateful extractor will process (its params, its incremental
// since, its until). Versions of one issue always arrive in raw-id order,
// so the newest version is the one with the highest raw id.
func latestIssueRawVersions(db dal.Dal, extractor *helper.StatefulApiExtractor[apiIssue]) (map[string]uint64, errors.Error) {
	latest := map[string]uint64{}
	table := extractor.GetRawDataTable()
	if !db.HasTable(table) {
		return latest, nil
	}
	clauses := []dal.Clause{
		dal.Select("id, data"),
		dal.From(table),
		dal.Where("params = ?", extractor.GetRawDataParams()),
		dal.Orderby("id ASC"),
	}
	if extractor.IsIncremental() {
		if since := extractor.GetSince(); since != nil {
			clauses = append(clauses, dal.Where("created_at >= ?", *since))
		}
	}
	clauses = append(clauses, dal.Where("created_at < ?", *extractor.GetUntil()))
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()
	for cursor.Next() {
		row := &helper.RawData{}
		if err := db.Fetch(cursor, row); err != nil {
			return nil, errors.Default.Wrap(err, "error scanning raw issue versions")
		}
		var head struct {
			Id string `json:"id"`
		}
		// a row whose id cannot be read is simply absent from the map:
		// Extract's miss-then-process default keeps it on the safe path
		if err := json.Unmarshal(row.Data, &head); err != nil || head.Id == "" {
			continue
		}
		latest[head.Id] = row.ID // id ASC order: the last write wins
	}
	return latest, nil
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
