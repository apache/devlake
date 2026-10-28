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

package impl

import (
	"fmt"
	"net/url"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/gitextractor/parser"
	"github.com/apache/devlake/plugins/gitextractor/tasks"
	giturls "github.com/chainguard-dev/git-urls"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginTask
	plugin.PluginModel
} = (*GitExtractor)(nil)

type GitExtractor struct{}

type DynamicGitUrl interface {
	GetDynamicGitUrl(taskCtx plugin.TaskContext, connectionId uint64, repoUrl string) (string, errors.Error)
}

func (p GitExtractor) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{}
}

func (p GitExtractor) Description() string {
	return "extract infos from git repository"
}

func (p GitExtractor) Name() string {
	return "gitextractor"
}

// return all available subtasks, framework will run them for you in order
func (p GitExtractor) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CloneGitRepoMeta,
		tasks.CollectGitCommitMeta,
		tasks.CollectGitBranchMeta,
		tasks.CollectGitTagMeta,
		tasks.CollectGitDiffLineMeta,
	}
}

// PrepareTaskData based on task context and user input options, return data that shared among all subtasks
func (p GitExtractor) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	log := taskCtx.GetLogger().Nested("gitextractor.PrepareTaskData")
	var op parser.GitExtractorOptions
	if err := helper.DecodeMapStruct(options, &op, true); err != nil {
		return nil, err
	}

	if op.PluginName != "" {
		pluginInstance, err := plugin.GetPlugin(op.PluginName)
		if err != nil {
			return nil, errors.Default.Wrap(err, fmt.Sprintf("failed to get plugin instance for plugin: %s", op.PluginName))
		}

		if pluginGit, ok := pluginInstance.(DynamicGitUrl); ok {
			gitUrl, err := pluginGit.GetDynamicGitUrl(taskCtx, op.ConnectionId, op.Url)
			if err != nil {
				return nil, errors.Default.Wrap(err, "failed to get Git URL")
			}

			op.Url = gitUrl
		} else {
			log.Printf("Plugin does not implement DynamicGitUrl interface for plugin: %s", op.PluginName)
		}
	}

	parsedURL, err := giturls.Parse(op.Url)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to parse git url")
	}

	// append username to the git url
	if op.User != "" {
		parsedURL.User = url.UserPassword(op.User, op.Password)
		op.Url = parsedURL.String()
	}

	// commit stat, especially commit files(part of stat) are expensive to collect, so we skip them by default
	cfg := taskCtx.GetConfigReader()
	loadBool := func(optValue **bool, cfgKey string, defValue bool) {
		// if user specified the option, use it
		if *optValue != nil {
			return
		}
		// or fallback to .env configuration
		if cfg.IsSet(cfgKey) {
			defValue = cfg.GetBool(cfgKey)
		}
		*optValue = &defValue
	}
	loadBool(&op.UseGoGit, "UseGoGit", false)
	loadBool(&op.SkipCommitStat, "SKIP_COMMIT_STAT", false)
	loadBool(&op.SkipCommitFiles, "SKIP_COMMIT_FILES", true)
	if cfg.GetBool("FORCE_FULL_GIT_HISTORY") {
		op.ForceFullClone = true
	}
	log.Info("UseGoGit: %v", *op.UseGoGit)
	log.Info("SkipCommitStat: %v", *op.SkipCommitStat)
	log.Info("SkipCommitFiles: %v", *op.SkipCommitFiles)
	log.Info("NoShallowClone: %v", op.NoShallowClone)
	log.Info("ForceFullClone: %v", op.ForceFullClone)

	taskData := &parser.GitExtractorTaskData{
		Options:   &op,
		ParsedURL: parsedURL,
	}
	return taskData, nil
}

func (p GitExtractor) Close(taskCtx plugin.TaskContext) errors.Error {
	if taskData, ok := taskCtx.GetData().(*parser.GitExtractorTaskData); ok {
		if !taskCtx.GetConfigReader().GetBool("GIT_EXTRACTOR_KEEP_REPO") {
			if taskData.GitRepo != nil {
				if err := taskData.GitRepo.Close(taskCtx.GetContext()); err != nil {
					return errors.Convert(err)
				}
			}
		}
	}
	return errors.Default.New("task ctx is not GitExtractorTaskData which is unexpected")
}

func (p GitExtractor) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/gitextractor"
}

func (p GitExtractor) TestConnection(id uint64) errors.Error {
	return nil
}
