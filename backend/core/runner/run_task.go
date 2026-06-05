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

package runner

import (
	gocontext "context"
	"fmt"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/models/common"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/utils"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	contextimpl "github.com/apache/incubator-devlake/impls/context"
	"github.com/apache/incubator-devlake/impls/logruslog"
)

// RunTask FIXME ...
func RunTask(
	ctx gocontext.Context,
	basicRes context.BasicRes,
	progress chan plugin.RunningProgress,
	taskId uint64,
) (err errors.Error) {
	db := basicRes.GetDal()
	task := &models.Task{}
	if err := db.First(task, dal.Where("id = ?", taskId)); err != nil {
		return err
	}
	dbPipeline := &models.Pipeline{}
	if err := db.First(dbPipeline, dal.Where("id = ? ", task.PipelineId)); err != nil {
		return err
	}

	logger, err := getTaskLogger(basicRes.GetLogger(), task)
	if err != nil {
		return err
	}
	beganAt := time.Now()
	if task.BeganAt != nil {
		beganAt = *task.BeganAt
	}
	// make sure task status always correct even if it panicked
	defer func() {
		if r := recover(); r != nil {
			var e error
			switch et := r.(type) {
			case error:
				e = et
			default:
				e = fmt.Errorf("%v", et)
			}
			err = errors.Default.Wrap(e, fmt.Sprintf("run task failed with panic (%s)", utils.GatherCallFrames(0)))
			logger.Error(err, "run task failed with panic")
		}
		finishedAt := time.Now()
		spentSeconds := finishedAt.Unix() - beganAt.Unix()
		isCancelled := errors.Is(err, gocontext.Canceled) || ctx.Err() == gocontext.Canceled
		if err != nil {
			taskStatus := models.TASK_FAILED
			if isCancelled {
				taskStatus = models.TASK_CANCELLED
			}
			lakeErr := errors.AsLakeErrorType(err)
			subTaskName := "unknown"
			if lakeErr = lakeErr.As(errors.SubtaskErr); lakeErr != nil {
				if meta, ok := lakeErr.GetData().(*plugin.SubTaskMeta); ok {
					subTaskName = meta.Name
				}
			} else {
				lakeErr = errors.Convert(err)
			}
			columns := []dal.DalSet{
				{ColumnName: "status", Value: taskStatus},
				{ColumnName: "message", Value: lakeErr.Error()},
				{ColumnName: "error_name", Value: lakeErr.Messages().Format()},
				{ColumnName: "finished_at", Value: finishedAt},
				{ColumnName: "spent_seconds", Value: spentSeconds},
			}
			if taskStatus == models.TASK_FAILED {
				columns = append(columns, dal.DalSet{ColumnName: "failed_sub_task", Value: subTaskName})
			}
			dbe := db.UpdateColumns(task, columns)
			if dbe != nil {
				logger.Error(dbe, "failed to finalize task status into db (task %s)", taskStatus)
			}
		} else {
			dbe := db.UpdateColumns(task, []dal.DalSet{
				{ColumnName: "status", Value: models.TASK_COMPLETED},
				{ColumnName: "message", Value: ""},
				{ColumnName: "finished_at", Value: finishedAt},
				{ColumnName: "spent_seconds", Value: spentSeconds},
			})
			if dbe != nil {
				logger.Error(dbe, "failed to finalize task status into db (task succeeded)")
			}
		}
		// update finishedTasks
		errors.Must(db.UpdateColumn(
			&models.Pipeline{},
			"finished_tasks", dal.Expr("finished_tasks + 1"),
			dal.Where("id=?", task.PipelineId),
		))
		// not return err if the `SkipOnFail` is true and the error is not canceled
		if dbPipeline.SkipOnFail && !isCancelled {
			err = nil
		}
	}()

	if task.Status == models.TASK_COMPLETED {
		return nil
	}

	// start execution
	logger.Info("start executing task: %d", task.ID)
	dbe := db.UpdateColumns(task, []dal.DalSet{
		{ColumnName: "status", Value: models.TASK_RUNNING},
		{ColumnName: "message", Value: ""},
		{ColumnName: "began_at", Value: beganAt},
	})
	if dbe != nil {
		return dbe
	}

	err = RunPluginTask(
		ctx,
		basicRes.ReplaceLogger(logger),
		task,
		progress,
		&dbPipeline.SyncPolicy,
	)
	return err
}

