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
	"strings"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

var SyncIssuesMeta = plugin.SubTaskMeta{
	Name:             "syncIssues",
	EntryPoint:       SyncIssues,
	EnabledByDefault: true,
	Description:      "Sync Jira issues from Snowflake into _tool_jira_issues and _tool_jira_board_issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func SyncIssues(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	boardId := data.Options.BoardId
	projectKeys := data.Options.ProjectKeys

	mappings, err := getTypeMappings(data, db)
	if err != nil {
		return err
	}

	var timeAfter *time.Time
	syncPolicy := subtaskCtx.TaskContext().SyncPolicy()
	if syncPolicy != nil {
		timeAfter = syncPolicy.TimeAfter
	}

	query, args := buildIssuesQuery(projectKeys, timeAfter)

	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for issues")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			issueId                  uint64
			issueKey                 string
			projectId                uint64
			projectKey               string
			projectName              string
			issueType                string
			statusName               string
			statusKey                string
			summary                  *string
			description              *string
			created                  time.Time
			updated                  time.Time
			resolutionDate           *time.Time
			dueDate                  *time.Time
			parentId                 *uint64
			originalEstimateSeconds  *int64
			remainingEstimateSeconds *int64
			timeSpentSeconds         *int64
			storyPoint               *float64
			epicKey                  *string
			sprintId                 *uint64
			isSubtask                bool
			priorityName             *string
			components               *string
			fixVersions              *string
		)
		if scanErr := rows.Scan(
			&issueId, &issueKey, &projectId, &projectKey, &projectName,
			&issueType, &statusName, &statusKey,
			&summary, &description,
			&created, &updated, &resolutionDate, &dueDate,
			&parentId, &originalEstimateSeconds, &remainingEstimateSeconds,
			&timeSpentSeconds, &storyPoint, &epicKey, &sprintId, &isSubtask,
			&priorityName, &components, &fixVersions,
		); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan Snowflake issue row")
		}

		// Compute StdType from scope config
		stdType := mappings.StdTypeMappings[issueType]
		if stdType == "" {
			stdType = strings.ToUpper(issueType)
		}

		// Compute StdStatus — first the category default, then scope config override
		stdStatus := getStdStatus(statusKey)
		if overrides, ok := mappings.StandardStatusMappings[issueType]; ok {
			if v, ok2 := overrides[statusKey]; ok2 {
				stdStatus = v.StandardStatus
			}
		}

		issue := &jiramodels.JiraIssue{
			ConnectionId: connectionId,
			IssueId:      issueId,
			ProjectId:    uint64(projectId),
			ProjectName:  projectName,
			IssueKey:     issueKey,
			Summary:      stringVal(summary),
			Description:  stringVal(description),
			Type:         issueType,
			StatusName:   statusName,
			StatusKey:    statusKey,
			StdType:      stdType,
			StdStatus:    stdStatus,
			StoryPoint:   storyPoint,
			EpicKey:      stringVal(epicKey),
			PriorityName: stringVal(priorityName),
			Components:   stringVal(components),
			FixVersions:  stringVal(fixVersions),
			Created:      created,
			Updated:      updated,
			Subtask:      isSubtask,
		}
		if originalEstimateSeconds != nil {
			minutes := *originalEstimateSeconds / 60
			issue.OriginalEstimateMinutes = &minutes
		}
		if remainingEstimateSeconds != nil {
			issue.RemainingEstimateMinutes = *remainingEstimateSeconds / 60
		}
		if timeSpentSeconds != nil {
			minutes := *timeSpentSeconds / 60
			issue.SpentMinutes = &minutes
		}
		issue.ResolutionDate = resolutionDate
		issue.DueDate = dueDate
		if parentId != nil && *parentId != 0 {
			issue.ParentId = *parentId
		}
		if sprintId != nil {
			issue.SprintId = *sprintId
		}

		if dbErr := db.CreateOrUpdate(issue); dbErr != nil {
			return dbErr
		}

		// Maintain _tool_jira_board_issues so convertor scope queries work
		boardIssue := &jiramodels.JiraBoardIssue{
			ConnectionId: connectionId,
			BoardId:      boardId,
			IssueId:      issueId,
		}
		if dbErr := db.CreateOrUpdate(boardIssue); dbErr != nil {
			return dbErr
		}

		count++
		if count%500 == 0 {
			logger.Info("synced %d issues", count)
			subtaskCtx.SetProgress(count, -1)
		}
	}
	if goErr = rows.Err(); goErr != nil {
		return errors.Default.Wrap(goErr, "Snowflake issue rows iteration error")
	}
	logger.Info("finished syncing %d issues for board %d", count, boardId)
	return nil
}

