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
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/jenkins/impl"
	"github.com/apache/devlake/plugins/jenkins/models"
	"github.com/apache/devlake/plugins/jenkins/tasks"
	"github.com/stretchr/testify/assert"
)

// The incremental stage collection must include builds that started before the
// last collection but finished after it (e.g. a build that waited days on an
// input step): they reach _tool_jenkins_builds only once finished.
func TestJenkinsStagesFinishedSince(t *testing.T) {
	var jenkins impl.Jenkins
	dataflowTester := e2ehelper.NewDataFlowTester(t, "jenkins", jenkins)

	// devlake#1: started 3 days before, ran 1 hour     -> collected by an earlier run
	// devlake#2: started 3 days before, ran 4 days     -> finished after the last run
	// devlake#3: started after the last run, ran 10 min
	dataflowTester.FlushTabler(&models.JenkinsBuild{})
	dataflowTester.ImportCsvIntoTabler("./raw_tables/_tool_jenkins_builds_finished_since.csv", models.JenkinsBuild{})

	lastCollection := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var builds []string
	err := dataflowTester.Dal.Pluck("tjb.full_name", &builds,
		dal.From("_tool_jenkins_builds as tjb"),
		tasks.FinishedSince(lastCollection),
		dal.Orderby("tjb.full_name"),
	)
	assert.Nil(t, err)
	assert.Equal(t, []string{"devlake#2", "devlake#3"}, builds)
}
