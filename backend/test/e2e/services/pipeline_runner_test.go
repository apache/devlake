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

package services

import (
	"testing"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/server/services"
	"github.com/apache/incubator-devlake/test/helper"
	"github.com/stretchr/testify/assert"
)

func TestComputePipelineStatus(t *testing.T) {
	client := helper.StartDevLakeServer(t, nil)
	db := client.GetDal()
	// insert fake tasks to database
	pipeline := &models.Pipeline{
		TotalTasks: 3,
	}
	err := db.Create(pipeline)
	assert.Nil(t, err)
	assert.NotZero(t, pipeline.ID)

	task_row1_col1 := &models.Task{
		PipelineId:  pipeline.ID,
		PipelineRow: 1,
		PipelineCol: 1,
		Plugin:      "github",
		Status:      models.TASK_COMPLETED,
	}
	err = db.Create(task_row1_col1)
	assert.Nil(t, err)
	assert.NotZero(t, task_row1_col1.ID)

	task_row1_col2 := &models.Task{
		PipelineId:  pipeline.ID,
		PipelineRow: 1,
		PipelineCol: 2,
		Plugin:      "gitext",
		Status:      models.TASK_FAILED,
	}
	err = db.Create(task_row1_col2)
	assert.Nil(t, err)
	assert.NotZero(t, task_row1_col2.ID)

	task_row2_col1 := &models.Task{
		PipelineId:  pipeline.ID,
		PipelineRow: 2,
		PipelineCol: 1,
		Plugin:      "refdiff",
		Status:      models.TASK_CREATED,
	}
	err = db.Create(task_row2_col1)
	assert.Nil(t, err)
	assert.NotZero(t, task_row2_col1.ID)

	// pipeline.status == "failed" if SkipOnFailed=false and any tasks failed
	status, err := services.ComputePipelineStatus(pipeline, false)
	if !assert.Nil(t, err) {
		println(err.Messages().Format())
	}
	assert.Equal(t, models.TASK_FAILED, status)

	// pipeline.status == "completed" if all latest tasks were succeeded
	task_row1_col2.Status = models.TASK_COMPLETED
	err = db.Update(task_row1_col2)
	assert.Nil(t, err)
	task_row2_col1.Status = models.TASK_COMPLETED
	err = db.Update(task_row2_col1)
	assert.Nil(t, err)
	status, err = services.ComputePipelineStatus(pipeline, false)
	if !assert.Nil(t, err) {
		println(err.Messages().Format())
	}
	assert.Equal(t, models.TASK_COMPLETED, status)

	pipeline.SkipOnFail = true
	err = db.Update(pipeline)
	assert.Nil(t, err)
	status, err = services.ComputePipelineStatus(pipeline, false)
	assert.Nil(t, err)
	assert.Equal(t, models.TASK_COMPLETED, status)

	// pipeline.status == "partial" if SkipOnFail=true and some were succeeded while others not
	task_row1_col1.Status = models.TASK_FAILED
	err = db.Update(task_row1_col1)
	assert.Nil(t, err)
	status, err = services.ComputePipelineStatus(pipeline, false)
	assert.Nil(t, err)
	assert.Equal(t, models.TASK_PARTIAL, status)

	// pipeline.status == "failed" is SkipOnFail=true and all tasks were fail
	task_row1_col1.Status = models.TASK_FAILED
	err = db.Update(task_row1_col1)
	assert.Nil(t, err)
	task_row1_col2.Status = models.TASK_FAILED
	err = db.Update(task_row1_col2)
	assert.Nil(t, err)
	task_row2_col1.Status = models.TASK_FAILED
	err = db.Update(task_row2_col1)
	assert.Nil(t, err)
	status, err = services.ComputePipelineStatus(pipeline, false)
	assert.Nil(t, err)
	assert.Equal(t, models.TASK_FAILED, status)

	// pipeline.status == "completed" if all failed tasks were reran successfully
	task_row1_col1_rerun := &models.Task{
		PipelineId:  pipeline.ID,
		PipelineRow: 1,
		PipelineCol: 1,
		Plugin:      "github",
		Status:      models.TASK_COMPLETED,
	}
	err = db.Create(task_row1_col1_rerun)
	assert.Nil(t, err)
	assert.NotZero(t, task_row1_col1_rerun.ID)

	task_row1_col2_rerun := &models.Task{
		PipelineId:  pipeline.ID,
		PipelineRow: 1,
		PipelineCol: 2,
		Plugin:      "gitext",
		Status:      models.TASK_COMPLETED,
	}
	err = db.Create(task_row1_col2_rerun)
	assert.Nil(t, err)
	assert.NotZero(t, task_row1_col2_rerun.ID)

	task_row2_col1_rerun := &models.Task{
		PipelineId:  pipeline.ID,
		PipelineRow: 2,
		PipelineCol: 1,
		Plugin:      "refdiff",
		Status:      models.TASK_COMPLETED,
	}
	err = db.Create(task_row2_col1_rerun)
	assert.Nil(t, err)
	assert.NotZero(t, task_row2_col1.ID)

	status, err = services.ComputePipelineStatus(pipeline, false)
	assert.Nil(t, err)
	assert.Equal(t, models.TASK_COMPLETED, status)

	// pipeline.status == "partial" if there were failed task in reran tasks
	task_row1_col1_rerun.Status = models.TASK_CANCELLED
	err = db.Update(task_row1_col1_rerun)
	assert.Nil(t, err)
	status, err = services.ComputePipelineStatus(pipeline, false)
	assert.Nil(t, err)
	assert.Equal(t, models.TASK_PARTIAL, status)

	// pipeline.status == "cancelled" if the pipeline was cancelled by the user
	// regardless of individual task statuses
	task_row1_col1_rerun.Status = models.TASK_COMPLETED
	err = db.Update(task_row1_col1_rerun)
	assert.Nil(t, err)
	status, err = services.ComputePipelineStatus(pipeline, true)
	assert.Nil(t, err)
	assert.Equal(t, models.TASK_CANCELLED, status)

	// pipeline.status == "cancelled" even when some tasks failed
	task_row1_col1_rerun.Status = models.TASK_FAILED
	err = db.Update(task_row1_col1_rerun)
	assert.Nil(t, err)
	status, err = services.ComputePipelineStatus(pipeline, true)
	assert.Nil(t, err)
	assert.Equal(t, models.TASK_CANCELLED, status)
}

