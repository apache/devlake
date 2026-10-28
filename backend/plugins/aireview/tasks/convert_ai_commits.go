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
	domainCode "github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
)

var ConvertAiCommitsMeta = plugin.SubTaskMeta{
	Name:             "convertAiCommits",
	EntryPoint:       ConvertAiCommits,
	EnabledByDefault: true,
	Description:      "Convert tool-layer AI commits into project-scoped domain table ai_commits",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAiCommitsMeta},
}

// ConvertAiCommits reads _tool_aireview_commits scoped to the current project
// and writes project-stamped rows into the ai_commits domain table.
// Only runs in project mode; no-ops silently in single-repo mode.
func ConvertAiCommits(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	projectName := data.Options.ProjectName
	if projectName == "" {
		logger.Info("convertAiCommits: skipping — no projectName set (single-repo mode)")
		return nil
	}

	if err := db.Delete(&domainCode.AiCommit{}, dal.Where("project_name = ?", projectName)); err != nil {
		return errors.Default.Wrap(err, "failed to delete existing ai_commits for project")
	}

	cursor, err := db.Cursor(
		dal.From(&models.AiCommit{}),
		dal.Join("JOIN project_mapping pm ON _tool_aireview_commits.repo_id = pm.row_id AND pm.`table` = 'repos'"),
		dal.Where("pm.project_name = ?", projectName),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to cursor ai commits")
	}
	defer cursor.Close()

	batch := make([]*domainCode.AiCommit, 0, 100)
	for cursor.Next() {
		var src models.AiCommit
		if fetchErr := db.Fetch(cursor, &src); fetchErr != nil {
			return errors.Default.Wrap(fetchErr, "failed to fetch ai commit row")
		}

		batch = append(batch, &domainCode.AiCommit{
			DomainEntity: domainlayer.DomainEntity{
				Id: generateAiDomainId("ac", projectName, src.Id),
			},
			ProjectName:  projectName,
			CommitSha:    src.CommitSha,
			RepoId:       src.RepoId,
			AiTool:       src.AiTool,
			AuthorName:   src.AuthorName,
			AuthoredDate: src.AuthoredDate,
		})

		if len(batch) >= 100 {
			if saveErr := saveDomainAiCommitBatch(db, batch); saveErr != nil {
				return saveErr
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if saveErr := saveDomainAiCommitBatch(db, batch); saveErr != nil {
			return saveErr
		}
	}

	logger.Info("convertAiCommits: done for project %s", projectName)
	return nil
}

func saveDomainAiCommitBatch(db dal.Dal, batch []*domainCode.AiCommit) errors.Error {
	for _, c := range batch {
		if err := db.CreateOrUpdate(c); err != nil {
			return errors.Default.Wrap(err, "failed to save domain ai commit")
		}
	}
	return nil
}
