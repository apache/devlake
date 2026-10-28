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
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks/apiv2models"
)

var _ plugin.SubTaskEntryPoint = CollectParentIssues

var CollectParentIssuesMeta = plugin.SubTaskMeta{
	Name:             "collectParentIssues",
	EntryPoint:       CollectParentIssues,
	EnabledByDefault: true,
	Description:      "collect parent issues (Features, Outcomes) referenced by epic_key but not in current collection",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

// CollectParentIssues collects parent issues that are referenced in epic_key field
// but were not collected due to JQL filter restrictions (e.g., Features in a different project)
func CollectParentIssues(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*JiraTaskData)
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	connectionId := data.Options.ConnectionId
    boardId := data.Options.BoardId

	logger.Info("collecting parent issues for connection_id=%d, board_id=%d", connectionId, data.Options.BoardId)

	// Collect parent issues iteratively (they may have their own parents)
	maxIterations := 10 // Prevent infinite loops
	totalCollected := 0

	for iteration := 0; iteration < maxIterations; iteration++ {
		// Find all unique epic_key values that reference issues not in our collection
		var epicKeys []struct {
			EpicKey string
		}
		err := db.All(&epicKeys,
            dal.Select("DISTINCT epic_key"),
            dal.From(&models.JiraIssue{}),
            dal.Where("connection_id = ? AND epic_key IS NOT NULL AND epic_key != '' AND issue_id IN (SELECT issue_id FROM _tool_jira_board_issues WHERE connection_id = ? AND board_id = ?)", connectionId, connectionId, boardId),
		)
		if err != nil {
			return errors.Default.Wrap(err, "failed to query epic_key values")
		}

		if len(epicKeys) == 0 {
			logger.Info("no parent keys found, skipping parent issue collection")
			break
		}

		// Convert to string slice
		var parentKeys []string
		for _, ek := range epicKeys {
			parentKeys = append(parentKeys, ek.EpicKey)
		}

		// Filter out keys that are already collected
		var existingIssues []struct {
			IssueKey string
		}
		err = db.All(&existingIssues,
			dal.Select("DISTINCT issue_key"),
			dal.From(&models.JiraIssue{}),
			dal.Where("connection_id = ? AND issue_key IN ?", connectionId, parentKeys),
		)
		if err != nil {
			return errors.Default.Wrap(err, "failed to query existing issue keys")
		}

		existingKeyMap := make(map[string]bool)
		for _, issue := range existingIssues {
			existingKeyMap[issue.IssueKey] = true
		}

		var keysToCollect []string
		for _, key := range parentKeys {
			if !existingKeyMap[key] {
				keysToCollect = append(keysToCollect, key)
			}
		}

		if len(keysToCollect) == 0 {
			logger.Info("iteration %d: all parent issues already collected", iteration+1)
			break
		}

		logger.Info("iteration %d: collecting %d missing parent issues: %v", iteration+1, len(keysToCollect), keysToCollect)

		// Collect each parent issue directly
		for _, issueKey := range keysToCollect {
			err = collectAndExtractSingleIssue(taskCtx, data, db, issueKey)
			if err != nil {
				// Log but don't fail - the issue might not exist or we might not have permission
				logger.Warn(err, "failed to collect parent issue %s, skipping", issueKey)
				continue
			}
			totalCollected++
		}
	}

	logger.Info("collected %d parent issues in total", totalCollected)
	return nil
}

