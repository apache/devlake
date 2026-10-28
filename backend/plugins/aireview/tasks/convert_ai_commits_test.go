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

func TestConvertAiCommits_NoProjectName(t *testing.T) {
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

	err := ConvertAiCommits(mockCtx)
	assert.Nil(t, err)
}

func TestConvertAiCommits_WithProject(t *testing.T) {
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
		dst := args.Get(1).(*models.AiCommit)
		*dst = models.AiCommit{
			Id:        "src-1",
			CommitSha: "abc123",
			RepoId:    "repo-1",
			AiTool:    models.AiToolCursor,
		}
	}).Return(nil)

	// Generated Dal.CreateOrUpdate mock always records (entity, clauses), even
	// when production code calls CreateOrUpdate(c) with no extra clauses.
	mockDalI.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertAiCommits(mockCtx)
	assert.Nil(t, err)
	mockDalI.AssertCalled(t, "CreateOrUpdate", mock.Anything, mock.Anything)
}

func TestSaveDomainAiCommitBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)
		err := saveDomainAiCommitBatch(mockDal, []*domainCode.AiCommit{
			{ProjectName: "p1"},
		})
		assert.Nil(t, err)
	})

	t.Run("empty batch", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		err := saveDomainAiCommitBatch(mockDal, []*domainCode.AiCommit{})
		assert.Nil(t, err)
	})

	t.Run("error on save", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))
		err := saveDomainAiCommitBatch(mockDal, []*domainCode.AiCommit{{ProjectName: "p1"}})
		assert.NotNil(t, err)
	})
}
