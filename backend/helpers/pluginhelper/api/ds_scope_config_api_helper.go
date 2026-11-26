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
	"fmt"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/srvhelper"
	"github.com/apache/devlake/server/api/shared"
)

// DsScopeConfigApiHelper
type DsScopeConfigApiHelper[C plugin.ToolLayerConnection, S plugin.ToolLayerScope, SC plugin.ToolLayerScopeConfig] struct {
	*ModelApiHelper[SC]
	*srvhelper.ScopeConfigSrvHelper[C, S, SC]
}

func NewDsScopeConfigApiHelper[
	C plugin.ToolLayerConnection,
	S plugin.ToolLayerScope,
	SC plugin.ToolLayerScopeConfig,
](
	basicRes context.BasicRes,
	dalHelper *srvhelper.ScopeConfigSrvHelper[C, S, SC],
	sterilizer func(sc SC) SC,
) *DsScopeConfigApiHelper[C, S, SC] {
	return &DsScopeConfigApiHelper[C, S, SC]{
		ModelApiHelper:       NewModelApiHelper[SC](basicRes, dalHelper.ModelSrvHelper, []string{"scopeConfigId"}, sterilizer),
		ScopeConfigSrvHelper: dalHelper,
	}
}

func (connApi *DsScopeConfigApiHelper[C, S, SC]) GetAll(input *plugin.ApiResourceInput) (out *plugin.ApiResourceOutput, err errors.Error) {
	connectionId, err := extractConnectionId(input)
	if err != nil {
		return nil, err
	}
	scopeConfigs := errors.Must1(connApi.ScopeConfigSrvHelper.GetAllByConnectionId(connectionId))
	return &plugin.ApiResourceOutput{
		Body: scopeConfigs,
	}, nil
}

func (connApi *DsScopeConfigApiHelper[C, S, SC]) GetProjectsByScopeConfig(input *plugin.ApiResourceInput) (out *plugin.ApiResourceOutput, err errors.Error) {
	var scopeConfig *SC
	scopeConfig, err = connApi.FindByPk(input)
	if err != nil {
		return nil, err
	}
	projectDetails := errors.Must1(connApi.ScopeConfigSrvHelper.GetProjectsByScopeConfig(input.Params["plugin"], scopeConfig))
	return &plugin.ApiResourceOutput{
		Body: projectDetails,
	}, nil
}

func (connApi *DsScopeConfigApiHelper[C, S, SC]) Post(input *plugin.ApiResourceInput) (out *plugin.ApiResourceOutput, err errors.Error) {
	// fix connectionId
	connectionId, err := extractConnectionId(input)
	if err != nil {
		return nil, err
	}
	input.Body["connectionId"] = connectionId
	return connApi.ModelApiHelper.Post(input)
}

func (connApi *DsScopeConfigApiHelper[C, S, SC]) Patch(input *plugin.ApiResourceInput) (out *plugin.ApiResourceOutput, err errors.Error) {
	connectionId, err := extractConnectionId(input)
	if err != nil {
		return nil, err
	}
	// The scope config is looked up by its own id, so the connection in the URL is not
	// implied by the lookup. Verify the ownership before writing anything, otherwise a
	// PATCH aimed at the wrong connection would silently reassign the scope config to it.
	var scopeConfig *SC
	scopeConfig, err = connApi.FindByPk(input)
	if err != nil {
		return nil, err
	}
	if (*scopeConfig).ScopeConfigConnectionId() != connectionId {
		return nil, errors.NotFound.New(fmt.Sprintf(
			"scope config %d does not belong to connection %d",
			(*scopeConfig).ScopeConfigId(), connectionId))
	}
	// fix connectionId
	input.Body["connectionId"] = connectionId
	return connApi.ModelApiHelper.Patch(input)
}

func (connApi *DsScopeConfigApiHelper[C, S, SC]) Delete(input *plugin.ApiResourceInput) (out *plugin.ApiResourceOutput, err errors.Error) {
	var scopeConfig *SC
	scopeConfig, err = connApi.FindByPk(input)
	if err != nil {
		return nil, err
	}
	refs, err := connApi.ScopeConfigSrvHelper.DeleteScopeConfig(scopeConfig)
	if err != nil {
		return &plugin.ApiResourceOutput{Body: &shared.ApiBody{
			Success: false,
			Message: err.Error(),
			Data:    refs,
		}, Status: err.GetType().GetHttpCode()}, err
	}
	return &plugin.ApiResourceOutput{
		Body: scopeConfig,
	}, nil
}
