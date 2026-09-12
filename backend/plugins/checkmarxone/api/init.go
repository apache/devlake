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

package api

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/checkmarxone/models"
	"github.com/go-playground/validator/v10"
)

var vld *validator.Validate
var basicRes context.BasicRes
var dsHelper *api.DsHelper[models.CheckmarxoneConnection, models.CheckmarxoneProject, srvhelper.NoScopeConfig]

// Init initialises the api package variables
func Init(br context.BasicRes, p plugin.PluginMeta) {
	basicRes = br
	vld = validator.New()
	dsHelper = api.NewDataSourceHelper[
		models.CheckmarxoneConnection,
		models.CheckmarxoneProject,
		srvhelper.NoScopeConfig,
	](
		br,
		p.Name(),
		[]string{"name"},
		func(c models.CheckmarxoneConnection) models.CheckmarxoneConnection {
			return c.Sanitize()
		},
		nil,
		nil,
	)
}