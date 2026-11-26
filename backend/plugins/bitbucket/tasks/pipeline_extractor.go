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
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/bitbucket/models"
)

type bitbucketApiCommit struct {
	//Type  string `json:"type"`
	Hash string `json:"hash"`
}

type bitbucketApiPipelineTarget struct {
	//Type     string `json:"type"`
	//RefType  string `json:"ref_type"`
	RefName string `json:"ref_name"`
	//Selector struct {
	//	Type string `json:"type"`
	//} `json:"selector"`
	Commit *bitbucketApiCommit `json:"commit"`
}

type BitbucketApiPipeline struct {
	Uuid string `json:"uuid"`
	//Type  string `json:"type"`
	State struct {
		Name string `json:"name"`
		//	Type   string `json:"type"`
		Result *struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"result"`
		Stage *struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"stage"`
	} `json:"state"`
	Target            *bitbucketApiPipelineTarget `json:"target"`
	CreatedOn         *common.Iso8601Time         `json:"created_on"`
	BuildNumber       int                         `json:"build_number"`
	CompletedOn       *common.Iso8601Time         `json:"completed_on"`
	DurationInSeconds uint64                      `json:"duration_in_seconds"`
	Links             struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
	} `json:"links"`
}

var ExtractApiPipelinesMeta = plugin.SubTaskMeta{
	Name:             "Extract Pipelines",
	EntryPoint:       ExtractApiPipelines,
	EnabledByDefault: true,
	Description:      "Extract raw pipelines data into tool layer table BitbucketPipeline",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

func ExtractApiPipelines(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*BitbucketTaskData)

	// The collector appends a full copy of every pipeline on each run, because
	// Bitbucket ignores the updated_on filter on /pipelines. Extract only the raw
	// rows added since the last run; the deployment/production patterns are part
	// of the config, so changing them triggers a full re-extract.
	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[BitbucketApiPipeline]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_PIPELINE_TABLE,
			Params: BitbucketApiParams{
				ConnectionId: data.Options.ConnectionId,
				FullName:     data.Options.FullName,
			},
			SubtaskConfig: data.RegexEnricher.PlainMap(),
		},
		Extract: func(bitbucketApiPipeline *BitbucketApiPipeline, row *api.RawData) ([]any, errors.Error) {
			bitbucketPipeline := &models.BitbucketPipeline{
				ConnectionId:        data.Options.ConnectionId,
				BitbucketId:         bitbucketApiPipeline.Uuid,
				WebUrl:              bitbucketApiPipeline.Links.Self.Href,
				Status:              bitbucketApiPipeline.State.Name,
				RefName:             bitbucketApiPipeline.Target.RefName,
				CommitSha:           bitbucketApiPipeline.Target.Commit.Hash,
				RepoId:              data.Options.FullName,
				BuildNumber:         bitbucketApiPipeline.BuildNumber,
				DurationInSeconds:   bitbucketApiPipeline.DurationInSeconds,
				BitbucketCreatedOn:  common.Iso8601TimeToTime(bitbucketApiPipeline.CreatedOn),
				BitbucketCompleteOn: common.Iso8601TimeToTime(bitbucketApiPipeline.CompletedOn),
				Type:                data.RegexEnricher.ReturnNameIfMatched(devops.DEPLOYMENT, bitbucketApiPipeline.Target.RefName),
				Environment:         data.RegexEnricher.ReturnNameIfOmittedOrMatched(devops.PRODUCTION, bitbucketApiPipeline.Target.RefName),
			}
			if bitbucketApiPipeline.State.Result != nil {
				bitbucketPipeline.Result = bitbucketApiPipeline.State.Result.Name
			} else if bitbucketApiPipeline.State.Stage != nil {
				bitbucketPipeline.Result = bitbucketApiPipeline.State.Stage.Name
			}

			return []any{bitbucketPipeline}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
