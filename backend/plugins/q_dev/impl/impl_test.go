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

package impl

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apache/incubator-devlake/plugins/q_dev/tasks"
)

func TestQDev_BasicPluginMethods(t *testing.T) {
	plugin := &QDev{}

	assert.Equal(t, "q_dev", plugin.Name())
	assert.Equal(t, "To collect and enrich data from AWS Q Developer usage metrics", plugin.Description())
	assert.Equal(t, "github.com/apache/incubator-devlake/plugins/q_dev", plugin.RootPkgPath())

	// Test table info
	tables := plugin.GetTablesInfo()
	assert.Len(t, tables, 7)

	// Test subtask metas
	subtasks := plugin.SubTaskMetas()
	assert.Len(t, subtasks, 3)

	// Test API resources
	apiResources := plugin.ApiResources()
	assert.NotEmpty(t, apiResources)
	assert.Contains(t, apiResources, "test")
	assert.Contains(t, apiResources, "connections")
}

func TestQDev_TaskDataStructure(t *testing.T) {
	// Test that QDevTaskData has the expected structure (legacy mode)
	taskData := &tasks.QDevTaskData{
		Options: &tasks.QDevOptions{
			ConnectionId: 1,
			S3Prefix:     "test/",
		},
		S3Client: &tasks.QDevS3Client{
			Bucket: "test-bucket",
		},
		IdentityClient: &tasks.QDevIdentityClient{
			StoreId: "d-1234567890",
			Region:  "us-west-2",
		},
		S3Prefixes: []string{"test/"},
	}

	assert.NotNil(t, taskData.Options)
	assert.NotNil(t, taskData.S3Client)
	assert.NotNil(t, taskData.IdentityClient)

	assert.Equal(t, uint64(1), taskData.Options.ConnectionId)
	assert.Equal(t, "test/", taskData.Options.S3Prefix)
	assert.Equal(t, "test-bucket", taskData.S3Client.Bucket)
	assert.Equal(t, "d-1234567890", taskData.IdentityClient.StoreId)
	assert.Equal(t, "us-west-2", taskData.IdentityClient.Region)
	assert.Equal(t, []string{"test/"}, taskData.S3Prefixes)
}

func TestQDev_TaskDataWithAccountId(t *testing.T) {
	// Test new-style scope with AccountId and multiple S3Prefixes
	month := 1
	taskData := &tasks.QDevTaskData{
		Options: &tasks.QDevOptions{
			ConnectionId: 1,
			AccountId:    "034362076319",
			BasePath:     "user-report",
			Year:         2026,
			Month:        &month,
		},
		S3Client: &tasks.QDevS3Client{
			Bucket: "test-bucket",
		},
		S3Prefixes: []string{
			"user-report/AWSLogs/034362076319/KiroLogs/by_user_analytic/us-east-1/2026/01",
			"user-report/AWSLogs/034362076319/KiroLogs/user_report/us-east-1/2026/01",
		},
	}

	assert.Equal(t, "034362076319", taskData.Options.AccountId)
	assert.Equal(t, "user-report", taskData.Options.BasePath)
	assert.Equal(t, 2026, taskData.Options.Year)
	assert.Equal(t, &month, taskData.Options.Month)
	assert.Len(t, taskData.S3Prefixes, 2)
	assert.Contains(t, taskData.S3Prefixes[0], "by_user_analytic")
	assert.Contains(t, taskData.S3Prefixes[1], "user_report")
}

func TestQDev_TaskDataWithoutIdentityClient(t *testing.T) {
	// Test that QDevTaskData works without IdentityClient
	taskData := &tasks.QDevTaskData{
		Options: &tasks.QDevOptions{
			ConnectionId: 1,
		},
		S3Client: &tasks.QDevS3Client{
			Bucket: "test-bucket",
		},
		IdentityClient: nil,
		S3Prefixes:     []string{"some-prefix/"},
	}

	assert.NotNil(t, taskData.Options)
	assert.NotNil(t, taskData.S3Client)
	assert.Nil(t, taskData.IdentityClient)
	assert.Len(t, taskData.S3Prefixes, 1)
}
