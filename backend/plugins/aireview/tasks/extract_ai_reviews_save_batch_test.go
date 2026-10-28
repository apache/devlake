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
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSaveBatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

		batch := []*models.AiReview{
			{Id: "r1", PullRequestId: "pr-1"},
			{Id: "r2", PullRequestId: "pr-2"},
		}
		err := saveBatch(mockDal, batch)
		assert.Nil(t, err)
		mockDal.AssertNumberOfCalls(t, "CreateOrUpdate", 2)
	})

	t.Run("error on save", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		batch := []*models.AiReview{{Id: "r1"}}
		err := saveBatch(mockDal, batch)
		assert.NotNil(t, err)
	})

	t.Run("empty batch", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		err := saveBatch(mockDal, []*models.AiReview{})
		assert.Nil(t, err)
	})
}
