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
	"net/http"
	"net/url"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/circleci/models"
)

// UnfinishedDetailsLookback limits how far back the "unfinished details" collectors look.
// A workflow with an approval job that is never approved stays on_hold forever; without
// a limit it would be collected again on every run (#9304).
const UnfinishedDetailsLookback = 30 * 24 * time.Hour

// UnfinishedDetailsCreatedAfter returns the earliest created date of a workflow that the
// "unfinished details" collectors still collect again.
func UnfinishedDetailsCreatedAfter(now time.Time) time.Time {
	return now.Add(-UnfinishedDetailsLookback)
}

var accountIdGen *didgen.DomainIdGenerator
var projectIdGen *didgen.DomainIdGenerator
var pipelineIdGen *didgen.DomainIdGenerator
var jobIdGen *didgen.DomainIdGenerator

func getAccountIdGen() *didgen.DomainIdGenerator {
	if accountIdGen == nil {
		accountIdGen = didgen.NewDomainIdGenerator(&models.CircleciAccount{})
	}
	return accountIdGen
}

func getProjectIdGen() *didgen.DomainIdGenerator {
	if projectIdGen == nil {
		projectIdGen = didgen.NewDomainIdGenerator(&models.CircleciProject{})
	}
	return projectIdGen
}

func getWorkflowIdGen() *didgen.DomainIdGenerator {
	if pipelineIdGen == nil {
		pipelineIdGen = didgen.NewDomainIdGenerator(&models.CircleciWorkflow{})
	}
	return pipelineIdGen
}

func getJobIdGen() *didgen.DomainIdGenerator {
	if jobIdGen == nil {
		jobIdGen = didgen.NewDomainIdGenerator(&models.CircleciJob{})
	}
	return jobIdGen
}

type CircleciPageTokenResp[T any] struct {
	Items         T      `json:"items"`
	NextPageToken string `json:"next_page_token"`
}

func CreateRawDataSubTaskArgs(taskCtx plugin.SubTaskContext, rawTable string) (*api.RawDataSubTaskArgs, *CircleciTaskData) {
	data := taskCtx.GetData().(*CircleciTaskData)
	filteredData := *data
	filteredData.Options = &CircleciOptions{}
	*filteredData.Options = *data.Options
	params := models.CircleciApiParams{
		ConnectionId: data.Options.ConnectionId,
		ProjectSlug:  data.Options.ProjectSlug,
	}
	rawDataSubTaskArgs := &api.RawDataSubTaskArgs{
		Ctx:    taskCtx,
		Params: params,
		Table:  rawTable,
	}
	return rawDataSubTaskArgs, &filteredData
}

func findPipelineById(db dal.Dal, id string) (*models.CircleciPipeline, errors.Error) {
	if id == "" {
		return nil, errors.Default.New("id must not empty")
	}
	pipeline := &models.CircleciPipeline{}
	if err := db.First(pipeline, dal.Where("id = ?", id)); err != nil {
		return nil, err
	}
	return pipeline, nil
}

func ExtractNextPageToken(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
	res := CircleciPageTokenResp[any]{}
	err := api.UnmarshalResponse(prevPageResponse, &res)
	if err != nil {
		return nil, err
	}
	if res.NextPageToken == "" {
		return nil, api.ErrFinishCollect
	}
	return res.NextPageToken, nil
}

func BuildQueryParamsWithPageToken(reqData *api.RequestData, _ *time.Time) (url.Values, errors.Error) {
	query := url.Values{}
	if pageToken, ok := reqData.CustomData.(string); ok && pageToken != "" {
		query.Set("page-token", pageToken)
	}
	return query, nil
}

func ParseCircleciPageTokenResp(res *http.Response) ([]json.RawMessage, errors.Error) {
	data := CircleciPageTokenResp[[]json.RawMessage]{}
	err := api.UnmarshalResponse(res, &data)
	return data.Items, err
}

// ignoreDeletedOrBrokenBuilds skips per-item API failures that should not
// abort an entire collector subtask. 404 means the resource was deleted
// (retention); 500 means the CircleCI Server record is corrupt/stuck
// (e.g. pipeline exists but its /workflow endpoint errors).
func ignoreDeletedOrBrokenBuilds(res *http.Response) errors.Error {
	switch res.StatusCode {
	case http.StatusNotFound:
		return api.ErrIgnoreAndContinue
	case http.StatusInternalServerError:
		return api.ErrIgnoreAndContinue
	}
	return nil
}

func extractCreatedAt(item json.RawMessage) (time.Time, errors.Error) {
	var entity struct {
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(item, &entity); err != nil {
		return time.Time{}, errors.Default.Wrap(err, "failed to unmarshal item")
	}
	return entity.CreatedAt, nil
}
