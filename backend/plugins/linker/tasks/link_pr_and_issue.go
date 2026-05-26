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
	"strings"

	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

var LinkPrToIssueMeta = plugin.SubTaskMeta{
	Name:             "LinkPrToIssue",
	EntryPoint:       LinkPrToIssue,
	EnabledByDefault: true,
	Description:      "Try to link pull requests to issues, according to pull requests' title and description",
	DependencyTables: []string{code.PullRequest{}.TableName(), ticket.Issue{}.TableName()},
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
	ProductTables:    []string{crossdomain.PullRequestIssue{}.TableName()},
}

func normalizeIssueKey(issueKey string) string {
	issueKey = strings.ReplaceAll(issueKey, "#", "")
	issueKey = strings.TrimSpace(issueKey)
	return issueKey
}

func clearHistoryData(db dal.Dal, data *LinkerTaskData) errors.Error {
	sql := `
	DELETE FROM pull_request_issues
		WHERE pull_request_id IN (
			SELECT pr.id
				FROM pull_requests pr
					INNER JOIN project_mapping pm
					ON pm.table = 'repos'
						AND pm.row_id = pr.base_repo_id
				WHERE pm.project_name = ?
		)
		AND issue_id IN (
			SELECT bi.issue_id
				FROM board_issues bi
					INNER JOIN project_mapping pm2
					ON pm2.table = 'boards'
						AND pm2.row_id = bi.board_id
				WHERE pm2.project_name = ?
		)
		AND (_raw_data_table = '' OR _raw_data_table IS NULL)
		AND _raw_data_remark LIKE '%pull_requests,%'
`
	return db.Exec(sql, data.Options.ProjectName, data.Options.ProjectName)
}

func LinkPrToIssue(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinkerTaskData)

	if err := clearHistoryData(db, data); err != nil {
		return err
	}

	var clauses = []dal.Clause{
		dal.From(&code.PullRequest{}),
		dal.Join("LEFT JOIN project_mapping pm ON (pm.table = 'repos' AND pm.row_id = pull_requests.base_repo_id)"),
		dal.Where("pm.project_name = ?", data.Options.ProjectName),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}

	defer cursor.Close()

	var projectIssueIds []string
	if err := db.All(&projectIssueIds,
		dal.From(ticket.BoardIssue{}),
		dal.Select("board_issues.issue_id"),
		dal.Join("LEFT JOIN project_mapping pm ON (pm.table = 'boards' AND pm.row_id = board_issues.board_id)"),
		dal.Where("pm.project_name = ?", data.Options.ProjectName),
	); err != nil {
		return err
	}

	enricher, err := api.NewDataEnricher(api.DataEnricherArgs[code.PullRequest]{
		Ctx:   taskCtx,
		Name:  code.PullRequest{}.TableName(),
		Input: cursor,
		Enrich: func(pullRequest *code.PullRequest) ([]interface{}, errors.Error) {

			var issueKeys []string
			for _, text := range []string{pullRequest.Title, pullRequest.Description, pullRequest.HeadRef} {
				foundIssueKeys := data.PrToIssueRegexp.FindAllString(text, -1)
				if len(foundIssueKeys) > 0 {
					for _, issueKey := range foundIssueKeys {
						issueKey = normalizeIssueKey(issueKey)
						issueKeys = append(issueKeys, issueKey)
					}
					break
				}
			}
			var issues []*ticket.Issue

			var clauses = []dal.Clause{
				dal.From(&ticket.Issue{}),
				dal.Where("issues.id in ? AND issues.issue_key in ?", projectIssueIds, issueKeys),
			}

			if err := db.All(&issues, clauses...); err != nil {
				return nil, err
			}
			if len(issues) == 0 {
				return nil, nil
			}
			var result []interface{}
			for _, issue := range issues {
				pullRequestIssue := &crossdomain.PullRequestIssue{
					PullRequestId:  pullRequest.Id,
					IssueId:        issue.Id,
					PullRequestKey: pullRequest.PullRequestKey,
					IssueKey:       issue.IssueKey,
				}
				result = append(result, pullRequestIssue)
			}

			return result, nil
		},
	})
	if err != nil {
		return err
	}

	return enricher.Execute()
}
