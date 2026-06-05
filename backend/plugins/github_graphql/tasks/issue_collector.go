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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/utils"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
	githubTasks "github.com/apache/incubator-devlake/plugins/github/tasks"
	"github.com/merico-ai/graphql"
)

const RAW_ISSUES_TABLE = "github_graphql_issues"

type GraphqlQueryIssueWrapper struct {
	RateLimit struct {
		Cost int
	}
	Repository struct {
		IssueList struct {
			TotalCount graphql.Int
			Issues     []GraphqlQueryIssue `graphql:"nodes"`
			PageInfo   *api.GraphqlQueryPageInfo
		} `graphql:"issues(first: $pageSize, after: $skipCursor, orderBy: {field: CREATED_AT, direction: DESC})"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

type GraphqlQueryIssueDetailWrapper struct {
	RateLimit struct {
		Cost int
	}
	Repository struct {
		Issues []GraphqlQueryIssue `graphql:"issue(number: $number)" graphql-extend:"true"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

type GraphqlQueryIssue struct {
	DatabaseId   int
	Number       int
	State        string
	StateReason  string
	Title        string
	Body         string
	Author       *GraphqlInlineAccountQuery
	Url          string
	ClosedAt     *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	AssigneeList struct {
		// FIXME now domain layer just support one assignee
		Assignees []GraphqlInlineAccountQuery `graphql:"nodes"`
	} `graphql:"assignees(first: 100)"`
	Milestone *struct {
		Number int
	} `json:"milestone"`
	Labels struct {
		Nodes []struct {
			Id   string
			Name string
		}
	} `graphql:"labels(first: 100)"`
}

type missingGithubIssueRef struct {
	ConnectionId  uint64
	RepoId        int
	GithubId      int
	Number        int
	RawDataOrigin common.RawDataOrigin
}

type missingGithubIssueCleanupScope struct {
	ConnectionId  uint64
	RepoId        int
	GithubId      int
	Number        int
	RawDataOrigin common.RawDataOrigin
}

var CollectIssuesMeta = plugin.SubTaskMeta{
	Name:             "Collect Issues",
	EntryPoint:       CollectIssues,
	EnabledByDefault: true,
	Description:      "Collect Issue data from GithubGraphql api, supports both timeFilter and diffSync.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

var _ plugin.SubTaskEntryPoint = CollectIssues

func CollectIssues(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*githubTasks.GithubTaskData)
	apiCollector, err := api.NewStatefulApiCollector(api.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: githubTasks.GithubApiParams{
			ConnectionId: data.Options.ConnectionId,
			Name:         data.Options.Name,
		},
		Table: RAW_ISSUES_TABLE,
	})
	if err != nil {
		return err
	}

	// collect new issues since the previous run
	since := apiCollector.GetSince()
	err = apiCollector.InitGraphQLCollector(api.GraphqlCollectorArgs{
		GraphqlClient: data.GraphqlClient,
		PageSize:      10,
		BuildQuery: func(reqData *api.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryIssueWrapper{}
			if reqData == nil {
				return query, map[string]interface{}{}, nil
			}
			ownerName := strings.Split(data.Options.Name, "/")
			variables := map[string]interface{}{
				"pageSize":   graphql.Int(reqData.Pager.Size),
				"skipCursor": (*graphql.String)(reqData.Pager.SkipCursor),
				"owner":      graphql.String(ownerName[0]),
				"name":       graphql.String(ownerName[1]),
			}
			return query, variables, nil
		},
		GetPageInfo: func(iQuery interface{}, args *api.GraphqlCollectorArgs) (*api.GraphqlQueryPageInfo, error) {
			query := iQuery.(*GraphqlQueryIssueWrapper)
			return query.Repository.IssueList.PageInfo, nil
		},
		ResponseParser: func(queryWrapper any) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryIssueWrapper)
			issues := query.Repository.IssueList.Issues
			for _, rawL := range issues {
				rawL.ClosedAt = utils.NilIfZeroTime(rawL.ClosedAt)
				if since != nil && since.After(rawL.UpdatedAt) {
					return messages, api.ErrFinishCollect
				}
				messages = append(messages, errors.Must1(json.Marshal(rawL)))
			}
			return
		},
	})
	if err != nil {
		return err
	}

	// refetch(refresh) for existing issues in the database that are still OPEN
	db := taskCtx.GetDal()
	cursor, err := db.Cursor(
		dal.From(models.GithubIssue{}.TableName()),
		dal.Where("state = ? AND repo_id = ? AND connection_id=?", "OPEN", data.Options.GithubId, data.Options.ConnectionId),
	)
	if err != nil {
		return err
	}
	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(models.GithubIssue{}))
	if err != nil {
		return err
	}
	issueUpdatedAt := make(map[int]time.Time)
	requestedIssuesByQuery := sync.Map{}
	err = apiCollector.InitGraphQLCollector(api.GraphqlCollectorArgs{
		GraphqlClient: data.GraphqlClient,
		Input:         iterator,
		InputStep:     100,
		Incremental:   true,
		BuildQuery: func(reqData *api.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryIssueDetailWrapper{}
			if reqData == nil {
				return query, map[string]interface{}{}, nil
			}
			ownerName := strings.Split(data.Options.Name, "/")
			inputIssues := reqData.Input.([]interface{})
			outputIssues := []map[string]interface{}{}
			requestedIssues := make(map[int]missingGithubIssueRef, len(inputIssues))
			for _, i := range inputIssues {
				inputIssue := i.(*models.GithubIssue)
				outputIssues = append(outputIssues, map[string]interface{}{
					`number`: graphql.Int(inputIssue.Number),
				})
				issueUpdatedAt[inputIssue.Number] = inputIssue.GithubUpdatedAt
				requestedIssues[inputIssue.Number] = missingGithubIssueRef{
					ConnectionId:  inputIssue.ConnectionId,
					RepoId:        inputIssue.RepoId,
					GithubId:      inputIssue.GithubId,
					Number:        inputIssue.Number,
					RawDataOrigin: inputIssue.RawDataOrigin,
				}
			}
			requestedIssuesByQuery.Store(query, requestedIssues)
			variables := map[string]interface{}{
				"issue": outputIssues,
				"owner": graphql.String(ownerName[0]),
				"name":  graphql.String(ownerName[1]),
			}
			return query, variables, nil
		},
		ResponseParser: func(queryWrapper any) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryIssueDetailWrapper)
			v, ok := requestedIssuesByQuery.LoadAndDelete(query)
			var requestedIssues map[int]missingGithubIssueRef
			if ok {
				requestedIssues = v.(map[int]missingGithubIssueRef)
			}
			issues := query.Repository.Issues
			for _, rawL := range issues {
				if rawL.DatabaseId == 0 || rawL.Number == 0 {
					continue
				}
				if rawL.UpdatedAt.After(issueUpdatedAt[rawL.Number]) {
					messages = append(messages, errors.Must1(json.Marshal(rawL)))
				}
			}
			missingIssues := findMissingGithubIssues(requestedIssues, issues)
			if len(missingIssues) > 0 {
				err = cleanupMissingGithubIssues(db, taskCtx.GetLogger(), missingIssues)
			}
			return
		},
	})
	if err != nil {
		return err
	}

	return apiCollector.Execute()
}

