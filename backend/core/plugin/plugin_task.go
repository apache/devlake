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

package plugin

import (
	"context"

	corecontext "github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
)

type ProgressType int

const (
	TaskSetProgress ProgressType = iota
	TaskIncProgress
	SubTaskSetProgress
	SubTaskIncProgress
	SetCurrentSubTask
)

type RunningProgress struct {
	Type                 ProgressType
	Current              int
	Total                int
	SubTaskName          string
	SubTaskNumber        int
	CollectSubtaskNumber int
	OtherSubtaskNumber   int
} // nolint

// ExecContext This interface define all resources that needed for task/subtask execution
type ExecContext interface {
	corecontext.BasicRes
	GetName() string
	GetContext() context.Context
	GetData() interface{}
	SetProgress(current int, total int)
	IncProgress(quantity int)
	GetProgress() int
}

// SubTaskContext This interface define all resources that needed for subtask execution
type SubTaskContext interface {
	ExecContext
	TaskContext() TaskContext
}

// TaskContext This interface define all resources that needed for task execution
type TaskContext interface {
	ExecContext
	SetData(data interface{})
	SetSyncPolicy(syncPolicy *models.SyncPolicy)
	SyncPolicy() *models.SyncPolicy
	SubTaskContext(subtask string) (SubTaskContext, errors.Error)
}

type SubTask interface {
	// Execute FIXME ...
	Execute() errors.Error
} // nolint

// SubTaskEntryPoint All subtasks from plugins should comply to this prototype, so they could be orchestrated by framework
type SubTaskEntryPoint func(c SubTaskContext) errors.Error

const DOMAIN_TYPE_CODE = "CODE"                //nolint
const DOMAIN_TYPE_TICKET = "TICKET"            //nolint
const DOMAIN_TYPE_CODE_REVIEW = "CODEREVIEW"   //nolint
const DOMAIN_TYPE_CROSS = "CROSS"              //nolint
const DOMAIN_TYPE_CICD = "CICD"                //nolint
const DOMAIN_TYPE_CODE_QUALITY = "CODEQUALITY" //nolint

var DOMAIN_TYPES = []string{
	DOMAIN_TYPE_CODE,
	DOMAIN_TYPE_TICKET,
	DOMAIN_TYPE_CODE_REVIEW,
	DOMAIN_TYPE_CROSS,
	DOMAIN_TYPE_CICD,
	DOMAIN_TYPE_CODE_QUALITY,
} //nolint

// SubTaskMeta Metadata of a subtask
type SubTaskMeta struct {
	Name       string
	EntryPoint SubTaskEntryPoint
	// Required SubTask will be executed no matter what
	Required         bool
	EnabledByDefault bool
	Description      string
	DomainTypes      []string
	Dependencies     []*SubTaskMeta
	DependencyTables []string
	ProductTables    []string
	ForceRunOnResume bool // Should a subtask be ran despite it was finished before
}

// PluginTask Implement this interface to let framework run tasks for you
type PluginTask interface {
	// SubTaskMetas return all available subtasks, framework will run them for you in order
	SubTaskMetas() []SubTaskMeta
	// PrepareTaskData based on task context and user input options, return data that shared among all subtasks
	PrepareTaskData(taskCtx TaskContext, options map[string]interface{}) (interface{}, errors.Error)
}

// CloseablePluginTask Extends PluginTask, and invokes a Close method after all subtasks are done or fail
type CloseablePluginTask interface {
	PluginTask
	Close(taskCtx TaskContext) errors.Error
}