func TestCancelPipeline(t *testing.T) {
	client := helper.StartDevLakeServer(t, nil)
	db := client.GetDal()

	t.Run("cancels pending pipeline and all its tasks", func(t *testing.T) {
		pipeline := &models.Pipeline{
			TotalTasks: 2,
			Status:     models.TASK_CREATED,
		}
		err := db.Create(pipeline)
		assert.Nil(t, err)
		assert.NotZero(t, pipeline.ID)

		task1 := &models.Task{
			PipelineId:  pipeline.ID,
			PipelineRow: 1,
			PipelineCol: 1,
			Plugin:      "github",
			Status:      models.TASK_CREATED,
		}
		task2 := &models.Task{
			PipelineId:  pipeline.ID,
			PipelineRow: 1,
			PipelineCol: 2,
			Plugin:      "gitextractor",
			Status:      models.TASK_CREATED,
		}
		err = db.Create(task1)
		assert.Nil(t, err)
		assert.NotZero(t, task1.ID)
		err = db.Create(task2)
		assert.Nil(t, err)
		assert.NotZero(t, task2.ID)

		err = services.CancelPipeline(pipeline.ID)
		assert.Nil(t, err)

		cancelledPipeline := &models.Pipeline{}
		err = db.First(cancelledPipeline, dal.Where("id = ?", pipeline.ID))
		assert.Nil(t, err)
		assert.Equal(t, models.TASK_CANCELLED, cancelledPipeline.Status)

		cancelledTask1, err := services.GetTask(task1.ID)
		assert.Nil(t, err)
		assert.Equal(t, models.TASK_CANCELLED, cancelledTask1.Status)
		cancelledTask2, err := services.GetTask(task2.ID)
		assert.Nil(t, err)
		assert.Equal(t, models.TASK_CANCELLED, cancelledTask2.Status)
	})

	t.Run("cancels pending tasks but leaves completed tasks unchanged", func(t *testing.T) {
		pipeline := &models.Pipeline{
			TotalTasks: 2,
			Status:     models.TASK_RUNNING,
		}
		err := db.Create(pipeline)
		assert.Nil(t, err)
		assert.NotZero(t, pipeline.ID)

		finishedAt := time.Now()
		completedTask := &models.Task{
			PipelineId:  pipeline.ID,
			PipelineRow: 1,
			PipelineCol: 1,
			Plugin:      "github",
			Status:      models.TASK_COMPLETED,
			FinishedAt:  &finishedAt,
		}
		pendingTask := &models.Task{
			PipelineId:  pipeline.ID,
			PipelineRow: 2,
			PipelineCol: 1,
			Plugin:      "refdiff",
			Status:      models.TASK_CREATED,
		}
		err = db.Create(completedTask)
		assert.Nil(t, err)
		assert.NotZero(t, completedTask.ID)
		err = db.Create(pendingTask)
		assert.Nil(t, err)
		assert.NotZero(t, pendingTask.ID)

		err = services.CancelPipeline(pipeline.ID)
		assert.Nil(t, err)

		reloadedCompleted, err := services.GetTask(completedTask.ID)
		assert.Nil(t, err)
		assert.Equal(t, models.TASK_COMPLETED, reloadedCompleted.Status)

		reloadedPending, err := services.GetTask(pendingTask.ID)
		assert.Nil(t, err)
		assert.Equal(t, models.TASK_CANCELLED, reloadedPending.Status)
	})

	t.Run("returns error for non-existent pipeline", func(t *testing.T) {
		err := services.CancelPipeline(999999)
		assert.NotNil(t, err)
	})
}
