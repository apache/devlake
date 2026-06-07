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
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/server/services"

	"github.com/apache/incubator-devlake/helpers/dbhelper"
	"github.com/go-playground/validator/v10"

	"github.com/apache/incubator-devlake/core/errors"
	coremodels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/webhook/models"
)

type WebhookDeploymentReq struct {
	Id                  string `mapstructure:"id" validate:"required"`
	DisplayTitle        string `mapstructure:"displayTitle"`
	Result              string `mapstructure:"result"`
	Environment         string `validate:"omitempty,oneof=PRODUCTION STAGING TESTING DEVELOPMENT"`
	OriginalEnvironment string `mapstructure:"originalEnvironment"`
	Name                string `mapstructure:"name"`
	// DeploymentCommits is used for multiple commits in one deployment
	DeploymentCommits []WebhookDeploymentCommitReq `mapstructure:"deploymentCommits" validate:"omitempty,dive"`
	CreatedDate       *time.Time                   `mapstructure:"createdDate"`
	// QueuedDate   *time.Time `mapstructure:"queue_time"`
	StartedDate  *time.Time `mapstructure:"startedDate" validate:"required"`
	FinishedDate *time.Time `mapstructure:"finishedDate" validate:"required"`
}

type WebhookDeploymentCommitReq struct {
	DisplayTitle string     `mapstructure:"displayTitle"`
	RepoId       string     `mapstructure:"repoId"`
	RepoUrl      string     `mapstructure:"repoUrl" validate:"required"`
	Name         string     `mapstructure:"name"`
	RefName      string     `mapstructure:"refName"`
	CommitSha    string     `mapstructure:"commitSha" validate:"required"`
	CommitMsg    string     `mapstructure:"commitMsg"`
	Result       string     `mapstructure:"result"`
	Status       string     `mapstructure:"status"`
	CreatedDate  *time.Time `mapstructure:"createdDate"`
	// QueuedDate   *time.Time `mapstructure:"queue_time"`
	StartedDate  *time.Time `mapstructure:"startedDate" validate:"required"`
	FinishedDate *time.Time `mapstructure:"finishedDate" validate:"required"`
}

// PostDeployments
// @Summary create deployment by webhook
// @Description Create deployment pipeline by webhook.<br/>
// @Description example1: {"repo_url":"devlake","commit_sha":"015e3d3b480e417aede5a1293bd61de9b0fd051d","start_time":"2020-01-01T12:00:00+00:00","end_time":"2020-01-01T12:59:59+00:00","environment":"PRODUCTION"}<br/>
// @Description So we suggest request before task after deployment pipeline finish.
// @Description Both cicd_pipeline and cicd_task will be created
// @Tags plugins/webhook
// @Param body body WebhookDeploymentReq true "json body"
// @Success 200
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 403  {string} errcode.Error "Forbidden"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/webhook/connections/:connectionId/deployments [POST]
func PostDeployments(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.WebhookConnection{}
	err := connectionHelper.First(connection, input.Params)

	return postDeployments(input, connection, err)
}

// PostDeploymentsByName
// @Summary create deployment by webhook name
// @Description Create deployment pipeline by webhook name.<br/>
// @Description example1: {"repo_url":"devlake","commit_sha":"015e3d3b480e417aede5a1293bd61de9b0fd051d","start_time":"2020-01-01T12:00:00+00:00","end_time":"2020-01-01T12:59:59+00:00","environment":"PRODUCTION"}<br/>
// @Description So we suggest request before task after deployment pipeline finish.
// @Description Both cicd_pipeline and cicd_task will be created
// @Tags plugins/webhook
// @Param body body WebhookDeploymentReq true "json body"
// @Success 200
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 403  {string} errcode.Error "Forbidden"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /plugins/webhook/connections/by-name/:connectionName/deployments [POST]
func PostDeploymentsByName(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.WebhookConnection{}
	err := connectionHelper.FirstByName(connection, input.Params)

	return postDeployments(input, connection, err)
}

// PostDeploymentsByProjectName
// @Summary create deployment by project name
// @Description Create deployment pipeline by project name.<br/>
// @Description example1: {"repo_url":"devlake","commit_sha":"015e3d3b480e417aede5a1293bd61de9b0fd051d","start_time":"2020-01-01T12:00:00+00:00","end_time":"2020-01-01T12:59:59+00:00","environment":"PRODUCTION"}<br/>
// @Description So we suggest request before task after deployment pipeline finish.
// @Description Both cicd_pipeline and cicd_task will be created
// @Tags plugins/webhook
// @Param body body WebhookDeploymentReq true "json body"
// @Success 200
// @Failure 400  {string} errcode.Error "Bad Request"
// @Failure 403  {string} errcode.Error "Forbidden"
// @Failure 500  {string} errcode.Error "Internal Error"
// @Router /projects/:projectName/deployments [POST]
func PostDeploymentsByProjectName(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	// find or create the connection for this project
	connection, err, shouldReturn := getOrCreateConnection(input, "deployments")
	if shouldReturn {
		return nil, err
	}

	return postDeployments(input, connection, err)
}

