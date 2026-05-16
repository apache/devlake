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

package impl

import (
	"fmt"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/dbt/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginTask
	plugin.CloseablePluginTask
	plugin.PluginModel
} = (*Dbt)(nil)

type Dbt struct{}

func (p Dbt) Description() string {
	return "Convert data by dbt"
}

func (p Dbt) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.GitMeta,
		tasks.DbtConverterMeta,
	}
}

func (p Dbt) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{}
}

func (p Dbt) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.DbtOptions
	err := helper.Decode(options, &op, nil)
	if err != nil {
		return nil, err
	}
	if err := tasks.PrepareOptions(&op, taskCtx.GetConfig(tasks.DbtProjectBaseDirConfigKey)); err != nil {
		return nil, err
	}

	if op.ProjectTarget == "" {
		op.ProjectTarget = "dev"
	}

	return &tasks.DbtTaskData{
		Options: &op,
	}, nil
}

func (p Dbt) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/dbt"
}

func (p Dbt) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.DbtTaskData)
	if !ok || data == nil || data.Options == nil || !data.Options.ManagedProjectDir {
		return nil
	}
	if err := tasks.CleanupManagedProjectDir(data.Options); err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("cleanup dbt project path %q", data.Options.ProjectPath))
	}
	return nil
}

func (p Dbt) Name() string {
	return "dbt"
}
