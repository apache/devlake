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
	"reflect"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

var ConvertIssueChangelogsMeta = plugin.SubTaskMeta{
	Name:             "Convert Issue Changelogs",
	EntryPoint:       ConvertIssueChangelogs,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_youtrack_issue_changelogs into domain layer table issue_changelogs (state => std status via mappings and the workflow_states isResolved lookup; assignee => didgen account ids; everything else pass-through)",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{
		models.YoutrackIssueChangelog{}.TableName(),
		models.YoutrackWorkflowState{}.TableName(),
		models.YoutrackIssue{}.TableName(),
		RAW_ISSUE_CHANGELOGS_TABLE,
	},
	ProductTables: []string{ticket.IssueChangelogs{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertIssueChangelogs

func ConvertIssueChangelogs(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*YoutrackTaskData)
	connectionId := data.Options.ConnectionId
	projectId := data.Options.ProjectId

	changelogIdGen := didgen.NewDomainIdGenerator(&models.YoutrackIssueChangelog{})
	issueIdGen := didgen.NewDomainIdGenerator(&models.YoutrackIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.YoutrackAccount{})

	// the effective state/assignee field names decide the value-column
	// treatment; the core slots fall back to their default names
	stateField := valueOrDefault(data.ScopeConfig.StateField, defaultStateField)
	assigneeField := valueOrDefault(data.ScopeConfig.AssigneeField, defaultAssigneeField)

	// isResolved lookup by state name, cached per run from
	// _tool_youtrack_workflow_states — the changelog payloads
	// predate any state renames, so the lookup degrades to TODO for names
	// the bundle no longer knows
	var states []models.YoutrackWorkflowState
	if err := db.All(&states, dal.Where("connection_id = ? AND project_id = ?", connectionId, projectId)); err != nil {
		return err
	}
	isResolvedByName := make(map[string]bool, len(states))
	for _, state := range states {
		isResolvedByName[state.Name] = state.IsResolved
	}

	// scope the changelogs to this project via their parent issue
	cursor, err := db.Cursor(
		dal.Select("c.*"),
		dal.From("_tool_youtrack_issue_changelogs c"),
		dal.Join("LEFT JOIN _tool_youtrack_issues i ON (i.connection_id = c.connection_id AND i.id = c.issue_id)"),
		dal.Where("c.connection_id = ? AND i.project_id = ?", connectionId, projectId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Options: models.YoutrackApiParams{
				ConnectionId: connectionId,
				ProjectId:    projectId,
			},
			Table: RAW_ISSUE_CHANGELOGS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.YoutrackIssueChangelog{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			changelog := inputRow.(*models.YoutrackIssueChangelog)
			domainChangelog := &ticket.IssueChangelogs{
				DomainEntity: domainlayer.DomainEntity{Id: changelogIdGen.Generate(connectionId, changelog.Id)},
				IssueId:      issueIdGen.Generate(connectionId, changelog.IssueId),
				// FieldId stays empty: YouTrack field ids are internal and
				// the domain layer keys on FieldName
				FieldId:           "",
				FieldName:         changelog.FieldName,
				OriginalFromValue: changelog.OriginalFromValue,
				OriginalToValue:   changelog.OriginalToValue,
				FromValue:         changelog.FromValue,
				ToValue:           changelog.ToValue,
				CreatedDate:       changelog.Created,
				AuthorName:        changelog.AuthorName,
			}
			if changelog.AuthorId != "" {
				domainChangelog.AuthorId = accountIdGen.Generate(connectionId, changelog.AuthorId)
			}
			var err errors.Error
			switch changelog.FieldName {
			case stateField:
				// std statuses per the issue rule: the configured
				// mapping, else isResolved from the workflow_states cache;
				// Original* keep the stored display names
				domainChangelog.FromValue, err = stdChangelogValues(data.ScopeConfig.StatusMappings, isResolvedByName, changelog.OriginalFromValue)
				if err != nil {
					return nil, err
				}
				domainChangelog.ToValue, err = stdChangelogValues(data.ScopeConfig.StatusMappings, isResolvedByName, changelog.OriginalToValue)
				if err != nil {
					return nil, err
				}
			case assigneeField:
				// didgen account ids from the stored user ids; Original*
				// keep the full names
				domainChangelog.FromValue, err = accountChangelogValues(accountIdGen, connectionId, changelog.FromValue)
				if err != nil {
					return nil, err
				}
				domainChangelog.ToValue, err = accountChangelogValues(accountIdGen, connectionId, changelog.ToValue)
				if err != nil {
					return nil, err
				}
			}
			// everything else passes through, multi-value JSON included —
			// the domain table has no category column
			return []interface{}{domainChangelog}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

// decodeChangelogValues reads a tool-layer changelog value column back:
// a JSON array for multi-value changes, the scalar otherwise.
func decodeChangelogValues(value string) []string {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "[") {
		var values []string
		if err := json.Unmarshal([]byte(value), &values); err == nil {
			return values
		}
	}
	return []string{value}
}

// stdChangelogValues maps the stored display names of a state change to std
// statuses via the issue rule (the shared StdStatusFor helper), preserving
// the scalar/JSON shape of the tool column.
func stdChangelogValues(statusMappings map[string]string, isResolvedByName map[string]bool, displays string) (string, errors.Error) {
	names := decodeChangelogValues(displays)
	stdValues := make([]string, 0, len(names))
	for _, name := range names {
		stdValues = append(stdValues, StdStatusFor(statusMappings, name, isResolvedByName[name]))
	}
	return marshalActivityValues(stdValues)
}

// accountChangelogValues maps the stored user ids of an assignee change to
// didgen account ids, preserving the scalar/JSON shape of the tool column.
func accountChangelogValues(accountIdGen *didgen.DomainIdGenerator, connectionId uint64, valueIds string) (string, errors.Error) {
	ids := decodeChangelogValues(valueIds)
	accountIds := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			accountIds = append(accountIds, "")
			continue
		}
		accountIds = append(accountIds, accountIdGen.Generate(connectionId, id))
	}
	return marshalActivityValues(accountIds)
}
