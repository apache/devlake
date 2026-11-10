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
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/utils"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/github/models"
	githubTasks "github.com/apache/devlake/plugins/github/tasks"
)

var _ plugin.SubTaskEntryPoint = ExtractAccounts

var ExtractIssuesMeta = plugin.SubTaskMeta{
	Name:             "Extract Issues",
	EntryPoint:       ExtractIssues,
	EnabledByDefault: true,
	Description:      "Extract raw Issues data into tool layer table github_issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ExtractIssues(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*githubTasks.GithubTaskData)
	config := data.Options.ScopeConfig
	issueRegexes, err := githubTasks.NewIssueRegexes(config)
	if err != nil {
		return err
	}
	milestoneMap, err := getMilestoneMap(db, data.Options.GithubId, data.Options.ConnectionId)
	if err != nil {
		return err
	}

	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: githubTasks.GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_ISSUES_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			issue := &GraphqlQueryIssue{}
			err := errors.Convert(json.Unmarshal(row.Data, issue))
			if err != nil {
				return nil, err
			}
			// Normalize zero-date to nil for closedAt
			issue.ClosedAt = utils.NilIfZeroTime(issue.ClosedAt)
			results := make([]interface{}, 0, 1)
			githubIssue, err := convertGithubIssue(milestoneMap, issue, data.Options.ConnectionId, data.Options.GithubId)
			if err != nil {
				return nil, err
			}
			githubLabels, err := convertGithubLabels(issueRegexes, issue, githubIssue)
			if err != nil {
				return nil, err
			}
			results = append(results, githubLabels...)
			results = append(results, githubIssue)
			if len(issue.AssigneeList.Assignees) > 0 {
				extractGraphqlPreAccount(&results, issue.AssigneeList.Assignees[0].Account(), data.Options.GithubId, data.Options.ConnectionId)
			}
			extractGraphqlPreAccount(&results, issue.Author.Account(), data.Options.GithubId, data.Options.ConnectionId)
			for _, assignee := range issue.AssigneeList.Assignees {
				issueAssignee := &models.GithubIssueAssignee{
					ConnectionId: githubIssue.ConnectionId,
					IssueId:      githubIssue.GithubId,
					RepoId:       githubIssue.RepoId,
					AssigneeId:   assignee.Id,
					AssigneeName: assignee.Login,
				}
				results = append(results, issueAssignee)
			}
			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}

// create a milestone map for numberId to databaseId
func getMilestoneMap(db dal.Dal, repoId int, connectionId uint64) (map[int]int, errors.Error) {
	milestoneMap := map[int]int{}
	var milestones []struct {
		MilestoneId int
		RepoId      int
		Number      int
	}
	err := db.All(
		&milestones,
		dal.From(&models.GithubMilestone{}),
		dal.Where("repo_id = ? and connection_id = ?", repoId, connectionId),
	)
	if err != nil {
		return nil, err
	}
	for _, milestone := range milestones {
		milestoneMap[milestone.Number] = milestone.MilestoneId
	}
	return milestoneMap, nil
}

func convertGithubIssue(milestoneMap map[int]int, issue *GraphqlQueryIssue, connectionId uint64, repositoryId int) (*models.GithubIssue, errors.Error) {
	githubIssue := &models.GithubIssue{
		ConnectionId:    connectionId,
		GithubId:        issue.DatabaseId,
		RepoId:          repositoryId,
		Number:          issue.Number,
		State:           issue.State,
		Title:           issue.Title,
		Body:            strings.ReplaceAll(issue.Body, "\x00", `<0x00>`),
		Url:             issue.Url,
		ClosedAt:        issue.ClosedAt,
		GithubCreatedAt: issue.CreatedAt,
		GithubUpdatedAt: issue.UpdatedAt,
	}
	if len(issue.AssigneeList.Assignees) > 0 {
		githubIssue.AssigneeId = issue.AssigneeList.Assignees[0].Id
		githubIssue.AssigneeName = issue.AssigneeList.Assignees[0].Login
	}
	if author := issue.Author.Account(); author.Id != 0 {
		githubIssue.AuthorId = author.Id
		githubIssue.AuthorName = author.Login
	}
	if issue.ClosedAt != nil {
		githubIssue.LeadTimeMinutes = githubTasks.IssueLeadTimeMinutes(issue.CreatedAt, *issue.ClosedAt)
	}
	if issue.Milestone != nil {
		if milestoneId, ok := milestoneMap[issue.Milestone.Number]; ok {
			githubIssue.MilestoneId = milestoneId
		}
	}
	return githubIssue, nil
}

func convertGithubLabels(issueRegexes *githubTasks.IssueRegexes, issue *GraphqlQueryIssue, githubIssue *models.GithubIssue) ([]interface{}, errors.Error) {
	var results []interface{}
	var joinedLabels []string
	for _, label := range issue.Labels.Nodes {
		results = append(results, &models.GithubIssueLabel{
			ConnectionId: githubIssue.ConnectionId,
			IssueId:      githubIssue.GithubId,
			LabelName:    label.Name,
		})

		if issueRegexes.SeverityRegex != nil && issueRegexes.SeverityRegex.MatchString(label.Name) {
			githubIssue.Severity = label.Name
		}
		if issueRegexes.ComponentRegex != nil && issueRegexes.ComponentRegex.MatchString(label.Name) {
			githubIssue.Component = label.Name
		}
		if issueRegexes.PriorityRegex != nil && issueRegexes.PriorityRegex.MatchString(label.Name) {
			githubIssue.Priority = label.Name
		}
		if issueRegexes.TypeRequirementRegex != nil && issueRegexes.TypeRequirementRegex.MatchString(label.Name) {
			githubIssue.StdType = ticket.REQUIREMENT
		} else if issueRegexes.TypeBugRegex != nil && issueRegexes.TypeBugRegex.MatchString(label.Name) {
			githubIssue.StdType = ticket.BUG
		} else if issueRegexes.TypeIncidentRegex != nil && issueRegexes.TypeIncidentRegex.MatchString(label.Name) {
			githubIssue.StdType = ticket.INCIDENT
		}
		joinedLabels = append(joinedLabels, label.Name)
	}
	if len(joinedLabels) > 0 {
		githubIssue.Type = strings.Join(joinedLabels, ",")
	}
	return results, nil
}