func getOrCreateConnection(input *plugin.ApiResourceInput, webhookSuffix string) (*models.WebhookConnection, errors.Error, bool) {
	connection := &models.WebhookConnection{}
	projectName := input.Params["projectName"]
	webhookName := fmt.Sprintf("%s_%s", projectName, webhookSuffix)
	err := findByProjectName(connection, input.Params, pluginName, webhookName)
	dal := basicRes.GetDal()
	if err != nil {
		// if not found, we will attempt to create a new connection
		// Use direct comparison against the package sentinel; only treat other errors as fatal.
		if !dal.IsErrorNotFound(err) {
			logger.Error(err, "failed to find webhook connection for project", "projectName", projectName)
			return nil, err, true
		}

		// create the connection
		logger.Debug("creating webhook connection for project %s", projectName)
		connection.Name = webhookName

		// find the project and blueprint with which we will associate this connection
		projectOutput, err := services.GetProject(projectName)
		if err != nil {
			logger.Error(err, "failed to find project for webhook connection", "projectName", projectName)
			return nil, err, true
		}

		if projectOutput == nil {
			logger.Error(err, "project not found for webhook connection", "projectName", projectName)
			return nil, errors.NotFound.New("project not found: " + projectName), true
		}

		if projectOutput.Blueprint == nil {
			logger.Error(err, "unable to create webhook as the project has no blueprint", "projectName", projectName)
			return nil, errors.BadInput.New("project has no blueprint: " + projectName), true
		}

		connectionInput := &plugin.ApiResourceInput{
			Params: map[string]string{
				"plugin": "webhook",
			},
			Body: map[string]interface{}{
				"name": webhookName,
			},
		}

		err = connectionHelper.Create(connection, connectionInput)
		if err != nil {
			logger.Error(err, "failed to create webhook connection for project", "projectName", projectName)
			return nil, err, true
		}

		// get the blueprint
		blueprintId := projectOutput.Blueprint.ID
		blueprint, err := services.GetBlueprint(blueprintId, true)

		if err != nil {
			logger.Error(err, "failed to find blueprint for webhook connection", "blueprintId", blueprintId)
			return nil, err, true
		}

		// we need to associate this connection with the blueprint
		blueprintConnection := &coremodels.BlueprintConnection{
			BlueprintId:  blueprint.ID,
			PluginName:   pluginName,
			ConnectionId: connection.ID,
		}

		logger.Info("adding blueprint connection for blueprint %d and connection %d", blueprint.ID, connection.ID)
		err = dal.Create(blueprintConnection)
		if err != nil {
			logger.Error(err, "failed to create blueprint connection for project", "projectName", projectName)
			return nil, err, true
		}
	}
	return connection, err, false
}

func postDeployments(input *plugin.ApiResourceInput, connection *models.WebhookConnection, err errors.Error) (*plugin.ApiResourceOutput, errors.Error) {
	if err != nil {
		return nil, err
	}
	// get request
	request := &WebhookDeploymentReq{}
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
	if err := CreateDeploymentAndDeploymentCommits(connection, request, tx, logger); err != nil {
		logger.Error(err, "create deployments")
		return nil, err
	}

	return &plugin.ApiResourceOutput{Body: nil, Status: http.StatusOK}, nil
}

