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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/unithelper"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCollectIncidentsReturnsCollectorSetupError(t *testing.T) {
	// Loading the previous collector state fails, so the collector can't be created.
	dbErr := errors.Default.New("db unavailable")
	mockDal := new(mockdal.Dal)
	mockDal.On("First", mock.Anything, mock.Anything).Return(dbErr).Once()
	mockDal.On("IsErrorNotFound", dbErr).Return(false).Once()

	taskCtx := unithelper.DummySubTaskContext(mockDal)
	taskCtx.On("GetData").Return(&PagerDutyTaskData{
		Options: &PagerDutyOptions{ConnectionId: 1, ServiceId: "service-1", ServiceName: "service"},
	})

	err := CollectIncidents(taskCtx)

	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "db unavailable")
	}
	mockDal.AssertExpectations(t)
}
