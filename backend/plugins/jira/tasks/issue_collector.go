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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/plugins/jira/models"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const RAW_ISSUE_TABLE = "jira_api_issues"

var _ plugin.SubTaskEntryPoint = CollectIssues

var CollectIssuesMeta = plugin.SubTaskMeta{
	Name:             "collectIssues",
	EntryPoint:       CollectIssues,
	EnabledByDefault: true,
	Description:      "collect Jira issues, supports both timeFilter and diffSync.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
}

func CollectIssues(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*JiraTaskData)
	logger := taskCtx.GetLogger()
	apiCollector, err := api.NewStatefulApiCollector(api.RawDataSubTaskArgs{
		Ctx: taskCtx,
		/*
			This struct will be JSONEncoded and stored into database along with raw data itself, to identity minimal
			set of data to be process, for example, we process JiraIssues by Board
		*/
		Params: JiraApiParams{
			ConnectionId: data.Options.ConnectionId,
			BoardId:      data.Options.BoardId,
		},
		/*
			Table store raw data
		*/
		Table: RAW_ISSUE_TABLE,
	})
	if err != nil {
		return err
	}

	// build jql
	// IMPORTANT: we have to keep paginated data in a consistence order to avoid data-missing, if we sort issues by
	//  `updated`, issue will be jumping between pages if it got updated during the collection process
	loc, err := getTimeZone(taskCtx)
	if err != nil {
		logger.Info("failed to get timezone, err: %v", err)
	} else {
		logger.Info("got user's timezone: %v", loc.String())
	}
	boardJQL, err := getBoardJQL(taskCtx.GetDal(), data)
	if err != nil {
		return err
	}
	jql := buildBoardIssueJQL(boardJQL, apiCollector.GetSince(), loc)

	if strings.EqualFold(string(data.JiraServerInfo.DeploymentType), string(models.DeploymentServer)) {
		logger.Info("Using api/2/search for Jira issue collection")
		err = setupIssueApiV2Collector(apiCollector, data, jql)
	} else {
		logger.Info("Using api/3/search/jql for Jira issue collection")
		err = setupIssueApiV3Collector(apiCollector, data, jql)
	}
	if err != nil {
		return err
	}

	return apiCollector.Execute()
}

func setupIssueApiV2Collector(apiCollector *api.StatefulApiCollector, data *JiraTaskData, jql string) errors.Error {
	return apiCollector.InitCollector(api.ApiCollectorArgs{
		ApiClient: data.ApiClient,
		PageSize:  data.Options.PageSize,
		/*
			url may use arbitrary variables from different connection in any order, we need GoTemplate to allow more
			flexible for all kinds of possibility.
			Pager contains information for a particular page, calculated by ApiCollector, and will be passed into
			GoTemplate to generate a url for that page.
			We want to do page-fetching in ApiCollector, because the logic are highly similar, by doing so, we can
			avoid duplicate logic for every tasks, and when we have a better idea like improving performance, we can
			do it in one place
		*/
		UrlTemplate: "api/2/search",
		/*
			(Optional) Return query string for request, or you can plug them into UrlTemplate directly
		*/
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("jql", jql)
			query.Set("startAt", fmt.Sprintf("%v", reqData.Pager.Skip))
			query.Set("maxResults", fmt.Sprintf("%v", reqData.Pager.Size))
			query.Set("expand", "changelog")
			query.Set("fields", "*all")
			return query, nil
		},
		/*
			Some api might do pagination by http headers
		*/
		//Header: func(pager *plugin.Pager) http.Header {
		//},
		/*
			Sometimes, we need to collect data based on previous collected data, like jira changelog, it requires
			issue_id as part of the url.
			We can mimic `stdin` design, to accept a `Input` function which produces a `Iterator`, collector
			should iterate all records, and do data-fetching for each on, either in parallel or sequential order
			UrlTemplate: "api/3/issue/{{ Input.ID }}/changelog"
		*/
		//Input: databaseIssuesIterator,
		/*
			For api endpoint that returns number of total pages, ApiCollector can collect pages in parallel with ease,
			or other techniques are required if this information was missing.
		*/
		GetTotalPages:  GetTotalPagesFromResponse,
		Concurrency:    10,
		ResponseParser: parseIssuesFromResponse,
	})
}

func setupIssueApiV3Collector(apiCollector *api.StatefulApiCollector, data *JiraTaskData, jql string) errors.Error {
	return apiCollector.InitCollector(api.ApiCollectorArgs{
		ApiClient:             data.ApiClient,
		PageSize:              data.Options.PageSize,
		UrlTemplate:           "api/3/search/jql",
		GetNextPageCustomData: getNextPageCustomDataForV3,
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("jql", jql)
			query.Set("maxResults", fmt.Sprintf("%v", reqData.Pager.Size))
			query.Set("expand", "changelog")
			query.Set("fields", "*all")
			if reqData.CustomData != nil {
				query.Set("nextPageToken", reqData.CustomData.(string))
			}
			return query, nil
		},
		ResponseParser: parseIssuesFromResponse,
	})
}

func parseIssuesFromResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	var data struct {
		Issues []json.RawMessage `json:"issues"`
	}
	blob, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Convert(err)
	}
	err = json.Unmarshal(blob, &data)
	if err != nil {
		return nil, errors.Convert(err)
	}
	return data.Issues, nil
}

