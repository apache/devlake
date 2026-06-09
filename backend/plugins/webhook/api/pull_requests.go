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

package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/log"

	"github.com/apache/incubator-devlake/helpers/dbhelper"
	"github.com/go-playground/validator/v10"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/webhook/models"
)

type WebhookPullRequestReq struct {
	Id             string     `mapstructure:"id" validate:"required"`
	BaseRepoId     string     `mapstructure:"baseRepoId"`
	HeadRepoId     string     `mapstructure:"headRepoId"`
	Status         string     `mapstructure:"status" validate:"omitempty,oneof=OPEN CLOSED MERGED"`
	OriginalStatus string     `mapstructure:"originalStatus"`
	Title          string     `mapstructure:"displayTitle" validate:"required"`
	Description    string     `mapstructure:"description"`
	Url            string     `mapstructure:"url"`
	AuthorName     string     `mapstructure:"authorName"`
	AuthorId       string     `mapstructure:"authorId"`
	MergedByName   string     `mapstructure:"mergedByName"`
	MergedById     string     `mapstructure:"mergedById"`
	ParentPrId     string     `mapstructure:"parentPrId"`
	PullRequestKey int        `mapstructure:"pullRequestKey" validate:"required"`
	CreatedDate    time.Time  `mapstructure:"createdDate" validate:"required"`
	MergedDate     *time.Time `mapstructure:"mergedDate"`
	ClosedDate     *time.Time `mapstructure:"closedDate"`
	Type           string     `mapstructure:"type"`
	Component      string     `mapstructure:"component"`
	MergeCommitSha string     `mapstructure:"mergeCommitSha"`
	HeadRef        string     `mapstructure:"headRef"`
	BaseRef        string     `mapstructure:"baseRef"`
	BaseCommitSha  string     `mapstructure:"baseCommitSha"`
	HeadCommitSha  string     `mapstructure:"headCommitSha"`
	Additions      int        `mapstructure:"additions"`
	Deletions      int        `mapstructure:"deletions"`
	IsDraft        bool       `mapstructure:"isDraft"`
}

// PostPullRequests
// @Summary create pull requests by webhook
// @Description Create pull request by webhook.<br/>
// @Description example1: {"id": "pr1","baseRepoId": "webhook:1","headRepoId": "repo_fork1","status": "MERGED","originalStatus": "OPEN","displayTitle": "Feature: Add new functionality","description": "This PR adds new features","url": "https://github.com/org/repo/pull/1","authorName": "johndoe","authorId": "johnd123","mergedByName": "janedoe","mergedById": "janed123","parentPrId": "","pullRequestKey": 1,"createdDate": "2025-02-20T16:17:36Z","mergedDate": "2025-02-20T17:17:36Z","closedDate": null,"type": "feature","component": "backend","mergeCommitSha": "bf0a79c57dff8f5f1f393de315ee5105a535e059","headRef": "repo_fork1:feature-branch","baseRef": "main","baseCommitSha": "e73325c2c9863f42ea25871cbfaeebcb8edcf604","headCommitSha": "b22f772f1197edfafd4cc5fe679a2d299ec12837","additions": 100,"deletions": 50,"isDraft": false}<br/>
// @Description "baseRepoId" must be equal to "webhook:{connectionId}" for this to work correctly and calculate DORA metrics
// @Tags plugins/webhook
// @Param body body WebhookPullRequestReq true "json body"
// @Success 200
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 403  {string} errcode.Error "Forbidden"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/webhook/connections/:connectionId/pull_requests [POST]
func PostPullRequests(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.WebhookConnection{}
	err := connectionHelper.First(connection, input.Params)

	return postPullRequests(input, connection, err)
}

// PostPullRequestsByName
// @Summary create pull requests by webhook name
// @Description Create pull request by webhook name.<br/>
// @Description example1: {"id": "pr1","baseRepoId": "webhook:1","headRepoId": "repo_fork1","status": "MERGED","originalStatus": "OPEN","displayTitle": "Feature: Add new functionality","description": "This PR adds new features","url": "https://github.com/org/repo/pull/1","authorName": "johndoe","authorId": "johnd123","mergedByName": "janedoe","mergedById": "janed123","parentPrId": "","pullRequestKey": 1,"createdDate": "2025-02-20T16:17:36Z","mergedDate": "2025-02-20T17:17:36Z","closedDate": null,"type": "feature","component": "backend","mergeCommitSha": "bf0a79c57dff8f5f1f393de315ee5105a535e059","headRef": "repo_fork1:feature-branch","baseRef": "main","baseCommitSha": "e73325c2c9863f42ea25871cbfaeebcb8edcf604","headCommitSha": "b22f772f1197edfafd4cc5fe679a2d299ec12837","additions": 100,"deletions": 50,"isDraft": false}<br/>
// @Description "baseRepoId" must be equal to "webhook:{connectionId}" for this to work correctly and calculate DORA metrics
// @Tags plugins/webhook
// @Param body body WebhookPullRequestReq true "json body"
// @Success 200
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 403  {string} errcode.Error "Forbidden"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/webhook/connections/by-name/:connectionName/pull_requests [POST]
func PostPullRequestsByName(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.WebhookConnection{}
	err := connectionHelper.FirstByName(connection, input.Params)

	return postPullRequests(input, connection, err)
}

