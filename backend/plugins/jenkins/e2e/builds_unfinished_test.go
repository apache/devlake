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

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/jenkins/impl"
	"github.com/apache/devlake/plugins/jenkins/models"
	"github.com/apache/devlake/plugins/jenkins/tasks"
	"github.com/stretchr/testify/assert"
)

var singleJobOptions = &tasks.JenkinsOptions{
	ConnectionId: 1,
	JobName:      `devlake`,
	JobFullName:  `devlake`,
	JobPath:      `job/`,
}

// A single job's builds still running at the last collection are re-collected until they
// finish: the incremental list only returns builds started since that collection.
func TestJenkinsUnfinishedBuilds(t *testing.T) {
	var jenkins impl.Jenkins
	dataflowTester := e2ehelper.NewDataFlowTester(t, "jenkins", jenkins)

	// devlake#1 finished, devlake#2 running, other#7 running but in another job
	dataflowTester.FlushTabler(&models.JenkinsBuild{})
	dataflowTester.ImportCsvIntoTabler("./raw_tables/_tool_jenkins_builds_unfinished.csv", models.JenkinsBuild{})

	var builds []tasks.SimpleBuild
	err := dataflowTester.Dal.All(&builds, tasks.UnfinishedBuilds(singleJobOptions)...)
	assert.Nil(t, err)
	assert.Equal(t, []tasks.SimpleBuild{{Number: "2", FullName: "devlake#2"}}, builds)
}

// A build listed while running and re-collected once finished ends up with its final state.
func TestJenkinsUnfinishedBuildExtraction(t *testing.T) {
	var jenkins impl.Jenkins
	dataflowTester := e2ehelper.NewDataFlowTester(t, "jenkins", jenkins)

	// row 1: devlake#5 from the list, running; row 2: its detail, re-collected once finished
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_jenkins_api_builds_unfinished.csv", "_raw_jenkins_api_builds")
	dataflowTester.FlushTabler(&models.JenkinsBuild{})
	dataflowTester.FlushTabler(&models.JenkinsBuildCommit{})
	dataflowTester.Subtask(tasks.ExtractApiBuildsMeta, &tasks.JenkinsTaskData{Options: singleJobOptions})

	var build models.JenkinsBuild
	err := dataflowTester.Dal.First(&build, dal.Where("full_name = ?", "devlake#5"))
	assert.Nil(t, err)
	assert.False(t, build.Building)
	assert.Equal(t, "SUCCESS", build.Result)
	assert.Equal(t, float64(90000000), build.Duration)
	assert.Equal(t, "devlake", build.JobName)
	assert.Equal(t, "job/", build.JobPath)
}