func findMissingGithubIssues(requestedIssues map[int]missingGithubIssueRef, resolvedIssues []GraphqlQueryIssue) []missingGithubIssueRef {
	if len(requestedIssues) == 0 {
		return nil
	}

	resolvedNumbers := make(map[int]struct{}, len(resolvedIssues))
	for _, issue := range resolvedIssues {
		if issue.DatabaseId == 0 || issue.Number == 0 {
			continue
		}
		resolvedNumbers[issue.Number] = struct{}{}
	}

	missingIssues := make([]missingGithubIssueRef, 0)
	for number, issue := range requestedIssues {
		if _, ok := resolvedNumbers[number]; ok {
			continue
		}
		missingIssues = append(missingIssues, issue)
	}
	sort.Slice(missingIssues, func(i, j int) bool {
		return missingIssues[i].Number < missingIssues[j].Number
	})
	return missingIssues
}

func cleanupMissingGithubIssues(db dal.Dal, logger log.Logger, issues []missingGithubIssueRef) errors.Error {
	var allErrors []error
	for _, issue := range issues {
		scope, ok := buildMissingGithubIssueCleanupScope(issue)
		if !ok {
			logger.Warn(nil, "GitHub issue #%d no longer resolves from the source API, but source scope is incomplete so stale cleanup is skipped", issue.Number)
			continue
		}
		logger.Warn(nil, "GitHub issue #%d no longer resolves from the source API, deleting stale local data for the current repository scope", issue.Number)
		err := cleanupMissingGithubIssue(db, scope)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}
	return errors.Default.Combine(allErrors)
}

