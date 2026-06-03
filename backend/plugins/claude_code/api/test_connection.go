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
	gocontext "context"
	"net/http"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/claude_code/models"
	"github.com/apache/incubator-devlake/plugins/claude_code/service"
)

// TestConnection validates a Claude Code connection before saving it.
func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.ClaudeCodeConnection{}
	if err := helper.Decode(input.Body, connection, vld); err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}

	connection.Normalize()
	if err := validateConnection(connection); err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}

	result, err := service.TestConnection(gocontext.Background(), basicRes, connection)
	if err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}
	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}

// TestExistingConnection validates a stored Claude Code connection with optional overrides.
func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &models.ClaudeCodeConnection{}
	if err := connectionHelper.First(connection, input.Params); err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, errors.BadInput.Wrap(err, "find connection from db"))
	}
	if err := (&models.ClaudeCodeConnection{}).MergeFromRequest(connection, input.Body); err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, errors.Convert(err))
	}

	connection.Normalize()
	if err := validateConnection(connection); err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}

	result, err := service.TestConnection(gocontext.Background(), basicRes, connection)
	if err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}
	return &plugin.ApiResourceOutput{Body: result, Status: http.StatusOK}, nil
}