// PostPullRequestsByProjectName
// @Summary create pull requests by project name
// @Description Create pull request by project name. The webhook connection will be created automatically if it does not exist.<br/>
// @Description example1: {"id": "pr1","baseRepoId": "webhook:1","headRepoId": "repo_fork1","status": "MERGED","originalStatus": "OPEN","displayTitle": "Feature: Add new functionality","description": "This PR adds new features","url": "https://github.com/org/repo/pull/1","authorName": "johndoe","authorId": "johnd123","mergedByName": "janedoe","mergedById": "janed123","parentPrId": "","pullRequestKey": 1,"createdDate": "2025-02-20T16:17:36Z","mergedDate": "2025-02-20T17:17:36Z","closedDate": null,"type": "feature","component": "backend","mergeCommitSha": "bf0a79c57dff8f5f1f393de315ee5105a535e059","headRef": "repo_fork1:feature-branch","baseRef": "main","baseCommitSha": "e73325c2c9863f42ea25871cbfaeebcb8edcf604","headCommitSha": "b22f772f1197edfafd4cc5fe679a2d299ec12837","additions": 100,"deletions": 50,"isDraft": false}<br/>
// @Description "baseRepoId" should be equal to "webhook:{connectionId}" for consistent DORA calculations
// @Tags plugins/webhook
// @Param body body WebhookPullRequestReq true "json body"
// @Success 200
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 403  {string} errcode.Error "Forbidden"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /projects/:projectName/pull_requests [POST]
func PostPullRequestsByProjectName(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err, shouldReturn := getOrCreateConnection(input, "pull_requests")
	if shouldReturn {
		return nil, err
	}

	return postPullRequests(input, connection, err)
}

func postPullRequests(input *plugin.ApiResourceInput, connection *models.WebhookConnection, err errors.Error) (*plugin.ApiResourceOutput, errors.Error) {
	if err != nil {
		return nil, err
	}
	// get request
	request := &WebhookPullRequestReq{}
	err = api.DecodeMapStruct(input.Body, request, true)
	if err != nil {
		return &plugin.ApiResourceOutput{Body: err.Error(), Status: http.StatusBadRequest}, nil
	}
	// validate
	vld = validator.New()
	err = errors.Convert(vld.Struct(request))
	if err != nil {
		return nil, errors.BadInput.Wrap(vld.Struct(request), `input json error`)
	}
	txHelper := dbhelper.NewTxHelper(basicRes, &err)
	defer txHelper.End()
	tx := txHelper.Begin()
	if err := CreatePullRequest(connection, request, tx, logger); err != nil {
		logger.Error(err, "create pull requests")
		return nil, err
	}

	return &plugin.ApiResourceOutput{Body: nil, Status: http.StatusOK}, nil
}

func CreatePullRequest(connection *models.WebhookConnection, request *WebhookPullRequestReq, tx dal.Transaction, logger log.Logger) errors.Error {
	// validation
	if request == nil {
		return errors.BadInput.New("request body is nil")
	}
	// create a pull_request record
	pullRequest := &code.PullRequest{
		DomainEntity: domainlayer.DomainEntity{
			Id: fmt.Sprintf("%s:%d:%d", "webhook", connection.ID, request.PullRequestKey),
		},
		BaseRepoId:     fmt.Sprintf("%s:%d", "webhook", connection.ID),
		HeadRepoId:     request.HeadRepoId,
		Status:         request.Status,
		OriginalStatus: request.OriginalStatus,
		Title:          request.Title,
		Description:    request.Description,
		Url:            request.Url,
		AuthorName:     request.AuthorName,
		AuthorId:       request.AuthorId,
		MergedByName:   request.MergedByName,
		MergedById:     request.MergedById,
		ParentPrId:     request.ParentPrId,
		PullRequestKey: request.PullRequestKey,
		CreatedDate:    request.CreatedDate,
		MergedDate:     request.MergedDate,
		ClosedDate:     request.ClosedDate,
		Type:           request.Type,
		Component:      request.Component,
		MergeCommitSha: request.MergeCommitSha,
		HeadRef:        request.HeadRef,
		BaseRef:        request.BaseRef,
		BaseCommitSha:  request.BaseCommitSha,
		HeadCommitSha:  request.HeadCommitSha,
		Additions:      request.Additions,
		Deletions:      request.Deletions,
		IsDraft:        request.IsDraft,
	}
	if err := tx.CreateOrUpdate(pullRequest); err != nil {
		logger.Error(err, "failed to save pull request")
		return err
	}
	return nil
}