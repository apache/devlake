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

	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/argocd/impl"
	"github.com/apache/devlake/plugins/argocd/models"
	"github.com/apache/devlake/plugins/argocd/models/migrationscripts/archived"
	"github.com/apache/devlake/plugins/argocd/tasks"
)

func TestSyncOperationDataFlow(t *testing.T) {
	var argocd impl.ArgoCD
	dataflowTester := e2ehelper.NewDataFlowTester(t, "argocd", argocd)

	taskData := &tasks.ArgocdTaskData{
		Options: &tasks.ArgocdOptions{
			ConnectionId:    1,
			ApplicationName: "myapp",
			ScopeConfig:     &models.ArgocdScopeConfig{},
		},
	}

	// Two collections: the first saw revision bbb Degraded, the second a newer Healthy
	// sync, by which time bbb is an ordinary history entry again.
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_argocd_api_sync_operations.csv", "_raw_argocd_api_sync_operations")
	dataflowTester.FlushTabler(&archived.ArgocdApplication{})
	dataflowTester.FlushTabler(&models.ArgocdRevisionImage{})

	dataflowTester.FlushTabler(&models.ArgocdSyncOperation{})
	dataflowTester.Subtask(tasks.ExtractSyncOperationsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		models.ArgocdSyncOperation{},
		e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/_tool_argocd_sync_operations.csv",
			TargetFields: []string{"connection_id", "application_name", "deployment_id", "revision", "phase", "health_status"},
		},
	)

	dataflowTester.FlushTabler(&devops.CICDDeployment{})
	dataflowTester.FlushTabler(&devops.CicdDeploymentCommit{})
	dataflowTester.Subtask(tasks.ConvertSyncOperationsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(
		devops.CICDDeployment{},
		e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/cicd_deployments.csv",
			TargetFields: []string{"id", "result", "status", "original_status", "original_result"},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		devops.CicdDeploymentCommit{},
		e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/cicd_deployment_commits.csv",
			TargetFields: []string{"id", "cicd_deployment_id", "result", "commit_sha"},
		},
	)
}