func CreateDeploymentAndDeploymentCommits(connection *models.WebhookConnection, request *WebhookDeploymentReq, tx dal.Transaction, logger log.Logger) errors.Error {
	// validation
	if request == nil {
		return errors.BadInput.New("request body is nil")
	}
	if len(request.DeploymentCommits) == 0 {
		return errors.BadInput.New("deployment_commits is empty")
	}
	// set default values for optional fields
	deploymentId := request.Id
	scopeId := fmt.Sprintf("%s:%d", "webhook", connection.ID)
	if request.CreatedDate == nil {
		request.CreatedDate = request.StartedDate
	}
	if request.FinishedDate == nil {
		now := time.Now()
		request.FinishedDate = &now
	}
	if request.Result == "" {
		request.Result = devops.RESULT_SUCCESS
	}
	if request.Environment == "" {
		request.Environment = devops.PRODUCTION
	}
	duration := float64(request.FinishedDate.Sub(*request.StartedDate).Milliseconds() / 1e3)
	name := request.Name
	if name == "" {
		var commitShaList []string
		for _, commit := range request.DeploymentCommits {
			commitShaList = append(commitShaList, commit.CommitSha)
		}
		name = fmt.Sprintf(`deploy %s to %s`, strings.Join(commitShaList, ","), request.Environment)
	}
	createdDate := time.Now()
	if request.CreatedDate != nil {
		createdDate = *request.CreatedDate
	} else if request.StartedDate != nil {
		createdDate = *request.StartedDate
	}

	// prepare deploymentCommits and deployment records
	// queuedDuration := dateInfo.CalculateQueueDuration()
	deploymentCommits := make([]*devops.CicdDeploymentCommit, len(request.DeploymentCommits))
	for i, commit := range request.DeploymentCommits {
		if commit.Result == "" {
			commit.Result = devops.RESULT_SUCCESS
		}
		if commit.Status == "" {
			commit.Status = devops.STATUS_DONE
		}
		if commit.Name == "" {
			commit.Name = fmt.Sprintf(`deployment for %s`, commit.CommitSha)
		}
		if commit.CreatedDate == nil {
			commit.CreatedDate = &createdDate
		}
		if commit.StartedDate == nil {
			commit.StartedDate = request.StartedDate
		}
		if commit.FinishedDate == nil {
			commit.FinishedDate = request.FinishedDate
		}
		// create a deployment_commit record
		deploymentCommits[i] = &devops.CicdDeploymentCommit{
			DomainEntity: domainlayer.DomainEntity{
				Id: GenerateDeploymentCommitId(connection.ID, deploymentId, commit.RepoUrl, commit.CommitSha),
			},
			CicdDeploymentId: deploymentId,
			CicdScopeId:      scopeId,
			Result:           commit.Result,
			Status:           commit.Status,
			OriginalResult:   commit.Result,
			OriginalStatus:   commit.Status,
			TaskDatesInfo: devops.TaskDatesInfo{
				CreatedDate:  *commit.CreatedDate,
				StartedDate:  commit.StartedDate,
				FinishedDate: commit.FinishedDate,
			},
			DurationSec:         &duration,
			RepoId:              commit.RepoId,
			Name:                commit.Name,
			DisplayTitle:        commit.DisplayTitle,
			RepoUrl:             commit.RepoUrl,
			Environment:         request.Environment,
			OriginalEnvironment: request.OriginalEnvironment,
			RefName:             commit.RefName,
			CommitSha:           commit.CommitSha,
			CommitMsg:           commit.CommitMsg,
			//QueuedDurationSec: queuedDuration,
		}
	}

	if err := tx.CreateOrUpdate(deploymentCommits); err != nil {
		logger.Error(err, "failed to save deployment commits")
		return err
	}

	// create a deployment record
	deployment := deploymentCommits[0].ToDeploymentWithCustomDisplayTitle(request.DisplayTitle)
	deployment.Name = name
	deployment.CreatedDate = createdDate
	deployment.StartedDate = request.StartedDate
	deployment.FinishedDate = request.FinishedDate
	deployment.Result = request.Result
	if err := tx.CreateOrUpdate(deployment); err != nil {
		logger.Error(err, "failed to save deployment")
		return err
	}
	return nil
}

func GenerateDeploymentCommitId(connectionId uint64, deploymentId string, repoUrl string, commitSha string) string {
	urlHash16 := fmt.Sprintf("%x", md5.Sum([]byte(repoUrl)))[:16]
	return fmt.Sprintf("%s:%d:%s:%s:%s", "webhook", connectionId, deploymentId, urlHash16, commitSha)
}

// findByProjectName finds the connection by project name and plugin name
func findByProjectName(connection interface{}, params map[string]string, pluginName string, webhookName string) errors.Error {
	projectName := params["projectName"]
	if projectName == "" {
		return errors.BadInput.New("missing projectName")
	}
	if len(projectName) > 100 {
		return errors.BadInput.New("invalid projectName")
	}
	if pluginName == "" {
		return errors.BadInput.New("missing pluginName")
	}
	// We need to join three tables: _tool_webhook_connections, _devlake_blueprint_connections, and _devlake_blueprints
	// to find the connection associated with the given project name and plugin name.
	// The SQL query would look something like this:
	// SELECT wc.*
	// FROM _tool_webhook_connections AS wc
	// JOIN _devlake_blueprint_connections AS bc ON wc.id = bc.connection_id AND bc.plugin_name = ?
	// JOIN _devlake_blueprints AS bp ON bc.blueprint_id = bp.id
	// WHERE bp.project_name = ? and _tool_webhook_connections.name = ?
	// LIMIT 1;

	basicRes.GetLogger().Debug("finding project webhook connection for project %s and plugin %s", projectName, pluginName)
	// Using DAL to construct the query
	clauses := []dal.Clause{dal.From(connection)}
	clauses = append(clauses,
		dal.Join("left join _devlake_blueprint_connections bc ON _tool_webhook_connections.id = bc.connection_id and bc.plugin_name = ?", pluginName),
		dal.Join("left join _devlake_blueprints bp ON bc.blueprint_id = bp.id"),
		dal.Where("bp.project_name = ? and _tool_webhook_connections.name = ?", projectName, webhookName),
	)

	dal := basicRes.GetDal()
	return dal.First(connection, clauses...)
}
