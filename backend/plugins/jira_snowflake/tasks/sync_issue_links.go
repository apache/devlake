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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

var SyncIssueLinksMeta = plugin.SubTaskMeta{
	Name:             "syncIssueLinks",
	EntryPoint:       SyncIssueLinks,
	EnabledByDefault: false,
	Description:      "Sync Jira issue links from Snowflake into _tool_jira_issue_relationships",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func SyncIssueLinks(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	projectKeys := data.Options.ProjectKeys

	inClause, args := buildProjectInClause(projectKeys)

	const queryTmpl = `
SELECT
    il.ID               AS link_id,
    il.SOURCE           AS issue_id,
    src.ISSUE_KEY       AS issue_key,
    lt.ID               AS type_id,
    lt.LINKNAME         AS type_name,
    lt.INWARD,
    lt.OUTWARD,
    il.DESTINATION      AS inward_issue_id,
    dst.ISSUE_KEY       AS inward_issue_key
FROM JIRA_ISSUELINK il
JOIN JIRA_ISSUE_NON_PII src ON src.ID = il.SOURCE
JOIN JIRA_ISSUE_NON_PII dst ON dst.ID = il.DESTINATION
JOIN JIRA_ISSUELINKTYPE lt   ON lt.ID = il.LINKTYPE
WHERE src.PROJECT IN %s
`
	query := fmt.Sprintf(queryTmpl, inClause) //nolint:gosec // G201: inClause contains only '?' placeholders from buildProjectInClause; no user data is interpolated

	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for issue links")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			linkId         uint64
			issueId        uint64
			issueKey       string
			typeId         uint64
			typeName       string
			inward         string
			outward        string
			inwardIssueId  uint64
			inwardIssueKey string
		)
		if scanErr := rows.Scan(
			&linkId, &issueId, &issueKey,
			&typeId, &typeName, &inward, &outward,
			&inwardIssueId, &inwardIssueKey,
		); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan Snowflake issue link row")
		}

		rel := &jiramodels.JiraIssueRelationship{
			ConnectionId:   connectionId,
			IssueId:        issueId,
			IssueKey:       issueKey,
			TypeId:         typeId,
			TypeName:       typeName,
			Inward:         inward,
			Outward:        outward,
			InwardIssueId:  inwardIssueId,
			InwardIssueKey: inwardIssueKey,
		}
		if dbErr := db.CreateOrUpdate(rel); dbErr != nil {
			return dbErr
		}
		count++
	}
	if goErr = rows.Err(); goErr != nil {
		return errors.Default.Wrap(goErr, "Snowflake issue link rows iteration error")
	}
	logger.Info("finished syncing %d issue links", count)
	return nil
}
