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
	domainCode "github.com/apache/devlake/core/models/domainlayer/code"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	mocklog "github.com/apache/devlake/mocks/core/log"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSaveFailurePredictionBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

		batch := []*domainCode.AiFailurePrediction{
			{ProjectName: "proj1"},
			{ProjectName: "proj2"},
		}
		err := saveFailurePredictionBatch(mockDal, batch)
		assert.Nil(t, err)
		mockDal.AssertNumberOfCalls(t, "CreateOrUpdate", 2)
	})

	t.Run("empty batch", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		err := saveFailurePredictionBatch(mockDal, []*domainCode.AiFailurePrediction{})
		assert.Nil(t, err)
	})

	t.Run("error on save", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		batch := []*domainCode.AiFailurePrediction{{ProjectName: "p1"}}
		err := saveFailurePredictionBatch(mockDal, batch)
		assert.NotNil(t, err)
	})
}

func TestConvertFailurePredictions_NoProjectName(t *testing.T) {
	mockCtx := new(mockplugin.SubTaskContext)
	mockDalI := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)

	data := &AiReviewTaskData{
		Options: &AiReviewOptions{ProjectName: ""},
	}

	mockCtx.On("GetDal").Return(mockDalI)
	mockCtx.On("GetLogger").Return(mockLogger)
	mockCtx.On("GetData").Return(data)
	mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()

	err := ConvertFailurePredictions(mockCtx)
	assert.Nil(t, err)
}

func TestConvertFailurePredictions_WithProject(t *testing.T) {
	mockCtx := new(mockplugin.SubTaskContext)
	mockDalI := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)
	mockRows := new(mockdal.Rows)

	data := &AiReviewTaskData{
		Options: &AiReviewOptions{ProjectName: "my-project"},
	}

	mockCtx.On("GetDal").Return(mockDalI)
	mockCtx.On("GetLogger").Return(mockLogger)
	mockCtx.On("GetData").Return(data)
	mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()

	mockDalI.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDalI.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	mockDalI.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*models.AiFailurePrediction)
		*dst = models.AiFailurePrediction{
			Id:            "pred-1",
			PullRequestId: "pr-1",
			RepoId:        "repo-1",
			AiTool:        "CodeRabbit",
			RiskScore:     80,
		}
	}).Return(nil)

	mockDalI.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertFailurePredictions(mockCtx)
	assert.Nil(t, err)
	mockDalI.AssertCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything)
}
