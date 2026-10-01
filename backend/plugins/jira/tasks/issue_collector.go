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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira/models"
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
		Params: JiraApiParams{
			ConnectionId: data.Options.ConnectionId,
			BoardId:      data.Options.BoardId,
		},
		Table: RAW_ISSUE_TABLE,
	})
	if err != nil {
		return err
	}

	// IMPORTANT: we sort by `created ASC` to keep paginated data in a consistent order.
	// Sorting by `updated` would cause issues to jump between pages during collection.
	loc, err := getTimeZone(taskCtx)
	if err != nil {
		logger.Info("failed to get timezone, err: %v", err)
	} else {
		logger.Info("got user's timezone: %v", loc.String())
	}
	incrementalJql := "ORDER BY created ASC"
	if apiCollector.GetSince() != nil {
		incrementalJql = buildJQL(*apiCollector.GetSince(), loc)
	}

	// Use the search API with `filter = {id}` JQL instead of the board Agile API.
	// The board Agile API applies kanban sub-filters server-side, which silently
	// excludes resolved issues (e.g. those with a released fixVersion).
	// The search API with the saved filter JQL returns all matching issues.
	var extraJql string
	if data.Options.ScopeConfig != nil && data.Options.ScopeConfig.ExtraJQL != "" {
		renderedJql, renderErr := renderExtraJQL(data.Options.ScopeConfig.ExtraJQL, data)
		if renderErr != nil {
			return renderErr
		}
		extraJql = renderedJql
	}
	filterJql := buildFilterJQL(data.FilterId, extraJql, incrementalJql)
	logger.Info("collecting issues via search API with JQL: %s", filterJql)

	pageSize := data.Options.PageSize
	if pageSize == 0 {
		pageSize = 100
	}

	skipUnparseable := taskCtx.GetConfigReader().GetBool("JIRA_SKIP_UNPARSEABLE_ISSUES")
	if skipUnparseable {
		logger.Info("JIRA_SKIP_UNPARSEABLE_ISSUES is enabled, unparseable issue pages will be skipped")
	}

	if strings.EqualFold(string(data.JiraServerInfo.DeploymentType), string(models.DeploymentServer)) {
		logger.Info("Using api/2/search for JIRA Server issue collection")
		err = setupIssueV2Collector(apiCollector, data, filterJql, pageSize, skipUnparseable, logger)
	} else {
		logger.Info("Using api/3/search/jql for JIRA Cloud issue collection")
		err = setupIssueV3Collector(apiCollector, data, filterJql, pageSize, skipUnparseable, logger)
	}
	if err != nil {
		return err
	}

	return apiCollector.Execute()
}

// JqlTemplateData holds the variables available inside an ExtraJQL template.
// Users reference these with Go template syntax, e.g. `{{.BoardName}}`.
type JqlTemplateData struct {
	BoardId     uint64 // numeric ID of the connected Jira board
	BoardName   string // display name of the connected Jira board
	ProjectName string // DevLake project name; empty string if no project context
}

// renderExtraJQL executes the ExtraJQL scope-config field as a Go text/template,
// substituting board-level variables so the same scope config can produce
// different JQL for different boards.
//
// The template is parsed with an empty FuncMap (no built-in helpers such as
// printf) and missingkey=error so that typos in variable names produce an
// explicit error rather than silently rendering "<no value>".
func renderExtraJQL(tmplStr string, data *JiraTaskData) (string, errors.Error) {
	tmpl, err := template.New("extraJql").
		Funcs(template.FuncMap{}).
		Option("missingkey=error").
		Parse(tmplStr)
	if err != nil {
		return "", errors.BadInput.Wrap(err, "invalid ExtraJQL template")
	}

	vars := JqlTemplateData{
		BoardId:     data.Options.BoardId,
		ProjectName: data.ProjectName,
	}
	if data.Board != nil {
		vars.BoardName = data.Board.Name
	}

	var buf bytes.Buffer
	if execErr := tmpl.Execute(&buf, vars); execErr != nil {
		return "", errors.BadInput.Wrap(execErr, "failed to render ExtraJQL template")
	}
	return buf.String(), nil
}

