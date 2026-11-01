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
	"os"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/gitextractor/parser"
	"github.com/apache/devlake/plugins/gitextractor/store"
)

var CloneGitRepoMeta = plugin.SubTaskMeta{
	Name:             "Clone Git Repo",
	EntryPoint:       CloneGitRepo,
	EnabledByDefault: true,
	Required:         true,
	Description:      "clone a git repo, make it available to later tasks",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	ForceRunOnResume: true,
}

func CloneGitRepo(subTaskCtx plugin.SubTaskContext) errors.Error {
	taskData, ok := subTaskCtx.GetData().(*parser.GitExtractorTaskData)
	if !ok {
		subTaskCtx.GetLogger().Error(nil, "git repo reference not found on context")
		return errors.Default.New("git repo reference not found on context")
	}
	op := taskData.Options
	storage := store.NewDatabase(subTaskCtx, op.RepoId)
	var err errors.Error
	logger := subTaskCtx.GetLogger()

	// temporary dir for cloning
	localDir, e := os.MkdirTemp("", "gitextractor")
	if e != nil {
		return errors.Convert(e)
	}

	// clone repo
	repoCloner, err := parser.NewGitcliCloner(subTaskCtx, localDir)
	if err != nil {
		return err
	}
	err = repoCloner.CloneRepo()
	if err != nil {
		if errors.Is(err, parser.ErrNoData) {
			taskData.SkipAllSubtasks = true
			return nil
		}
		return err
	}
	if repoCloner.IsIncremental() {
		storage.SetIncrementalMode(repoCloner.IsIncremental())
	}
	// We have done comparison experiments for git2go and go-git, and the results show that git2go has better performance.
	var repoCollector parser.RepoCollector
	if taskData.Options.UseGitCli {
		repoCollector, err = parser.NewGitcliRepoCollector(localDir, op.RepoId, storage, logger)
	} else if *taskData.Options.UseGoGit {
		repoCollector, err = parser.NewGogitRepoCollector(localDir, op.RepoId, storage, logger)
	} else {
		repoCollector, err = parser.NewLibgit2RepoCollector(localDir, op.RepoId, storage, logger)
	}
	if err != nil {
		return err
	}

	// inject clean up callback to remove the cloned dir
	cleanup := func() {
		_ = os.RemoveAll(localDir)
		_ = repoCloner.CloseRepo()
	}
	if e := repoCollector.SetCleanUp(cleanup); e != nil {
		return errors.Convert(e)
	}

	// pass the collector down to next subtask
	taskData.GitRepo = repoCollector
	subTaskCtx.TaskContext().SetData(taskData)
	return nil
}