// RunPluginTask FIXME ...
func RunPluginTask(
	ctx gocontext.Context,
	basicRes context.BasicRes,
	task *models.Task,
	progress chan plugin.RunningProgress,
	syncPolicy *models.SyncPolicy,
) errors.Error {
	pluginMeta, err := plugin.GetPlugin(task.Plugin)
	if err != nil {
		return errors.Default.WrapRaw(err)
	}
	pluginTask, ok := pluginMeta.(plugin.PluginTask)
	if !ok {
		return errors.Default.New(fmt.Sprintf("plugin %s doesn't support PluginTask interface", task.Plugin))
	}
	return RunPluginSubTasks(
		ctx,
		basicRes,
		task,
		pluginTask,
		progress,
		syncPolicy,
	)
}

// RunPluginSubTasks FIXME ...
func RunPluginSubTasks(
	ctx gocontext.Context,
	basicRes context.BasicRes,
	task *models.Task,
	pluginTask plugin.PluginTask,
	progress chan plugin.RunningProgress,
	syncPolicy *models.SyncPolicy,
) errors.Error {
	logger := basicRes.GetLogger()
	logger.Info("start plugin")
	// find out all possible subtasks this plugin can offer
	subtaskMetas := pluginTask.SubTaskMetas()
	subtasksFlag := make(map[string]bool)
	for _, subtaskMeta := range subtaskMetas {
		subtasksFlag[subtaskMeta.Name] = subtaskMeta.EnabledByDefault
	}
	/* subtasksFlag example
	subtasksFlag := map[string]bool{
		"collectProject": true,
		"convertCommits": true,
		...
	}
	*/

	// user specifies what subtasks to run
	if len(task.Subtasks) != 0 {
		// decode user specified subtasks
		var specifiedTasks []string
		err := api.Decode(task.Subtasks, &specifiedTasks, nil)
		if err != nil {
			return errors.Default.Wrap(err, "subtasks could not be decoded")
		}
		if len(specifiedTasks) > 0 {
			// first, disable all subtasks
			for task := range subtasksFlag {
				subtasksFlag[task] = false
			}
			// second, check specified subtasks is valid and enable them if so
			for _, task := range specifiedTasks {
				if _, ok := subtasksFlag[task]; ok {
					subtasksFlag[task] = true
				} else {
					return errors.Default.New(fmt.Sprintf("subtask %s does not exist", task))
				}
			}
		}
	}

	// 1. make sure `Collect` subtasks skip if `SkipCollectors` is true
	// 2. make sure `Required` subtasks are always enabled
	for _, subtaskMeta := range subtaskMetas {
		if syncPolicy != nil && syncPolicy.SkipCollectors && strings.Contains(strings.ToLower(subtaskMeta.Name), "collect") {
			subtasksFlag[subtaskMeta.Name] = false
		}
		if subtaskMeta.Required {
			subtasksFlag[subtaskMeta.Name] = true
		}
	}

	// calculate total step(number of task to run)
	steps := 0
	for _, enabled := range subtasksFlag {
		if enabled {
			steps++
		}
	}

	taskCtx := contextimpl.NewDefaultTaskContext(ctx, basicRes, task.Plugin, subtasksFlag, progress)
	if closeablePlugin, ok := pluginTask.(plugin.CloseablePluginTask); ok {
		defer closeablePlugin.Close(taskCtx)
	}
	options := task.Options
	taskData, err := pluginTask.PrepareTaskData(taskCtx, options)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("error preparing task data for %s", task.Plugin))
	}
	taskCtx.SetSyncPolicy(syncPolicy)
	taskCtx.SetData(taskData)

	// record subtasks sequence to DB
	collectSubtaskNumber := 0
	otherSubtaskNumber := 0
	isCollector := false
	subtask := []models.Subtask{}
	for _, subtaskMeta := range subtaskMetas {
		subtaskCtx, err := taskCtx.SubTaskContext(subtaskMeta.Name)
		if err != nil {
			// sth went wrong
			return errors.Default.Wrap(err, fmt.Sprintf("error getting context subtask %s", subtaskMeta.Name))
		}
		if subtaskCtx == nil {
			// subtask was disabled
			continue
		}
		if strings.Contains(strings.ToLower(subtaskMeta.Name), "collect") || strings.Contains(strings.ToLower(subtaskMeta.Name), "clone git repo") {
			collectSubtaskNumber++
			isCollector = true
		} else {
			otherSubtaskNumber++
			isCollector = false
		}
		s := models.Subtask{
			Name:        subtaskCtx.GetName(),
			TaskID:      task.ID,
			IsCollector: isCollector,
		}
		if isCollector {
			s.Sequence = collectSubtaskNumber
		} else {
			s.Sequence = otherSubtaskNumber
		}
		subtask = append(subtask, s)
	}
	if err := basicRes.GetDal().CreateOrUpdate(subtask); err != nil {
		basicRes.GetLogger().Error(err, "error writing subtask list to DB")
	}

	// execute subtasks in order
	taskCtx.SetProgress(0, steps)
	subtaskNumber := 0
	for _, subtaskMeta := range subtaskMetas {
		subtaskCtx, err := taskCtx.SubTaskContext(subtaskMeta.Name)
		if err != nil {
			// sth went wrong
			return errors.Default.Wrap(err, fmt.Sprintf("error getting context subtask %s", subtaskMeta.Name))
		}
		subtaskNumber++
		if subtaskCtx == nil {
			// subtask was disabled
			continue
		}
		// run subtask
		if progress != nil {
			progress <- plugin.RunningProgress{
				Type:          plugin.SetCurrentSubTask,
				SubTaskName:   subtaskMeta.Name,
				SubTaskNumber: subtaskNumber,
			}
		}
		subtaskFinished := false
		if !subtaskMeta.ForceRunOnResume {
			if task.ID > 0 {
				sfc := errors.Must1(basicRes.GetDal().Count(
					dal.From(&models.Subtask{}), dal.Where("task_id = ? AND name = ? AND finished_at IS NOT NULL", task.ID, subtaskMeta.Name),
				),
				)
				subtaskFinished = sfc > 0
			}
		}
		if subtaskFinished {
			logger.Info("subtask %s already finished previously", subtaskMeta.Name)
		} else {
			logger.Info("executing subtask %s", subtaskMeta.Name)
			start := time.Now()
			err = runSubtask(basicRes, subtaskCtx, task.ID, subtaskNumber, subtaskMeta.EntryPoint)
			logger.Info("subtask %s finished in %d ms, records processed: %d", subtaskMeta.Name, time.Since(start).Milliseconds(), subtaskCtx.GetProgress())
			if err != nil {
				err = errors.SubtaskErr.Wrap(err, fmt.Sprintf("subtask %s ended unexpectedly", subtaskMeta.Name), errors.WithData(&subtaskMeta))
				logger.Error(err, "")
				where := dal.Where("task_id = ? and name = ?", task.ID, subtaskCtx.GetName())
				if err := basicRes.GetDal().UpdateColumns(subtask, []dal.DalSet{
					{ColumnName: "is_failed", Value: true},
					{ColumnName: "message", Value: err.Error()},
				}, where); err != nil {
					basicRes.GetLogger().Error(err, "error writing subtask %v status to DB", subtaskCtx.GetName())
				}
				return err
			}
		}
		taskCtx.IncProgress(1)
	}

	return nil
}

