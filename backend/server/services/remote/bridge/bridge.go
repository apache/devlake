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

package bridge

import (
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/server/services/remote/models"
)

type (
	Bridge struct {
		invoker Invoker
	}
	Invoker interface {
		Call(methodName string, ctx plugin.ExecContext, args ...any) *CallResult
		Stream(methodName string, ctx plugin.ExecContext, args ...any) *MethodStream
	}
)

func NewBridge(invoker Invoker) *Bridge {
	return &Bridge{invoker: invoker}
}

func enrichRemoteTaskData(ctx plugin.SubTaskContext) interface{} {
	data := ctx.GetData()
	taskData, ok := data.(models.RemotePluginTaskData)
	if !ok {
		return data
	}
	syncPolicy := ctx.TaskContext().SyncPolicy()
	if syncPolicy == nil {
		syncPolicy = &coreModels.SyncPolicy{}
	}
	models.ApplySyncPolicy(&taskData, syncPolicy)
	return taskData
}

func (b *Bridge) RemoteSubtaskEntrypointHandler(subtaskMeta models.SubtaskMeta) plugin.SubTaskEntryPoint {
	return func(ctx plugin.SubTaskContext) errors.Error {
		args := []interface{}{enrichRemoteTaskData(ctx)}
		for _, arg := range subtaskMeta.Arguments {
			args = append(args, arg)
		}
		stream := b.invoker.Stream(subtaskMeta.EntryPointName, NewChildRemoteContext(ctx), args...)
		for recv := range stream.Receive() {
			if recv.Err != nil {
				return recv.Err
			}
			progress := RemoteProgress{}
			err := recv.Get(&progress)
			if err != nil {
				return err
			}
			if progress.Total != 0 {
				ctx.SetProgress(progress.Current, progress.Total)
			} else if progress.Increment != 0 {
				ctx.IncProgress(progress.Increment)
			}
		}
		return nil
	}
}
