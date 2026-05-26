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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/plugins/gitlab/models"

	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

const (
	// https://docs.gitlab.com/ee/api/pipelines.html#list-project-pipelines
	// https://docs.gitlab.com/ee/api/deployments.html#list-project-deployments
	// https://docs.gitlab.com/ee/api/jobs.html#list-project-jobs
	StatusSuccess            = "success"
	StatusFailed             = "failed"
	StatusCanceled           = "canceled"
	StatusRunning            = "running"
	StatusWaitingForResource = "waiting_for_resource"
	StatusPending            = "pending"
	StatusPreparing          = "preparing"
	StatusCreated            = "created"
	StatusManual             = "manual"
	StatusScheduled          = "scheduled"
	StatusSkipped            = "skipped"
	StatusBlocked            = "blocked"
	// the following two status are handle in codes, but cannot be seen in documents.
	StatusCompleted  = "COMPLETED"
	StatusUndeployed = "UNDEPLOYED"
)

type GitlabInput struct {
	GitlabId int
	Iid      int
}

func GetTotalPagesFromResponse(res *http.Response, args *api.ApiCollectorArgs) (int, errors.Error) {
	total := res.Header.Get("X-Total-Pages")
	if total == "" {
		return 0, api.ErrUndetermined
	}
	totalInt, err := strconv.Atoi(total)
	if err != nil {
		return 0, errors.Convert(err)
	}
	return totalInt, nil
}

func GetRawMessageFromResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	rawMessages := []json.RawMessage{}

	if res == nil {
		return nil, errors.Default.New("res is nil")
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error reading response from %s", res.Request.URL.String()))
	}

	err = errors.Convert(json.Unmarshal(resBody, &rawMessages))
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error decoding response from %s. raw response was: %s", res.Request.URL.String(), string(resBody)))
	}

	return rawMessages, nil
}

func GetOneRawMessageFromResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	rawMessage := json.RawMessage{}

	if res == nil {
		return nil, errors.Default.New("res is nil")
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error reading response from %s", res.Request.URL.String()))
	}

	err = errors.Convert(json.Unmarshal(resBody, &rawMessage))
	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("error decoding response from %s. raw response was: %s", res.Request.URL.String(), string(resBody)))
	}

	return []json.RawMessage{rawMessage}, nil
}

func GetRawMessageUpdatedAtAfter(timeAfter *time.Time) func(res *http.Response) ([]json.RawMessage, errors.Error) {
	type ApiModel struct {
		UpdatedAt *common.Iso8601Time `json:"updated_at"`
	}

	return func(res *http.Response) ([]json.RawMessage, errors.Error) {
		rawMessages, err := GetRawMessageFromResponse(res)
		if err != nil {
			return nil, errors.Default.Wrap(err, fmt.Sprintf("error reading response from %s", res.Request.URL.String()))
		}
		isFinish := true
		filterRawMessages := []json.RawMessage{}
		for _, rawMessage := range rawMessages {
			apiModel := &ApiModel{}
			err = errors.Convert(json.Unmarshal(rawMessage, apiModel))
			if err != nil {
				return nil, err
			}
			if timeAfter == nil || timeAfter.Before(apiModel.UpdatedAt.ToTime()) {
				// only finish when all items are created before `createDateAfter`
				// because gitlab's order may not strict enough
				isFinish = false
				filterRawMessages = append(filterRawMessages, rawMessage)
			}
		}
		if isFinish {
			return filterRawMessages, api.ErrFinishCollect
		}
		return filterRawMessages, nil
	}
}

func GetQuery(reqData *api.RequestData) (url.Values, errors.Error) {
	query := url.Values{}
	query.Set("with_stats", "true")
	query.Set("sort", "asc")
	query.Set("page", strconv.Itoa(reqData.Pager.Page))
	query.Set("per_page", strconv.Itoa(reqData.Pager.Size))
	return query, nil
}

func CreateRawDataSubTaskArgs(subtaskCtx plugin.SubTaskContext, Table string) (*api.RawDataSubTaskArgs, *GitlabTaskData) {
	data := subtaskCtx.GetData().(*GitlabTaskData)
	rawDataSubTaskArgs := &api.RawDataSubTaskArgs{
		Ctx: subtaskCtx,
		Params: models.GitlabApiParams{
			ProjectId:    data.Options.ProjectId,
			ConnectionId: data.Options.ConnectionId,
		},
		Table: Table,
	}
	return rawDataSubTaskArgs, data
}

func CreateSubtaskCommonArgs(subtaskCtx plugin.SubTaskContext, table string) (*api.SubtaskCommonArgs, *GitlabTaskData) {
	data := subtaskCtx.GetData().(*GitlabTaskData)
	args := &api.SubtaskCommonArgs{
		SubTaskContext: subtaskCtx,
		Table:          table,
		Params: models.GitlabApiParams{
			ConnectionId: data.Options.ConnectionId,
			ProjectId:    data.Options.ProjectId,
		},
	}
	return args, data
}

func GetMergeRequestsIterator(taskCtx plugin.SubTaskContext, apiCollector *api.StatefulApiCollector) (*api.DalCursorIterator, errors.Error) {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GitlabTaskData)
	clauses := []dal.Clause{
		dal.Select("gmr.gitlab_id, gmr.iid"),
		dal.From("_tool_gitlab_merge_requests gmr"),
		// collect only opening merge request's notes and commits to speed up the process
		dal.Where(
			`gmr.project_id = ? and gmr.connection_id = ?`,
			data.Options.ProjectId, data.Options.ConnectionId,
		),
	}
	if apiCollector != nil {
		if apiCollector.GetSince() != nil {
			clauses = append(clauses, dal.Where("gitlab_updated_at > ?", *apiCollector.GetSince()))
		}
	}
	// construct the input iterator
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return nil, err
	}

	return api.NewDalCursorIterator(db, cursor, reflect.TypeOf(GitlabInput{}))
}
