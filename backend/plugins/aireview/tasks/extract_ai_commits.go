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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
)

var ExtractAiCommitsMeta = plugin.SubTaskMeta{
	Name:             "extractAiCommits",
	EntryPoint:       ExtractAiCommits,
	EnabledByDefault: true,
	Description:      "Classify AI-assisted commits from commit messages and store hits in _tool_aireview_commits",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

type aiCommitScanRow struct {
	Sha           string     `gorm:"column:sha"`
	RepoId        string     `gorm:"column:repo_id"`
	CommitMessage []byte     `gorm:"column:commit_message"`
	GithubMessage []byte     `gorm:"column:github_message"`
	CommitAuthor  string     `gorm:"column:commit_author"`
	GithubAuthor  string     `gorm:"column:github_author"`
	PrcAuthor     string     `gorm:"column:prc_author"`
	CommitDate    *time.Time `gorm:"column:commit_date"`
	GithubDate    *time.Time `gorm:"column:github_date"`
	PrcDate       *time.Time `gorm:"column:prc_date"`
}

// ExtractAiCommits scans commits for the current project/repo and writes
// sparse _tool_aireview_commits rows for AI-positive commits only.
func ExtractAiCommits(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	repoIds, err := resolveAiCommitRepoIds(db, data)
	if err != nil {
		return err
	}
	if len(repoIds) == 0 {
		logger.Info("extractAiCommits: no repos to scan")
		return nil
	}

	if delErr := db.Delete(&models.AiCommit{}, dal.Where("repo_id IN ?", repoIds)); delErr != nil {
		return errors.Default.Wrap(delErr, "failed to delete existing ai commits")
	}

	githubSelect := "NULL AS github_message, NULL AS github_author, NULL AS github_date"
	githubJoin := ""
	if db.HasTable("_tool_github_commits") {
		githubSelect = "tc.message AS github_message, tc.author_name AS github_author, tc.authored_date AS github_date"
		githubJoin = "LEFT JOIN _tool_github_commits tc ON combined.sha = tc.sha"
	}

	clauses := []dal.Clause{
		dal.Select(`combined.sha AS sha, combined.repo_id AS repo_id,
			c.message AS commit_message, ` + githubSelect + `,
			c.author_name AS commit_author,
			prc_data.commit_author_name AS prc_author,
			c.authored_date AS commit_date,
			prc_data.commit_authored_date AS prc_date`),
		dal.From(`(
			SELECT rc.commit_sha AS sha, rc.repo_id AS repo_id
			FROM repo_commits rc
			WHERE rc.repo_id IN ?
			UNION
			SELECT prc.commit_sha, pr.base_repo_id
			FROM pull_request_commits prc
			INNER JOIN pull_requests pr ON prc.pull_request_id = pr.id
			WHERE pr.base_repo_id IN ?
			UNION
			SELECT pr.merge_commit_sha, pr.base_repo_id
			FROM pull_requests pr
			WHERE pr.base_repo_id IN ?
			  AND pr.status = ?
			  AND pr.merge_commit_sha IS NOT NULL
			  AND pr.merge_commit_sha != ?
		) combined`, repoIds, repoIds, repoIds, "MERGED", ""),
		dal.Join("LEFT JOIN commits c ON combined.sha = c.sha"),
		dal.Join(`LEFT JOIN (
			SELECT prc.commit_sha,
				pr.base_repo_id AS repo_id,
				MIN(prc.commit_author_name) AS commit_author_name,
				MIN(prc.commit_authored_date) AS commit_authored_date
			FROM pull_request_commits prc
			INNER JOIN pull_requests pr ON prc.pull_request_id = pr.id
			WHERE pr.base_repo_id IN ?
			GROUP BY prc.commit_sha, pr.base_repo_id
		) prc_data ON combined.sha = prc_data.commit_sha AND combined.repo_id = prc_data.repo_id`, repoIds),
	}
	if githubJoin != "" {
		clauses = append(clauses, dal.Join(githubJoin))
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query commits for AI classification")
	}
	defer cursor.Close()

	batchSize := 100
	batch := make([]*models.AiCommit, 0, batchSize)
	seen := make(map[string]bool)
	found := 0

	for cursor.Next() {
		var row aiCommitScanRow
		if fetchErr := db.Fetch(cursor, &row); fetchErr != nil {
			return errors.Default.Wrap(fetchErr, "failed to fetch commit row")
		}
		if row.Sha == "" || row.RepoId == "" {
			continue
		}

		id := generateAiCommitId(row.RepoId, row.Sha)
		if seen[id] {
			continue
		}

		message := firstNonEmptyBytes(row.CommitMessage, row.GithubMessage)
		author := firstNonEmpty(row.CommitAuthor, row.GithubAuthor, row.PrcAuthor)
		tool, isAI := detectAiCommit(message, author, data.AiCommitPatternsRegex)
		if !isAI {
			continue
		}

		seen[id] = true
		authoredDate := firstNonNilTime(row.CommitDate, row.GithubDate, row.PrcDate)
		batch = append(batch, &models.AiCommit{
			Id:           id,
			CommitSha:    row.Sha,
			RepoId:       row.RepoId,
			AiTool:       tool,
			AuthorName:   author,
			AuthoredDate: authoredDate,
		})
		found++

		if len(batch) >= batchSize {
			if saveErr := saveAiCommitBatch(db, batch); saveErr != nil {
				return saveErr
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if saveErr := saveAiCommitBatch(db, batch); saveErr != nil {
			return saveErr
		}
	}

	logger.Info("Completed AI commit extraction: %d AI-assisted commits found", found)
	return nil
}

func resolveAiCommitRepoIds(db dal.Dal, data *AiReviewTaskData) ([]string, errors.Error) {
	if data.Options.ProjectName != "" {
		var repoIds []string
		err := db.Pluck("row_id", &repoIds,
			dal.From("project_mapping"),
			dal.Where("project_name = ? AND `table` = ?", data.Options.ProjectName, "repos"),
		)
		if err != nil {
			return nil, errors.Default.Wrap(err, "failed to load project repos")
		}
		return repoIds, nil
	}
	if data.Options.RepoId != "" {
		return []string{data.Options.RepoId}, nil
	}
	return nil, nil
}

func generateAiCommitId(repoId, sha string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", repoId, sha)))
	return "aireview-commit:" + hex.EncodeToString(hash[:16])
}

func saveAiCommitBatch(db dal.Dal, batch []*models.AiCommit) errors.Error {
	for _, c := range batch {
		if err := db.CreateOrUpdate(c); err != nil {
			return errors.Default.Wrap(err, "failed to save AI commit")
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmptyBytes(values ...[]byte) string {
	for _, v := range values {
		if len(v) > 0 {
			return string(v)
		}
	}
	return ""
}

func firstNonNilTime(values ...*time.Time) time.Time {
	for _, v := range values {
		if v != nil && !v.IsZero() {
			return *v
		}
	}
	return time.Time{}
}
