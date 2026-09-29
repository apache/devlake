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
	"context"
	"reflect"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/refdiff/models"
	"github.com/apache/devlake/plugins/refdiff/utils"
)

var CalculateDeploymentCommitsDiffMeta = plugin.SubTaskMeta{
	Name:             "calculateDeploymentCommitsDiff",
	EntryPoint:       CalculateDeploymentCommitsDiff,
	EnabledByDefault: true,
	Description:      "Calculate commits diff between deployments in the specified project",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

func CalculateDeploymentCommitsDiff(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*RefdiffTaskData)
	db := taskCtx.GetDal()
	ctx := taskCtx.GetContext()
	logger := taskCtx.GetLogger()

	if data.Options.ProjectName == "" {
		return nil
	}

	// step 1. select all deployment commits that need to be calculated
	pairs := make([]*deploymentCommitPair, 0)
	err := db.All(
		&pairs,
		dal.Select("dc.id, dc.commit_sha, p.commit_sha as prev_commit_sha"),
		dal.From("cicd_deployment_commits dc"),
		dal.Join("LEFT JOIN project_mapping pm ON (pm.table = 'cicd_scopes' AND pm.row_id = dc.cicd_scope_id)"),
		dal.Join("LEFT JOIN cicd_deployment_commits p ON (dc.prev_success_deployment_commit_id = p.id)"),
		// p.commit_sha is NULL when there is no previous successful deployment, while the
		// finished pair is stored with an empty old_commit_sha, so compare against ''
		dal.Where(
			`
			pm.project_name = ?
			AND NOT EXISTS (
				SELECT 1
				FROM _tool_refdiff_finished_commits_diffs fcd
				WHERE fcd.new_commit_sha = dc.commit_sha AND fcd.old_commit_sha = COALESCE(p.commit_sha, '')
			)
			`,
			data.Options.ProjectName,
		),
		dal.Orderby(`dc.cicd_scope_id, dc.repo_url, dc.environment, dc.started_date`),
	)
	if err != nil {
		return err
	}
	pairsCount := len(pairs)
	if pairsCount == 0 {
		// graph is expensive, we should avoid creating one for nothing
		return nil
	}

	// step 2. construct a commit node graph and batch save
	graph, err := loadCommitGraph(ctx, db, data)
	if err != nil {
		return err
	}
	batch_save, err := api.NewBatchSave(taskCtx, reflect.TypeOf(&code.CommitsDiff{}), 1000)
	if err != nil {
		return err
	}

	// step 3. iterate all pairs and calculate diff
	taskCtx.SetProgress(0, pairsCount)
	for _, pair := range pairs {
		select {
		case <-ctx.Done():
			return errors.Convert(ctx.Err())
		default:
		}
		lostSha, oldCount, newCount := graph.CalculateLostSha(pair.PrevCommitSha, pair.CommitSha)
		for i, sha := range lostSha {
			commitsDiff := &code.CommitsDiff{
				NewCommitSha: pair.CommitSha,
				OldCommitSha: pair.PrevCommitSha,
				CommitSha:    sha,
				SortingIndex: i + 1,
			}
			err = batch_save.Add(commitsDiff)
			if err != nil {
				return err
			}
		}
		err = batch_save.Flush()
		if err != nil {
			return err
		}
		// mark commits_diff were calculated, no need to do it again in the future
		finishedCommitsDiff := &models.FinishedCommitsDiff{
			NewCommitSha: pair.CommitSha,
			OldCommitSha: pair.PrevCommitSha,
		}
		err = db.CreateOrUpdate(finishedCommitsDiff)
		if err != nil {
			return err
		}

		logger.Info(
			"total %d commits of difference found between [new][%s] and [old][%s(total:%d)]",
			newCount,
			pair.CommitSha,
			pair.PrevCommitSha,
			oldCount,
		)
		taskCtx.IncProgress(1)
	}
	return nil
}

type deploymentCommitPair struct {
	Id            string
	CommitSha     string
	PrevCommitSha string
}

func loadCommitGraph(ctx context.Context, db dal.Dal, data *RefdiffTaskData) (*utils.CommitNodeGraph, errors.Error) {
	graph := utils.NewCommitNodeGraph()

	cursor, err := db.Cursor(
		dal.Select("cp.commit_sha, cp.parent_commit_sha"),
		dal.From("commit_parents cp"),
		dal.Join("LEFT JOIN repo_commits rc ON (rc.commit_sha = cp.commit_sha)"),
		dal.Join("LEFT JOIN project_mapping pm ON (pm.table = 'repos' AND pm.row_id = rc.repo_id)"),
		dal.Where("pm.project_name = ?", data.Options.ProjectName),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()

	for cursor.Next() {
		select {
		case <-ctx.Done():
			return nil, errors.Convert(ctx.Err())
		default:
		}
		commitParent := &code.CommitParent{}
		err = db.Fetch(cursor, commitParent)
		if err != nil {
			return nil, err
		}
		graph.AddParent(commitParent.CommitSha, commitParent.ParentCommitSha)
	}

	return graph, nil
}
