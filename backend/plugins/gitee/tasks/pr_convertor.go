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
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/gitee/models"
	"reflect"
)

var ConvertPullRequestsMeta = plugin.SubTaskMeta{
	Name:             "convertPullRequests",
	EntryPoint:       ConvertPullRequests,
	EnabledByDefault: true,
	Description:      "ConvertPullRequests data from Gitee api",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
}

func ConvertPullRequests(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RAW_PULL_REQUEST_TABLE)
	repoId := data.Repo.GiteeId

	cursor, err := db.Cursor(
		dal.From(&models.GiteePullRequest{}),
		dal.Where("repo_id = ? and connection_id = ?", repoId, data.Options.ConnectionId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	prIdGen := didgen.NewDomainIdGenerator(&models.GiteePullRequest{})
	repoIdGen := didgen.NewDomainIdGenerator(&models.GiteeRepo{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.GiteeAccount{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		InputRowType:       reflect.TypeOf(models.GiteePullRequest{}),
		Input:              cursor,
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			pr := inputRow.(*models.GiteePullRequest)
			domainPr := &code.PullRequest{
				DomainEntity: domainlayer.DomainEntity{
					Id: prIdGen.Generate(data.Options.ConnectionId, pr.GiteeId),
				},
				BaseRepoId:     repoIdGen.Generate(data.Options.ConnectionId, pr.RepoId),
				Status:         convertGiteePrStatus(pr.State),
				OriginalStatus: pr.State,
				Title:          pr.Title,
				Url:            pr.Url,
				AuthorId:       accountIdGen.Generate(data.Options.ConnectionId, pr.AuthorId),
				AuthorName:     pr.AuthorName,
				Description:    pr.Body,
				CreatedDate:    pr.GiteeCreatedAt,
				MergedDate:     pr.MergedAt,
				ClosedDate:     pr.ClosedAt,
				PullRequestKey: pr.Number,
				Type:           pr.Type,
				Component:      pr.Component,
				MergeCommitSha: pr.MergeCommitSha,
				BaseRef:        pr.BaseRef,
				BaseCommitSha:  pr.BaseCommitSha,
				HeadRef:        pr.HeadRef,
				HeadCommitSha:  pr.HeadCommitSha,
			}
			return []interface{}{
				domainPr,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}

// convertGiteePrStatus maps a Gitee pull request state (open, closed, merged)
// to the domain values code.OPEN, code.CLOSED and code.MERGED.
func convertGiteePrStatus(state string) string {
	switch state {
	case "open":
		return code.OPEN
	case "merged":
		return code.MERGED
	case "closed":
		return code.CLOSED
	default:
		return state
	}
}
