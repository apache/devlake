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
	"hash/fnv"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

// hashUUID converts a UUID/hex string to a uint64 via FNV-1a.
// The CLOUDRHAI_MARTS schema uses UUID strings for changelog IDs instead of
// the numeric IDs that the upstream Jira API uses. We hash to uint64 to stay
// compatible with JiraIssueChangelogs.ChangelogId (uint64 primary key).
func hashUUID(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

var SyncChangelogsMeta = plugin.SubTaskMeta{
	Name:             "syncChangelogs",
	EntryPoint:       SyncChangelogs,
	EnabledByDefault: true,
	Description:      "Sync Jira issue changelogs from Snowflake into _tool_jira_issue_changelogs and _tool_jira_issue_changelog_items",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func SyncChangelogs(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	projectKeys := data.Options.ProjectKeys

	inClause, args := buildProjectInClause(projectKeys)
	syncPolicy := subtaskCtx.TaskContext().SyncPolicy()
	timeFilter := ""
	if syncPolicy != nil && syncPolicy.TimeAfter != nil {
		timeFilter = "AND cl.CREATED > ?"
		args = append(args, *syncPolicy.TimeAfter)
	}

	const queryTmpl = `
SELECT
    cl.ID                   AS changelog_id,
    cl.ISSUEID              AS issue_id,
    cl.CREATED,
    ci.FIELD,
    ci.FIELDTYPE,
    NULL                    AS field_id,
    ci.OLDVALUE             AS from_value,
    ci.OLDVALUE             AS from_string,
    ci.NEWVALUE             AS to_value,
    ci.NEWVALUE             AS to_string
FROM JIRA_CHANGEGROUP_RHAI_CLUSTERED cl
JOIN JIRA_CHANGEITEM_NON_PII_CLUSTERED ci ON ci.GROUPID = cl.ID
JOIN JIRA_ISSUE_NON_PII i                 ON i.ID = cl.ISSUEID
WHERE i.PROJECT IN %s
%s
`
	query := fmt.Sprintf(queryTmpl, inClause, timeFilter) //nolint:gosec // G201: inClause contains only '?' placeholders from buildProjectInClause; timeFilter is a static string

	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for changelogs")
	}
	defer rows.Close()

	changelogCount := 0
	itemCount := 0
	seenChangelogs := make(map[uint64]bool)

	for rows.Next() {
		var (
			changelogIdStr string
			issueId        uint64
			created        time.Time
			field          string
			fieldType      string
			fieldId        *string
			fromValue      *string
			fromString     *string
			toValue        *string
			toString       *string
		)
		if scanErr := rows.Scan(
			&changelogIdStr, &issueId, &created,
			&field, &fieldType, &fieldId,
			&fromValue, &fromString, &toValue, &toString,
		); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan Snowflake changelog row")
		}
		changelogId := hashUUID(changelogIdStr)

		// Upsert parent changelog header once per changelog ID
		if !seenChangelogs[changelogId] {
			cl := &jiramodels.JiraIssueChangelogs{
				ConnectionId: connectionId,
				ChangelogId:  changelogId,
				IssueId:      issueId,
				Created:      created,
			}
			if dbErr := db.CreateOrUpdate(cl); dbErr != nil {
				return dbErr
			}
			seenChangelogs[changelogId] = true
			changelogCount++
		}

		// Upsert changelog item
		item := &jiramodels.JiraIssueChangelogItems{
			ConnectionId: connectionId,
			ChangelogId:  changelogId,
			Field:        field,
			FieldType:    fieldType,
			FieldId:      stringVal(fieldId),
			FromValue:    stringVal(fromValue),
			FromString:   stringVal(fromString),
			ToValue:      stringVal(toValue),
			ToString:     stringVal(toString),
		}
		if dbErr := db.CreateOrUpdate(item); dbErr != nil {
			return dbErr
		}
		itemCount++
	}
	if goErr = rows.Err(); goErr != nil {
		return errors.Default.Wrap(goErr, "Snowflake changelog rows iteration error")
	}
	logger.Info("finished syncing %d changelogs (%d items)", changelogCount, itemCount)
	return nil
}