// UpdateProgressDetail FIXME ...
func UpdateProgressDetail(basicRes context.BasicRes, taskId uint64, progressDetail *models.TaskProgressDetail, p *plugin.RunningProgress) {
	cfg := basicRes.GetConfigReader()
	skipSubtaskProgressUpdate := cfg.GetBool("SKIP_SUBTASK_PROGRESS")

	task := &models.Task{
		Model: common.Model{ID: taskId},
	}
	subtask := &models.Subtask{}
	originalFinishedRecords := progressDetail.FinishedRecords
	switch p.Type {
	case plugin.TaskSetProgress:
		progressDetail.TotalSubTasks = p.Total
		progressDetail.FinishedSubTasks = p.Current
	case plugin.TaskIncProgress:
		progressDetail.FinishedSubTasks = p.Current
		// TODO: get rid of db update
		pct := float32(p.Current) / float32(p.Total)
		err := basicRes.GetDal().UpdateColumn(task, "progress", pct)
		if err != nil {
			basicRes.GetLogger().Error(err, "failed to update progress")
		}
	case plugin.SubTaskSetProgress:
		progressDetail.TotalRecords = p.Total
	case plugin.SubTaskIncProgress:
		progressDetail.FinishedRecords = p.Current
	case plugin.SetCurrentSubTask:
		progressDetail.SubTaskName = p.SubTaskName
		progressDetail.SubTaskNumber = p.SubTaskNumber
		// reset finished records
		progressDetail.FinishedRecords = 0
	}
	if skipSubtaskProgressUpdate {
		return
	}
	currentFinishedRecords := progressDetail.FinishedRecords
	currentTotalRecords := progressDetail.TotalRecords
	// update progress if progress is more than 1%
	// or there is progress if no total record provided
	if (currentTotalRecords > 0 && float64(currentFinishedRecords-originalFinishedRecords)/float64(currentTotalRecords) > 0.01) || (currentTotalRecords <= 0 && currentFinishedRecords > originalFinishedRecords) {
		// update subtask progress
		where := dal.Where("task_id = ? and name = ?", taskId, progressDetail.SubTaskName)
		err := basicRes.GetDal().UpdateColumns(subtask, []dal.DalSet{
			{ColumnName: "finished_records", Value: progressDetail.FinishedRecords},
		}, where)
		if err != nil {
			basicRes.GetLogger().Error(err, "failed to update _devlake_subtasks progress")
		}
	}
}