// collectAndExtractSingleIssue collects a single issue by key and extracts it
func collectAndExtractSingleIssue(taskCtx plugin.SubTaskContext, data *JiraTaskData, db dal.Dal, issueKey string) errors.Error {
	logger := taskCtx.GetLogger()

	// Fetch the issue from Jira API
	path := fmt.Sprintf("api/2/issue/%s", issueKey)
	query := url.Values{}
	query.Set("expand", "changelog")

	resp, err := data.ApiClient.Get(path, query, nil)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("failed to fetch issue %s", issueKey))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		logger.Info("issue %s not found, skipping", issueKey)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return errors.Default.New(fmt.Sprintf("unexpected status code %d for issue %s", resp.StatusCode, issueKey))
	}

	blob, err := errors.Convert01(io.ReadAll(resp.Body))
	if err != nil {
		return errors.Default.Wrap(err, "failed to read response body")
	}

	// Parse and extract the issue
	var issue apiv2models.Issue
	err = errors.Convert(json.Unmarshal(blob, &issue))
	if err != nil {
		return errors.Default.Wrap(err, "failed to parse issue JSON")
	}

	err = issue.SetAllFields(blob)
	if err != nil {
		return err
	}

	if issue.Fields.Created == nil {
		logger.Info("issue %s has no created date, skipping", issueKey)
		return nil
	}

	// Get type mappings
	mappings, err := getTypeMappings(data, db)
	if err != nil {
		return err
	}

	userFieldMap, err := getUserFieldMap(db, data.Options.ConnectionId, logger)
	if err != nil {
		return err
	}

	// Extract entities
	_, jiraIssue, comments, worklogs, changelogs, changelogItems, users := issue.ExtractEntities(data.Options.ConnectionId, userFieldMap)

	// Extract epic key from custom field if configured
	if data.Options.ScopeConfig != nil && data.Options.ScopeConfig.EpicKeyField != "" {
		unknownEpicKey := issue.Fields.AllFields[data.Options.ScopeConfig.EpicKeyField]
		switch ek := unknownEpicKey.(type) {
		case string:
			jiraIssue.EpicKey = ek
		case map[string]interface{}:
			if key, ok := ek["key"].(string); ok {
				jiraIssue.EpicKey = key
			}
		}
	}

	// Set resolution date and lead time
	if jiraIssue.ResolutionDate != nil {
		temp := uint(jiraIssue.ResolutionDate.Unix()-jiraIssue.Created.Unix()) / 60
		jiraIssue.LeadTimeMinutes = &temp
	}

	// Set type mappings
	jiraIssue.Type = mappings.TypeIdMappings[jiraIssue.Type]
	jiraIssue.StdType = mappings.StdTypeMappings[jiraIssue.Type]
	if jiraIssue.StdType == "" {
		jiraIssue.StdType = strings.ToUpper(jiraIssue.Type)
	}
	jiraIssue.StdStatus = getStdStatus(jiraIssue.StatusKey)
	if value, ok := mappings.StandardStatusMappings[jiraIssue.Type][jiraIssue.StatusKey]; ok {
		jiraIssue.StdStatus = value.StandardStatus
	}

	// Save the issue
	err = db.CreateOrUpdate(jiraIssue)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("failed to save issue %s", issueKey))
	}

	// Save comments
	for _, comment := range comments {
		err = db.CreateOrUpdate(comment)
		if err != nil {
			logger.Warn(err, "failed to save comment for issue %s", issueKey)
		}
	}

	// Save worklogs
	for _, worklog := range worklogs {
		err = db.CreateOrUpdate(worklog)
		if err != nil {
			logger.Warn(err, "failed to save worklog for issue %s", issueKey)
		}
	}

	// Save changelogs
	var issueUpdated *time.Time
	if len(changelogs) < 100 {
		issueUpdated = &jiraIssue.Updated
	}
	for _, changelog := range changelogs {
		changelog.IssueUpdated = issueUpdated
		err = db.CreateOrUpdate(changelog)
		if err != nil {
			logger.Warn(err, "failed to save changelog for issue %s", issueKey)
		}
	}

	// Save changelog items
	for _, changelogItem := range changelogItems {
		err = db.CreateOrUpdate(changelogItem)
		if err != nil {
			logger.Warn(err, "failed to save changelog item for issue %s", issueKey)
		}
	}

	// Save users
	for _, user := range users {
		if user.AccountId != "" {
			err = db.CreateOrUpdate(user)
			if err != nil {
				logger.Warn(err, "failed to save user for issue %s", issueKey)
			}
		}
	}

	logger.Info("successfully collected and extracted parent issue %s (%s)", issueKey, jiraIssue.Type)
	return nil
}
