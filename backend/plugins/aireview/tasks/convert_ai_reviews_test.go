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
	"strings"
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

func TestGenerateAiDomainId(t *testing.T) {
	t.Run("deterministic output", func(t *testing.T) {
		id1 := generateAiDomainId("ar", "my-project", "src-123")
		id2 := generateAiDomainId("ar", "my-project", "src-123")
		assert.Equal(t, id1, id2)
	})

	t.Run("different prefix produces different ID", func(t *testing.T) {
		id1 := generateAiDomainId("ar", "proj", "src-1")
		id2 := generateAiDomainId("fp", "proj", "src-1")
		assert.NotEqual(t, id1, id2)
	})

	t.Run("different project produces different ID", func(t *testing.T) {
		id1 := generateAiDomainId("ar", "project-a", "src-1")
		id2 := generateAiDomainId("ar", "project-b", "src-1")
		assert.NotEqual(t, id1, id2)
	})

	t.Run("different source produces different ID", func(t *testing.T) {
		id1 := generateAiDomainId("ar", "proj", "src-1")
		id2 := generateAiDomainId("ar", "proj", "src-2")
		assert.NotEqual(t, id1, id2)
	})

	t.Run("prefix appears in output", func(t *testing.T) {
		id := generateAiDomainId("ar", "proj", "src-1")
		assert.True(t, strings.HasPrefix(id, "ar:"))
	})

	t.Run("non-empty output", func(t *testing.T) {
		id := generateAiDomainId("x", "y", "z")
		assert.NotEmpty(t, id)
	})
}

func TestSaveAiReviewBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

		batch := []*domainCode.AiReview{
			{ProjectName: "proj1"},
			{ProjectName: "proj2"},
		}
		err := saveAiReviewBatch(mockDal, batch)
		assert.Nil(t, err)
		mockDal.AssertNumberOfCalls(t, "CreateOrUpdate", 2)
	})

	t.Run("empty batch", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		err := saveAiReviewBatch(mockDal, []*domainCode.AiReview{})
		assert.Nil(t, err)
	})

	t.Run("error on save", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		batch := []*domainCode.AiReview{{ProjectName: "p1"}}
		err := saveAiReviewBatch(mockDal, batch)
		assert.NotNil(t, err)
	})
}

func TestConvertAiReviews_NoProjectName(t *testing.T) {
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

	err := ConvertAiReviews(mockCtx)
	assert.Nil(t, err)
}

func TestConvertAiReviews_WithProject(t *testing.T) {
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
		dst := args.Get(1).(*models.AiReview)
		*dst = models.AiReview{
			Id:            "src-1",
			PullRequestId: "pr-1",
			RepoId:        "repo-1",
			AiTool:        "CodeRabbit",
			RiskLevel:     "high",
		}
	}).Return(nil)

	mockDalI.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertAiReviews(mockCtx)
	assert.Nil(t, err)
	mockDalI.AssertCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything)
}
