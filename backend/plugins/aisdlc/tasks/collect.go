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
	"fmt"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aisdlc/models"
	"github.com/apache/devlake/plugins/aisdlc/tasks/parse"
)

var CollectFilesMeta = plugin.SubTaskMeta{
	Name:             "collectFiles",
	EntryPoint:       CollectFiles,
	EnabledByDefault: true,
	Description:      "Read station artifacts from a GitLab project",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

// CollectFiles lists and stores artifact blobs. The checkpoint is separate from observations.
func CollectFiles(taskCtx plugin.SubTaskContext) errors.Error {
	ctx := taskCtx.GetContext()
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AisdlcTaskData)
	cfg := data.Options.ScopeConfig
	projectPath := data.Options.FullName

	glConn, err := LoadGitlabConnection(db, data.Connection.GitlabConnectionId)
	if err != nil {
		return err
	}
	client, clientErr := NewGitlabClient(ctx, glConn.Endpoint, glConn.Token)
	if clientErr != nil {
		return errors.Default.Wrap(clientErr, "gitlab client")
	}
	branch := cfg.Branch
	if branch == "" {
		project, projectErr := client.GetProject(ctx, projectPath)
		if projectErr != nil {
			return errors.Default.Wrap(projectErr, "gitlab project")
		}
		branch = project.DefaultBranch
	}
	if branch == "" {
		branch = "main"
	}
	commit, commitErr := client.BranchCommit(ctx, projectPath, branch)
	if commitErr != nil {
		return errors.Default.Wrap(commitErr, "gitlab branch commit")
	}
	// List and download the same commit. A branch name can move between the two calls.
	entries, listErr := client.ListBlobs(ctx, projectPath, commit, cfg.PathPrefix, cfg.Parser)
	if listErr != nil {
		_ = saveCheckpoint(db, data.Connection.ID, projectPath, commit, branch, 0, listErr.Error())
		return errors.Default.Wrap(listErr, "gitlab tree")
	}

	collectedAt := time.Now().UTC()
	var failures []string
	for _, entry := range entries {
		rawID := parse.ID("raw", fmt.Sprintf("%d", data.Connection.ID), projectPath, entry.Path, entry.ID)
		existing := &models.AisdlcRawFile{}
		findErr := db.First(existing, dal.Where("id = ? AND connection_id = ?", rawID, data.Connection.ID))
		if findErr == nil && existing.BlobId == entry.ID && existing.Body != "" {
			existing.CommitSha = commit
			existing.CollectedAt = collectedAt
			existing.RawDataParams = scopeRawParams(data.Connection.ID, projectPath)
			if updateErr := db.Update(existing); updateErr != nil {
				return updateErr
			}
			continue
		}
		body, rawErr := client.GetRaw(ctx, projectPath, commit, entry.Path)
		if rawErr != nil {
			logger.Warn(rawErr, "failed to download %s", entry.Path)
			failures = append(failures, entry.Path+": "+rawErr.Error())
			continue
		}
		row := &models.AisdlcRawFile{
			NoPKModel:    scopeStamp(data.Connection.ID, projectPath),
			Id:           rawID,
			ConnectionId: data.Connection.ID,
			ProjectPath:  projectPath,
			FilePath:     entry.Path,
			CommitSha:    commit,
			BlobId:       entry.ID,
			Body:         body,
			CollectedAt:  collectedAt,
		}
		if saveErr := db.CreateOrUpdate(row); saveErr != nil {
			return saveErr
		}
	}
	message := strings.Join(failures, "\n")
	if saveErr := saveCheckpoint(db, data.Connection.ID, projectPath, commit, branch, len(entries), message); saveErr != nil {
		return saveErr
	}
	if len(failures) > 0 {
		return errors.Default.New(fmt.Sprintf("%d download failure(s): %s", len(failures), message))
	}
	return nil
}

// LoadGitlabConnection reads token and endpoint from the GitLab plugin's connection table.
func LoadGitlabConnection(db dal.Dal, id uint64) (*GitlabConnLookup, errors.Error) {
	var glConn GitlabConnLookup
	if err := db.First(&glConn, dal.Where("id = ?", id)); err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("gitlab connection %d not found", id))
	}
	if glConn.Token == "" {
		return nil, errors.BadInput.New("referenced gitlab connection has no token")
	}
	if glConn.Endpoint == "" {
		glConn.Endpoint = "https://gitlab.com/api/v4"
	}
	return &glConn, nil
}

func saveCheckpoint(db dal.Dal, connectionID uint64, projectPath, commit, branch string, fileCount int, message string) errors.Error {
	row := &models.AisdlcCheckpoint{
		NoPKModel:        scopeStamp(connectionID, projectPath),
		Id:               checkpointID(connectionID, projectPath),
		ConnectionId:     connectionID,
		ProjectPath:      projectPath,
		ExaminedRevision: commit,
		Branch:           branch,
		FileCount:        fileCount,
		ErrorMessage:     message,
		CollectedAt:      time.Now().UTC(),
	}
	return db.CreateOrUpdate(row)
}

func checkpointID(connectionID uint64, projectPath string) string {
	return parse.ID("checkpoint", fmt.Sprintf("%d", connectionID), projectPath)
}

// appendCheckpointError records a later failure without clearing the revision collect stored.
func appendCheckpointError(db dal.Dal, connectionID uint64, projectPath, message string) errors.Error {
	row := &models.AisdlcCheckpoint{}
	err := db.First(row, dal.Where("id = ? AND connection_id = ?", checkpointID(connectionID, projectPath), connectionID))
	if err != nil {
		if db.IsErrorNotFound(err) {
			return nil
		}
		return err
	}
	if row.ErrorMessage != "" {
		row.ErrorMessage += "\n"
	}
	row.ErrorMessage += message
	row.RawDataParams = scopeRawParams(connectionID, projectPath)
	return db.Update(row)
}
