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
	"testing"
	"time"

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/apache/devlake/plugins/bamboo/models"
	"github.com/stretchr/testify/assert"
)

func TestConvertDeployBuildsToDeploymentCommitsInvalidEndpoint(t *testing.T) {
	client := &api.ApiClient{}
	client.Setup("http://bamboo.example.com:port/rest/api/latest/", nil, time.Second)
	// The endpoint is checked before any query, so the mock dal must not be called.
	taskCtx := unithelper.DummySubTaskContext(mockdal.NewDal(t))
	taskCtx.On("GetData").Return(&BambooOptions{
		Options:   &models.BambooOptions{ConnectionId: 1, PlanKey: "TEST-PLA2"},
		ApiClient: &api.ApiAsyncClient{ApiClient: client},
	})

	err := ConvertDeployBuildsToDeploymentCommits(taskCtx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse the Bamboo endpoint")
}
