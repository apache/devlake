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
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/utils"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	githubModels "github.com/apache/devlake/plugins/github/models"
	githubTasks "github.com/apache/devlake/plugins/github/tasks"
)

var _ plugin.SubTaskEntryPoint = ExtractReleases

var ExtractReleasesMeta = plugin.SubTaskMeta{
	Name:             "Extract Releases",
	EntryPoint:       ExtractReleases,
	EnabledByDefault: true,
	Description:      "extract raw release data into tool layer table github_releases",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

func ExtractReleases(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*githubTasks.GithubTaskData)
	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[GraphqlQueryRelease]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: githubTasks.GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_RELEASE_TABLE,
		},
		Extract: func(release *GraphqlQueryRelease, row *api.RawData) ([]interface{}, errors.Error) {
			release.PublishedAt = utils.NilIfZeroTime(release.PublishedAt)
			var results []interface{}
			githubRelease, err := convertGitHubRelease(release, data.Options.ConnectionId, data.Options.GithubId)
			if err != nil {
				return nil, errors.Convert(err)
			}
			results = append(results, githubRelease)

			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}

func convertGitHubRelease(release *GraphqlQueryRelease, connectionId uint64, githubId int) (*githubModels.GithubRelease, errors.Error) {
	ret := &githubModels.GithubRelease{
		ConnectionId: connectionId,
		GithubId:     githubId,
		NoPKModel:    common.NewNoPKModel(),

		Id:              release.Id,
		AuthorID:        release.Author.ID,
		CreatedAt:       release.CreatedAt,
		DatabaseID:      release.DatabaseID,
		Description:     release.Description,
		DescriptionHTML: release.Description,
		IsDraft:         release.IsDraft,
		IsLatest:        release.IsLatest,
		IsPrerelease:    release.IsPrerelease,
		Name:            release.Name,
		PublishedAt:     release.PublishedAt,
		ResourcePath:    release.ResourcePath,
		TagName:         release.TagName,
		UpdatedAt:       release.UpdatedAt,
		URL:             release.URL,
		CommitSha:       release.TagCommit.Oid,
	}
	if release.Author.Name != nil {
		ret.AuthorName = *release.Author.Name
	}
	return ret, nil
}