func runSubtask(
	basicRes context.BasicRes,
	ctx plugin.SubTaskContext,
	parentID uint64,
	subtaskNumber int,
	entryPoint plugin.SubTaskEntryPoint,
) errors.Error {
	beginAt := time.Now()
	subtask := &models.Subtask{
		Name:    ctx.GetName(),
		TaskID:  parentID,
		Number:  subtaskNumber,
		BeganAt: &beginAt,
	}
	recordSubtask(basicRes, subtask)
	// defer to record subtask status
	defer func() {
		finishedAt := time.Now()
		subtask.FinishedAt = &finishedAt
		subtask.SpentSeconds = finishedAt.Unix() - beginAt.Unix()

		recordSubtask(basicRes, subtask)
	}()
	return entryPoint(ctx)
}

func recordSubtask(basicRes context.BasicRes, subtask *models.Subtask) {
	where := dal.Where("task_id = ? and name = ?", subtask.TaskID, subtask.Name)
	if err := basicRes.GetDal().UpdateColumns(subtask, []dal.DalSet{
		{ColumnName: "began_at", Value: subtask.BeganAt},
		{ColumnName: "finished_at", Value: subtask.FinishedAt},
		{ColumnName: "spent_seconds", Value: subtask.SpentSeconds},
		//{ColumnName: "finished_records", Value: subtask.FinishedRecords}, // FinishedRecords is zero always.
		{ColumnName: "number", Value: subtask.Number},
	}, where); err != nil {
		basicRes.GetLogger().Error(err, "error writing subtask %d status to DB: %v", subtask.ID)
	}
}

func getTaskLogger(parentLogger log.Logger, task *models.Task) (log.Logger, errors.Error) {
	logger := parentLogger.Nested(fmt.Sprintf("task #%d", task.ID))
	loggingPath := logruslog.GetTaskLoggerPath(logger.GetConfig(), task)
	stream, err := logruslog.GetFileStream(loggingPath)
	if err != nil {
		return nil, err
	}
	logger.SetStream(&log.LoggerStreamConfig{
		Path:   loggingPath,
		Writer: stream,
	})
	return logger, nil
}