func getBoardJQL(db dal.Dal, data *JiraTaskData) (string, errors.Error) {
	var record models.JiraBoard
	err := db.First(&record, dal.Where("connection_id = ? AND board_id = ? ", data.Options.ConnectionId, data.Options.BoardId))
	if err != nil {
		return "", errors.Default.Wrap(err, fmt.Sprintf("error finding record in _tool_jira_boards table for connection_id:%d board_id:%d", data.Options.ConnectionId, data.Options.BoardId))
	}
	if strings.TrimSpace(record.Jql) == "" {
		return "", errors.Default.New(fmt.Sprintf("connection_id:%d board_id:%d filter jql is empty, please run collectBoardFilterBegin first", data.Options.ConnectionId, data.Options.BoardId))
	}
	return record.Jql, nil
}

// buildJQL build jql based on timeAfter and incremental mode
func buildJQL(since time.Time, location *time.Location) string {
	updatedAfter := buildUpdatedAfterJQL(since, location)
	if updatedAfter == "" {
		return jiraIssueOrderBy
	}
	return fmt.Sprintf("%s %s", updatedAfter, jiraIssueOrderBy)
}

const jiraIssueOrderBy = "ORDER BY created ASC"

func buildBoardIssueJQL(boardJQL string, since *time.Time, location *time.Location) string {
	var clauses []string
	boardJQL = stripJQLOrderBy(boardJQL)
	if boardJQL != "" {
		clauses = append(clauses, fmt.Sprintf("(%s)", boardJQL))
	}
	if since != nil {
		updatedAfter := buildUpdatedAfterJQL(*since, location)
		if updatedAfter != "" {
			clauses = append(clauses, updatedAfter)
		}
	}
	if len(clauses) == 0 {
		return jiraIssueOrderBy
	}
	return fmt.Sprintf("%s %s", strings.Join(clauses, " AND "), jiraIssueOrderBy)
}

func buildUpdatedAfterJQL(since time.Time, location *time.Location) string {
	if since.IsZero() {
		return ""
	}
	if location != nil {
		since = since.In(location)
	} else {
		since = since.In(time.UTC).Add(-24 * time.Hour)
	}
	return fmt.Sprintf("updated >= '%s'", since.Format("2006/01/02 15:04"))
}

func stripJQLOrderBy(jql string) string {
	jql = strings.TrimSpace(jql)
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false
	for i := 0; i < len(jql); i++ {
		c := jql[i]
		if escaped {
			escaped = false
			continue
		}
		if (inSingleQuote || inDoubleQuote) && c == '\\' {
			escaped = true
			continue
		}
		if !inDoubleQuote && c == '\'' {
			inSingleQuote = !inSingleQuote
			continue
		}
		if !inSingleQuote && c == '"' {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if inSingleQuote || inDoubleQuote {
			continue
		}
		if isJQLOrderByAt(jql, i) {
			return strings.TrimSpace(jql[:i])
		}
	}
	return jql
}

func isJQLOrderByAt(jql string, index int) bool {
	order := "order"
	if !hasPrefixFold(jql[index:], order) {
		return false
	}
	before := index - 1
	if before >= 0 && !isJQLTokenBoundary(jql[before]) {
		return false
	}
	afterOrder := index + len(order)
	if afterOrder >= len(jql) || !isJQLWhitespace(jql[afterOrder]) {
		return false
	}
	byIndex := afterOrder
	for byIndex < len(jql) && isJQLWhitespace(jql[byIndex]) {
		byIndex++
	}
	by := "by"
	if !hasPrefixFold(jql[byIndex:], by) {
		return false
	}
	afterBy := byIndex + len(by)
	return afterBy == len(jql) || isJQLTokenBoundary(jql[afterBy])
}

func hasPrefixFold(s string, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func isJQLWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

func isJQLTokenBoundary(c byte) bool {
	return isJQLWhitespace(c) || c == '(' || c == ')'
}

// getTimeZone get user's timezone from jira API
func getTimeZone(taskCtx plugin.SubTaskContext) (*time.Location, errors.Error) {
	data := taskCtx.GetData().(*JiraTaskData)
	connectionId := data.Options.ConnectionId
	var conn models.JiraConnection
	err := taskCtx.GetDal().First(&conn, dal.Where("id = ?", connectionId))
	if err != nil {
		return nil, err
	}
	var resp *http.Response
	var path string
	var query url.Values
	if strings.EqualFold(string(data.JiraServerInfo.DeploymentType), string(models.DeploymentServer)) {
		path = "api/2/user"
		query = url.Values{"username": []string{conn.Username}}
	} else {
		path = "api/3/user"
		var accountId string
		accountId, err = getAccountId(data.ApiClient, conn.Username)
		if err != nil {
			return nil, err
		}
		query = url.Values{"accountId": []string{accountId}}
	}
	resp, err = data.ApiClient.Get(path, query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var timeZone struct {
		TimeZone string `json:"timeZone"`
	}
	err = errors.Convert(json.NewDecoder(resp.Body).Decode(&timeZone))
	if err != nil {
		return nil, err
	}
	tz, err := errors.Convert01(time.LoadLocation(timeZone.TimeZone))
	if err != nil {
		return nil, err
	}
	if tz == nil {
		return nil, errors.Default.New(fmt.Sprintf("invalid time zone: %s", timeZone.TimeZone))
	}
	return tz, nil
}

func getAccountId(client *api.ApiAsyncClient, username string) (string, errors.Error) {
	resp, err := client.Get("api/3/user/picker", url.Values{"query": []string{username}}, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var accounts struct {
		Users []struct {
			AccountID   string `json:"accountId"`
			AccountType string `json:"accountType"`
			HTML        string `json:"html"`
			DisplayName string `json:"displayName"`
		} `json:"users"`
		Total  int    `json:"total"`
		Header string `json:"header"`
	}
	err = errors.Convert(json.NewDecoder(resp.Body).Decode(&accounts))
	if err != nil {
		return "", err
	}
	if len(accounts.Users) == 0 {
		return "", errors.Default.New("no user found")
	}
	return accounts.Users[0].AccountID, nil
}