func buildMissingGithubIssueCleanupScope(issue missingGithubIssueRef) (*missingGithubIssueCleanupScope, bool) {
	if issue.ConnectionId == 0 || issue.RepoId == 0 || issue.GithubId == 0 || issue.RawDataOrigin.RawDataTable == "" || issue.RawDataOrigin.RawDataParams == "" {
		return nil, false
	}
	return &missingGithubIssueCleanupScope{
		ConnectionId:  issue.ConnectionId,
		RepoId:        issue.RepoId,
		GithubId:      issue.GithubId,
		Number:        issue.Number,
		RawDataOrigin: issue.RawDataOrigin,
	}, true
}

func (scope *missingGithubIssueCleanupScope) issueScopedClauses() []dal.Clause {
	return []dal.Clause{
		dal.Where(
			"connection_id = ? AND issue_id = ? AND _raw_data_table = ? AND _raw_data_params = ?",
			scope.ConnectionId,
			scope.GithubId,
			scope.RawDataOrigin.RawDataTable,
			scope.RawDataOrigin.RawDataParams,
		),
	}
}

func (scope *missingGithubIssueCleanupScope) assigneeScopedClauses() []dal.Clause {
	return []dal.Clause{
		dal.Where(
			"connection_id = ? AND repo_id = ? AND issue_id = ? AND _raw_data_table = ? AND _raw_data_params = ?",
			scope.ConnectionId,
			scope.RepoId,
			scope.GithubId,
			scope.RawDataOrigin.RawDataTable,
			scope.RawDataOrigin.RawDataParams,
		),
	}
}

func (scope *missingGithubIssueCleanupScope) githubIssueScopedClauses() []dal.Clause {
	return []dal.Clause{
		dal.Where(
			"connection_id = ? AND repo_id = ? AND github_id = ? AND _raw_data_table = ? AND _raw_data_params = ?",
			scope.ConnectionId,
			scope.RepoId,
			scope.GithubId,
			scope.RawDataOrigin.RawDataTable,
			scope.RawDataOrigin.RawDataParams,
		),
	}
}

func (scope *missingGithubIssueCleanupScope) rawDataScopedClauses() []dal.Clause {
	if scope.RawDataOrigin.RawDataId == 0 {
		return nil
	}
	return []dal.Clause{dal.Where("id = ?", scope.RawDataOrigin.RawDataId)}
}

func cleanupMissingGithubIssue(db dal.Dal, scope *missingGithubIssueCleanupScope) errors.Error {
	deleteByIssueId := func(model any, table string) errors.Error {
		clauses := append([]dal.Clause{dal.From(table)}, scope.issueScopedClauses()...)
		err := db.Delete(model, clauses...)
		if err != nil {
			return errors.Default.Wrap(err, "failed to delete stale github issue data from "+table)
		}
		return nil
	}

	err := deleteByIssueId(&models.GithubIssueComment{}, models.GithubIssueComment{}.TableName())
	if err != nil {
		return err
	}
	err = deleteByIssueId(&models.GithubIssueEvent{}, models.GithubIssueEvent{}.TableName())
	if err != nil {
		return err
	}
	err = deleteByIssueId(&models.GithubIssueLabel{}, models.GithubIssueLabel{}.TableName())
	if err != nil {
		return err
	}
	err = db.Delete(
		&models.GithubIssueAssignee{},
		append([]dal.Clause{dal.From(models.GithubIssueAssignee{}.TableName())}, scope.assigneeScopedClauses()...)...,
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to delete stale github issue assignees")
	}
	err = db.Delete(
		&models.GithubPrIssue{},
		append([]dal.Clause{dal.From(models.GithubPrIssue{}.TableName())}, scope.issueScopedClauses()...)...,
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to delete stale github pull request issue links")
	}
	err = db.Delete(
		&models.GithubIssue{},
		append([]dal.Clause{dal.From(models.GithubIssue{}.TableName())}, scope.githubIssueScopedClauses()...)...,
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to delete stale github issue")
	}
	if rawDataClauses := scope.rawDataScopedClauses(); len(rawDataClauses) > 0 {
		err = db.Delete(
			&api.RawData{},
			append([]dal.Clause{dal.From(scope.RawDataOrigin.RawDataTable)}, rawDataClauses...)...,
		)
		if err != nil {
			return errors.Default.Wrap(err, "failed to delete stale raw github issue")
		}
	}
	return nil
}
