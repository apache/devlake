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
	"reflect"
	"regexp"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/argocd/models"
)

var _ plugin.SubTaskEntryPoint = ConvertSyncOperations

var ConvertSyncOperationsMeta = plugin.SubTaskMeta{
	Name:             "convertSyncOperations",
	EntryPoint:       ConvertSyncOperations,
	EnabledByDefault: true,
	Description:      "Convert sync operations to domain layer deployments",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{models.ArgocdSyncOperation{}.TableName()},
	ProductTables:    []string{"cicd_deployments", "cicd_deployment_commits"},
}

func ConvertSyncOperations(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*ArgocdTaskData)
	db := taskCtx.GetDal()

	var application *models.ArgocdApplication
	app := &models.ArgocdApplication{}
	if err := db.First(
		app,
		dal.Where("connection_id = ? AND name = ?", data.Options.ConnectionId, data.Options.ApplicationName),
	); err == nil {
		application = app
	}

	cursor, err := db.Cursor(
		dal.From(&models.ArgocdSyncOperation{}),
		dal.Where("connection_id = ? AND application_name = ?",
			data.Options.ConnectionId, data.Options.ApplicationName),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:   taskCtx,
			Table: RAW_SYNC_OPERATION_TABLE,
			Params: models.ArgocdApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.ApplicationName,
			},
		},
		InputRowType: reflect.TypeOf(models.ArgocdSyncOperation{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			syncOp := inputRow.(*models.ArgocdSyncOperation)
			results := make([]interface{}, 0, 2)

			if !includeSyncOperation(syncOp, data.Options.ScopeConfig, data.RegexEnricher) {
				return nil, nil
			}

			scopeId := didgen.NewDomainIdGenerator(&models.ArgocdApplication{}).
				Generate(data.Options.ConnectionId, syncOp.ApplicationName)

			deploymentId := didgen.NewDomainIdGenerator(&models.ArgocdSyncOperation{}).
				Generate(data.Options.ConnectionId, syncOp.ApplicationName, syncOp.DeploymentId)

			var created time.Time
			switch {
			case syncOp.StartedAt != nil && !syncOp.StartedAt.IsZero():
				created = *syncOp.StartedAt
			case syncOp.FinishedAt != nil && !syncOp.FinishedAt.IsZero():
				created = *syncOp.FinishedAt
			case !syncOp.CreatedAt.IsZero():
				created = syncOp.CreatedAt
			default:
				created = time.Now()
			}
			if created.IsZero() || created.Year() <= 1 {
				created = time.Now()
			}
			created = created.UTC()

			deployment := &devops.CICDDeployment{
				DomainEntity: domainlayer.DomainEntity{
					Id: deploymentId,
				},
				CicdScopeId:         scopeId,
				Name:                fmt.Sprintf("%s:%d", syncOp.ApplicationName, syncOp.DeploymentId),
				DisplayTitle:        syncOp.Message,
				Result:              convertPhaseToResult(syncOp.Phase),
				Status:              convertPhaseToStatus(syncOp.Phase),
				OriginalStatus:      syncOp.Phase,
				OriginalResult:      syncOp.Phase,
				Environment:         detectEnvironment(syncOp, application, data.Options.ScopeConfig, data.RegexEnricher),
				OriginalEnvironment: syncOp.ApplicationName,
				TaskDatesInfo: devops.TaskDatesInfo{
					CreatedDate:  created,
					StartedDate:  syncOp.StartedAt,
					FinishedDate: syncOp.FinishedAt,
				},
			}

			if syncOp.StartedAt != nil && syncOp.FinishedAt != nil {
				duration := syncOp.FinishedAt.Sub(*syncOp.StartedAt).Seconds()
				deployment.DurationSec = &duration
			}

			results = append(results, deployment)

			if syncOp.Revision != "" {
				// Priority: repo_url resolved at extraction time (always present for
				// multi-source apps) → application-level repo_url → deployment name
				// as a last-resort non-empty placeholder.
				repoUrl := deployment.Name
				switch {
				case syncOp.RepoURL != "":
					repoUrl = syncOp.RepoURL
				case application != nil && application.RepoURL != "":
					repoUrl = application.RepoURL
				}

				deploymentCommit := &devops.CicdDeploymentCommit{
					DomainEntity:        domainlayer.NewDomainEntity(deploymentId),
					CicdDeploymentId:    deploymentId,
					CicdScopeId:         scopeId,
					Name:                deployment.Name,
					DisplayTitle:        deployment.DisplayTitle,
					Url:                 deployment.Url,
					Result:              deployment.Result,
					Status:              deployment.Status,
					OriginalStatus:      deployment.OriginalStatus,
					OriginalResult:      deployment.OriginalResult,
					Environment:         deployment.Environment,
					OriginalEnvironment: deployment.OriginalEnvironment,
					TaskDatesInfo: devops.TaskDatesInfo{
						CreatedDate:  created,
						StartedDate:  syncOp.StartedAt,
						FinishedDate: syncOp.FinishedAt,
					},
					CommitSha: syncOp.Revision,
					RepoUrl:   repoUrl,
				}
				results = append(results, deploymentCommit)
			}

			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return converter.Execute()
}

func convertPhaseToResult(phase string) string {
	switch phase {
	case "Succeeded":
		return devops.RESULT_SUCCESS
	case "Failed", "Error":
		return devops.RESULT_FAILURE
	case "Terminating":
		return devops.RESULT_FAILURE
	case "Running":
		return devops.RESULT_DEFAULT
	default:
		return devops.RESULT_DEFAULT
	}
}

func convertPhaseToStatus(phase string) string {
	switch phase {
	case "Succeeded", "Failed", "Error":
		return devops.STATUS_DONE
	case "Running", "Terminating":
		return devops.STATUS_IN_PROGRESS
	default:
		return devops.STATUS_OTHER
	}
}

func detectEnvironment(
	syncOp *models.ArgocdSyncOperation,
	application *models.ArgocdApplication,
	config *models.ArgocdScopeConfig,
	enricher *api.RegexEnricher,
) string {
	if config == nil {
		return devops.TESTING
	}

	targets := []string{syncOp.ApplicationName}
	if application != nil {
		targets = append(targets,
			application.Name,
			application.Namespace,
			application.DestNamespace,
		)
	}

	if enricher != nil {
		if config.ProductionPattern != "" && enricher.ReturnNameIfMatched(devops.PRODUCTION, targets...) != "" {
			return devops.PRODUCTION
		}
		if enricher.ReturnNameIfMatched(devops.ENV_NAME_PATTERN, targets...) != "" {
			return devops.PRODUCTION
		}
		return devops.TESTING
	}

	if config.ProductionPattern != "" {
		if re, err := regexp.Compile(config.ProductionPattern); err == nil && re.MatchString(syncOp.ApplicationName) {
			return devops.PRODUCTION
		}
	}

	envPattern := config.EnvNamePattern
	if envPattern == "" {
		envPattern = "(?i)prod(.*)"
	}

	if re, err := regexp.Compile(envPattern); err == nil && re.MatchString(syncOp.ApplicationName) {
		return devops.PRODUCTION
	}

	return devops.TESTING
}

func includeSyncOperation(
	syncOp *models.ArgocdSyncOperation,
	config *models.ArgocdScopeConfig,
	enricher *api.RegexEnricher,
) bool {
	if config == nil || config.DeploymentPattern == "" {
		return true
	}
	if enricher != nil {
		return enricher.ReturnNameIfMatched(devops.DEPLOYMENT, syncOp.ApplicationName) != ""
	}
	re, err := regexp.Compile(config.DeploymentPattern)
	if err != nil {
		return true
	}
	return re.MatchString(syncOp.ApplicationName)
}
