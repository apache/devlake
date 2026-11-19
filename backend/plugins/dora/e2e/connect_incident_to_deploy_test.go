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

package e2e

import (
	"testing"

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/dora/impl"
	"github.com/apache/devlake/plugins/dora/tasks"
)

func TestConnectIncidentToDeploymentDataFlow(t *testing.T) {
	var plugin impl.Dora
	dataflowTester := e2ehelper.NewDataFlowTester(t, "dora", plugin)

	taskData := &tasks.DoraTaskData{
		Options: &tasks.DoraOptions{
			ProjectName: "project1",
		},
	}
	// import raw data table
	dataflowTester.ImportCsvIntoTabler("./connect_incident_to_deployment/prev_success_deployment_commit/cicd_deployment_commits_after.csv", &devops.CicdDeploymentCommit{})
	dataflowTester.ImportCsvIntoTabler("./connect_incident_to_deployment/raw_tables/project_mapping.csv", &crossdomain.ProjectMapping{})
	dataflowTester.ImportCsvIntoTabler("./connect_incident_to_deployment/raw_tables/incidents.csv", &ticket.Incident{})

	// verify converter
	dataflowTester.FlushTabler(&crossdomain.ProjectIncidentDeploymentRelationship{})
	dataflowTester.Subtask(tasks.ConnectIncidentToDeploymentMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&crossdomain.ProjectIncidentDeploymentRelationship{}, e2ehelper.TableOptions{
		CSVRelPath:  "./connect_incident_to_deployment/snapshot_tables/project_incident_deployment_relationships.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}

// Incidents whose component names a CI/CD scope of the project (by name or id, any case) are tied to that scope's
// latest production deployment; any other component, or none, keeps the project-wide lookup.
func TestConnectIncidentToDeploymentByComponentDataFlow(t *testing.T) {
	var plugin impl.Dora
	dataflowTester := e2ehelper.NewDataFlowTester(t, "dora", plugin)

	taskData := &tasks.DoraTaskData{
		Options: &tasks.DoraOptions{
			ProjectName: "project1",
		},
	}
	dataflowTester.ImportCsvIntoTabler("./connect_incident_to_deployment/prev_success_deployment_commit/cicd_deployment_commits_after.csv", &devops.CicdDeploymentCommit{})
	dataflowTester.ImportCsvIntoTabler("./connect_incident_to_deployment/raw_tables/project_mapping.csv", &crossdomain.ProjectMapping{})
	dataflowTester.ImportCsvIntoTabler("./connect_incident_to_deployment/by_component/cicd_scopes.csv", &devops.CicdScope{})
	dataflowTester.ImportCsvIntoTabler("./connect_incident_to_deployment/by_component/incidents.csv", &ticket.Incident{})

	dataflowTester.FlushTabler(&crossdomain.ProjectIncidentDeploymentRelationship{})
	dataflowTester.Subtask(tasks.ConnectIncidentToDeploymentMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&crossdomain.ProjectIncidentDeploymentRelationship{}, e2ehelper.TableOptions{
		CSVRelPath:  "./connect_incident_to_deployment/by_component/project_incident_deployment_relationships.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
