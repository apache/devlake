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
	"reflect"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
)

var ConnectIncidentToDeploymentMeta = plugin.SubTaskMeta{
	Name:             "ConnectIncidentToDeployment",
	EntryPoint:       ConnectIncidentToDeployment,
	EnabledByDefault: true,
	Description:      "Connect incident issue to deployment",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
}

type simpleCicdDeploymentCommit struct {
	Id           string
	FinishedDate *time.Time
}

// ConnectIncidentToDeployment will generate data to crossdomain.ProjectIncidentDeploymentRelationship.
func ConnectIncidentToDeployment(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*DoraTaskData)
	logger := taskCtx.GetLogger()
	// Clear previous results from the project
	err := db.Exec("DELETE FROM project_incident_deployment_relationships WHERE project_name = ?", data.Options.ProjectName)
	if err != nil {
		return errors.Default.Wrap(err, "error deleting previous project_incident_deployment_relationships")
	}
	logger.Info("delete previous project_incident_deployment_relationships")
	// select all issues belongs to the board
	clauses := []dal.Clause{
		dal.From(`incidents i`),
		dal.Join(`left join project_mapping pm on pm.row_id = i.scope_id and pm.table = i.table`),
		dal.Where("pm.project_name = ?", data.Options.ProjectName),
	}

	//count, err := db.Count(
	//	dal.From(`incidents i`),
	//	dal.Join(`left join project_mapping pm on pm.row_id = i.scope_id and pm.table = i.table`),
	//	dal.Where("pm.project_name = ?", data.Options.ProjectName),
	//)
	//if err != nil {
	//	logger.Error(err, "count incidents")
	//} else {
	//	logger.Info("incident count is %d", count)
	//}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		logger.Error(err, "db.cursor error")
		return err
	}
	defer cursor.Close()
	logger.Info("start enricher")
	enricher, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: DoraApiParams{
				ProjectName: data.Options.ProjectName,
			},
			Table: "incidents",
		},
		InputRowType: reflect.TypeOf(ticket.Incident{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			incident := inputRow.(*ticket.Incident)
			projectIssueMetric := &crossdomain.ProjectIncidentDeploymentRelationship{
				DomainEntity: domainlayer.DomainEntity{
					Id: incident.Id,
				},
				ProjectName: data.Options.ProjectName,
			}
			logger.Debug("get incident: %+v", incident.Id)
			scdc, err := findIncidentDeployment(db, incident, data.Options.ProjectName)
			if err != nil {
				logger.Error(err, "get all deployment commits")
				return nil, err
			}
			if scdc == nil {
				logger.Warn(nil, "deployment commit not found for incident %s", incident.Id)
				return nil, nil
			}
			projectIssueMetric.DeploymentId = scdc.Id
			return []interface{}{projectIssueMetric}, nil
		},
	})
	if err != nil {
		return err
	}

	return enricher.Execute()
}

// findIncidentDeployment returns the latest successful production deployment of the project that finished before
// the incident started. When the incident names a component that matches a CI/CD scope of the project (its name,
// such as "owner/repo", or its id, case-insensitive), only that scope's deployments are considered, so the incident
// is tied to the release of the repository that caused it rather than to whichever repository deployed last. If the
// component matches no scope, or that scope has no earlier production deployment, the project-wide lookup is used.
func findIncidentDeployment(db dal.Dal, incident *ticket.Incident, projectName string) (*simpleCicdDeploymentCommit, errors.Error) {
	component := strings.TrimSpace(incident.Component)
	if component != "" {
		scdc, err := latestProductionDeployment(db, incident, projectName, component)
		if err != nil || scdc != nil {
			return scdc, err
		}
	}
	return latestProductionDeployment(db, incident, projectName, "")
}

func latestProductionDeployment(db dal.Dal, incident *ticket.Incident, projectName string, component string) (*simpleCicdDeploymentCommit, errors.Error) {
	clauses := []dal.Clause{
		dal.Select("cicd_deployment_commits.cicd_deployment_id as id, cicd_deployment_commits.finished_date as finished_date"),
		dal.From(&devops.CicdDeploymentCommit{}),
		dal.Join("left join project_mapping pm on cicd_deployment_commits.cicd_scope_id = pm.row_id"),
	}
	where := `cicd_deployment_commits.finished_date < ?
		and cicd_deployment_commits.result = ?
		and cicd_deployment_commits.environment = ?
		and pm.table = ?
		and pm.project_name = ?`
	args := []interface{}{incident.CreatedDate, devops.RESULT_SUCCESS, devops.PRODUCTION, "cicd_scopes", projectName}
	if component != "" {
		clauses = append(clauses, dal.Join("left join cicd_scopes cs on cs.id = cicd_deployment_commits.cicd_scope_id"))
		where += ` and (lower(cs.name) = ? or lower(cs.id) = ?)`
		lowered := strings.ToLower(component)
		args = append(args, lowered, lowered)
	}
	clauses = append(clauses, dal.Where(where, args...), dal.Orderby("finished_date DESC"), dal.Limit(1))
	scdc := &simpleCicdDeploymentCommit{}
	err := db.All(scdc, clauses...)
	if err != nil {
		if db.IsErrorNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if scdc.Id == "" {
		return nil, nil
	}
	return scdc, nil
}
