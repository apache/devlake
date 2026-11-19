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
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/models/common"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/bamboo/models"
)

var ConvertDeployBuildsToDeploymentCommitsMeta = plugin.SubTaskMeta{
	Name:             "convertDeployBuildsToDeploymentCommits",
	EntryPoint:       ConvertDeployBuildsToDeploymentCommits,
	EnabledByDefault: true,
	Description:      "Convert tool layer table bamboo_deploy_builds into domain layer table cicd_deployment_commits",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

// deployBuildWithVcsRevision is a virtual tool layer table,
// it's used to store records from a complicated sql
// and generate id field for table cicd_deployment_commits.
type deployBuildWithVcsRevision struct {
	ConnectionId          uint64     `json:"connection_id" gorm:"primaryKey"`
	DeployBuildId         uint64     `json:"deploy_build_id" gorm:"primaryKey"`
	PlanResultKey         string     `json:"planResultKey" `
	EnvKey                string     `json:"envKey"`
	DeploymentVersionName string     `json:"deploymentVersionName"`
	DeploymentState       string     `json:"deploymentState"`
	LifeCycleState        string     `json:"lifeCycleState"`
	StartedDate           *time.Time `json:"startedDate"`
	QueuedDate            *time.Time `json:"queuedDate"`
	ExecutedDate          *time.Time `json:"executedDate"`
	FinishedDate          *time.Time `json:"finishedDate"`
	ReasonSummary         string     `json:"reasonSummary"`
	ProjectKey            string     `json:"project_key" gorm:"index"`
	PlanKey               string     `json:"plan_key" gorm:"index"`
	Environment           string     `gorm:"type:varchar(255)"`
	PlanBranchName        string     `gorm:"type:varchar(255)"`
	models.ApiBambooOperations
	common.NoPKModel
	RepositoryId    int `json:"repository_id" gorm:"primaryKey"`
	RepositoryName  string
	VcsRevisionKey  string
	ProjectPlanName string
	ProjectName     string
}

func (deployBuildWithVcsRevision deployBuildWithVcsRevision) GenerateCICDDeploymentCommitName() string {
	if deployBuildWithVcsRevision.ProjectPlanName != "" {
		return fmt.Sprintf("%s/%s", deployBuildWithVcsRevision.ProjectPlanName, deployBuildWithVcsRevision.DeploymentVersionName)
	}
	return deployBuildWithVcsRevision.DeploymentVersionName
}

func ConvertDeployBuildsToDeploymentCommits(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RAW_JOB_BUILD_TABLE)
	endpoint, parseErr := url.Parse(data.ApiClient.GetEndpoint())
	if parseErr != nil {
		return errors.Default.Wrap(parseErr, "failed to parse the Bamboo endpoint")
	}
	baseURL := endpoint.Scheme + "://" + endpoint.Host
	// INNER JOIN may cause loss of deploy entities here.
	// It is not known that bamboo deploy can be associated with commits, so this is not an issue at the moment.
	cursor, err := db.Cursor(
		dal.Select("db.*, pbc.repository_id, pbc.repository_name, pbc.vcs_revision_key, p.name as project_plan_name, p.project_name"),
		dal.From("_tool_bamboo_deploy_builds AS db"),
		dal.Join("INNER JOIN _tool_bamboo_plan_build_commits AS pbc ON db.connection_id = pbc.connection_id AND db.plan_result_key = pbc.plan_result_key"),
		dal.Join("LEFT JOIN _tool_bamboo_plans as p ON db.plan_key = p.plan_key"),
		dal.Where("db.connection_id = ? and db.plan_key = ?", data.Options.ConnectionId, data.Options.PlanKey))
	if err != nil {
		return err
	}
	defer cursor.Close()

	planIdGen := didgen.NewDomainIdGenerator(&models.BambooPlan{})
	converter, err := api.NewDataConverter(api.DataConverterArgs{
		InputRowType:       reflect.TypeOf(deployBuildWithVcsRevision{}),
		Input:              cursor,
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			input := inputRow.(*deployBuildWithVcsRevision)
			if input.VcsRevisionKey == "" {
				return nil, nil
			}
			deploymentCommitId := didgen.NewDomainIdGenerator(&deployBuildWithVcsRevision{}).Generate(data.Options.ConnectionId, input.DeployBuildId, input.RepositoryId)
			createdDate := time.Now()
			if input.StartedDate != nil {
				createdDate = *input.StartedDate
			}
			deploymentCommit := &devops.CicdDeploymentCommit{
				DomainEntity: domainlayer.DomainEntity{
					Id: deploymentCommitId,
				},
				CicdScopeId:      planIdGen.Generate(data.Options.ConnectionId, data.Options.PlanKey),
				CicdDeploymentId: deploymentCommitId,
				Name:             input.GenerateCICDDeploymentCommitName(),
				Result: devops.GetResult(&devops.ResultRule{
					Success: []string{ResultSuccess, ResultSuccessful},
					Failure: []string{ResultFailed},
					Default: devops.RESULT_DEFAULT,
				}, input.DeploymentState),
				OriginalResult: input.DeploymentState,
				Status: devops.GetStatus(&devops.StatusRule{
					Done:       []string{StatusFinished},
					InProgress: []string{StatusInProgress, StatusPending, StatusQueued},
					Default:    devops.STATUS_OTHER,
				}, input.LifeCycleState),
				OriginalStatus:      input.LifeCycleState,
				Environment:         input.Environment,
				OriginalEnvironment: input.Environment,
				TaskDatesInfo: devops.TaskDatesInfo{
					CreatedDate:  createdDate,
					QueuedDate:   input.QueuedDate,
					StartedDate:  input.ExecutedDate,
					FinishedDate: input.FinishedDate,
				},
				CommitSha:    input.VcsRevisionKey,
				RefName:      input.PlanBranchName,
				RepoId:       strconv.Itoa(input.RepositoryId),
				DisplayTitle: input.GenerateCICDDeploymentCommitName(),
			}
			parts := strings.Split(input.EnvKey, "-")
			if len(parts) > 1 {
				deploymentCommit.Url = fmt.Sprintf("%s/deploy/viewEnvironment.action?id=%s", baseURL, parts[1])
			}
			if data.RegexEnricher.ReturnNameIfMatched(devops.ENV_NAME_PATTERN, input.Environment) != "" {
				deploymentCommit.Environment = devops.PRODUCTION
			}
			if input.FinishedDate != nil && input.ExecutedDate != nil {
				duration := float64(input.FinishedDate.Sub(*input.ExecutedDate).Milliseconds() / 1e3)
				deploymentCommit.DurationSec = &duration
			}
			fakeRepoUrl, err := generateFakeRepoUrl(data.ApiClient.GetEndpoint(), input.RepositoryId)
			if err != nil {
				logger.Warn(err, "generate fake repo url, endpoint: %s, repo id: %d", data.ApiClient.GetEndpoint(), input.RepositoryId)
			} else {
				deploymentCommit.RepoUrl = fakeRepoUrl
			}

			return []interface{}{deploymentCommit}, nil
		},
	})

	if err != nil {
		return err
	}

	return converter.Execute()
}