// buildIssuesQuery constructs the parameterized Snowflake SELECT for issues.
// Returns the query template (with ? placeholders) and the bound arguments.
// Extracted as a pure function so it can be unit-tested independently of the DB.
func buildIssuesQuery(projectKeys []string, timeAfter *time.Time) (string, []interface{}) {
	inClause, args := buildProjectInClause(projectKeys)

	timeFilter := ""
	if timeAfter != nil {
		timeFilter = "AND i.UPDATED > ?"
		args = append(args, *timeAfter)
	}

	const queryTmpl = `
SELECT
    i.ID                                                    AS issue_id,
    i.ISSUE_KEY,
    p.ID                                                    AS project_id,
    i.PROJECT                                               AS project_key,
    p.PNAME                                                 AS project_name,
    it.PNAME                                                AS issue_type,
    s.PNAME                                                 AS status_name,
    -- redhat.atlassian.net statusCategory ids: 2=new, 3=done, 4=indeterminate
    -- (differs from older Atlassian docs that listed 3=indeterminate, 4=done).
    CASE s.STATUSCATEGORY
        WHEN '2' THEN 'new'
        WHEN '3' THEN 'done'
        WHEN '4' THEN 'indeterminate'
        ELSE          'indeterminate'
    END                                                     AS status_key,
    i.SUMMARY,
    i.DESCRIPTION,
    i.CREATED,
    i.UPDATED,
    i.RESOLUTIONDATE,
    i.DUEDATE,
    i.PARENT_ID,
    i.TIMEORIGINALESTIMATE                                  AS original_estimate_seconds,
    i.TIMEESTIMATE                                          AS remaining_estimate_seconds,
    i.TIMESPENT                                             AS time_spent_seconds,
    sp.NUMBERVALUE::FLOAT                                   AS story_point,
    el.STRINGVALUE                                          AS epic_key,
    sprint_cf.NUMBERVALUE::BIGINT                          AS sprint_id,
    COALESCE(it.PSTYLE = 'subtask', FALSE)                  AS is_subtask,
    i.PRIORITY                                              AS priority_name,
    i.COMPONENT                                             AS components,
    i.FIXFOR                                                AS fix_versions
FROM JIRA_ISSUE_NON_PII i
JOIN JIRA_ISSUETYPE_RHAI   it      ON i.ISSUETYPE      = it.ID
JOIN JIRA_ISSUESTATUS_RHAI s       ON i.ISSUESTATUS_ID  = s.ID
JOIN JIRA_PROJECT_RHAI     p       ON i.PROJECT         = p.PKEY
LEFT JOIN JIRA_CUSTOMFIELDVALUE_NON_PII sp
    ON sp.ISSUE = i.ID AND sp.CUSTOMFIELD_NAME = 'Story Points'
LEFT JOIN JIRA_CUSTOMFIELDVALUE_NON_PII el
    ON el.ISSUE = i.ID AND el.CUSTOMFIELD_NAME = 'Epic Link'
LEFT JOIN JIRA_CUSTOMFIELDVALUE_NON_PII sprint_cf
    ON sprint_cf.ISSUE = i.ID AND sprint_cf.CUSTOMFIELD_NAME = 'Sprint'
WHERE i.PROJECT IN %s
%s
QUALIFY ROW_NUMBER() OVER (
    PARTITION BY i.ID
    ORDER BY sprint_cf.NUMBERVALUE DESC NULLS LAST,
             sp.NUMBERVALUE         DESC NULLS LAST
) = 1
`
	query := fmt.Sprintf(queryTmpl, inClause, timeFilter) //nolint:gosec // G201: inClause contains only '?' placeholders from buildProjectInClause; timeFilter is a static string
	return query, args
}

func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