// buildFilterJQL composes a final JQL query from three inputs:
//   - filterId: a Jira saved-filter ID (referenced via `filter = {id}`)
//   - extraJql: optional user-supplied JQL fragment appended as an AND condition
//     (e.g. `project = "MyComponent"`) to scope a large board down to one project
//   - incrementalJql: the time-based clause generated by buildJQL, always ending
//     with "ORDER BY created ASC"
//
// extraJql is wrapped in parentheses so that any OR/NOT operators inside it
// do not interfere with the surrounding AND chain.
func buildFilterJQL(filterId string, extraJql string, incrementalJql string) string {
	const orderBy = "ORDER BY created ASC"

	var conditions []string
	if filterId != "" {
		conditions = append(conditions, fmt.Sprintf("filter = %s", filterId))
	}
	if extraJql != "" {
		conditions = append(conditions, fmt.Sprintf("(%s)", extraJql))
	}
	if incrementalJql != orderBy {
		// strip the trailing " ORDER BY created ASC" to isolate the time condition
		conditions = append(conditions, strings.TrimSuffix(incrementalJql, " "+orderBy))
	}

	if len(conditions) == 0 {
		return orderBy
	}
	return strings.Join(conditions, " AND ") + " " + orderBy
}

// parseIssuesResponse extracts the `issues` array from a Jira search response.
//
// A response that arrived with a successful status but carries a body which is not the
// expected JSON - a truncated payload, or an HTML error page substituted by a proxy -
// normally fails the whole collectIssues subtask, discarding an otherwise complete sync
// because of a single page. When skipUnparseable is set the page is logged and skipped
// instead. Non-2xx responses never reach this point; they are handled by the retry logic
// in the API client.
func parseIssuesResponse(res *http.Response, skipUnparseable bool, logger log.Logger) ([]json.RawMessage, errors.Error) {
	blob, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Convert(err)
	}
	var body struct {
		Issues []json.RawMessage `json:"issues"`
	}
	if err := json.Unmarshal(blob, &body); err != nil {
		if !skipUnparseable {
			return nil, errors.Convert(err)
		}
		if logger != nil {
			logger.Warn(err, "skipping unparseable issue page from %s (%d bytes)", responseUrl(res), len(blob))
		}
		return []json.RawMessage{}, nil
	}
	return body.Issues, nil
}

// responseUrl reports the request URL behind a response, for log messages. The request is
// always populated on responses returned by the API client, but a hand-built response in a
// test may not carry one.
func responseUrl(res *http.Response) string {
	if res == nil || res.Request == nil || res.Request.URL == nil {
		return "unknown url"
	}
	return res.Request.URL.String()
}

func setupIssueV2Collector(apiCollector *api.StatefulApiCollector, data *JiraTaskData, filterJql string, pageSize int, skipUnparseable bool, logger log.Logger) errors.Error {
	return apiCollector.InitCollector(api.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		PageSize:    pageSize,
		UrlTemplate: "api/2/search",
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("jql", filterJql)
			query.Set("startAt", fmt.Sprintf("%v", reqData.Pager.Skip))
			query.Set("maxResults", fmt.Sprintf("%v", reqData.Pager.Size))
			query.Set("expand", "changelog")
			query.Set("fields", "*all")
			return query, nil
		},
		GetTotalPages: GetTotalPagesFromResponse,
		Concurrency:   10,
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			return parseIssuesResponse(res, skipUnparseable, logger)
		},
	})
}

func setupIssueV3Collector(apiCollector *api.StatefulApiCollector, data *JiraTaskData, filterJql string, pageSize int, skipUnparseable bool, logger log.Logger) errors.Error {
	return apiCollector.InitCollector(api.ApiCollectorArgs{
		ApiClient:             data.ApiClient,
		PageSize:              pageSize,
		UrlTemplate:           "api/3/search/jql",
		GetNextPageCustomData: getNextPageCustomDataForV3,
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("jql", filterJql)
			query.Set("maxResults", fmt.Sprintf("%v", reqData.Pager.Size))
			query.Set("expand", "changelog")
			query.Set("fields", "*all")
			if reqData.CustomData != nil {
				query.Set("nextPageToken", reqData.CustomData.(string))
			}
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			return parseIssuesResponse(res, skipUnparseable, logger)
		},
	})
}

// buildJQL build jql based on timeAfter and incremental mode
func buildJQL(since time.Time, location *time.Location) string {
	jql := "ORDER BY created ASC"
	if !since.IsZero() {
		if location != nil {
			since = since.In(location)
		} else {
			since = since.In(time.UTC).Add(-24 * time.Hour)
		}
		jql = fmt.Sprintf("updated >= '%s' %s", since.Format("2006/01/02 15:04"), jql)
	}
	return jql
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
