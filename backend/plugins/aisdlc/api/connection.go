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
	"fmt"
	"net/http"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/aisdlc/models"
	"github.com/apache/devlake/plugins/aisdlc/tasks"
)

func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Post(input)
}

func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Patch(input)
}

func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}

func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var conn models.AisdlcConnection
	if err := api.Decode(input.Body, &conn, nil); err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode connection")
	}
	return testConnection(gocontext.Background(), &conn)
}

func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.Convert(err)
	}
	return testConnection(gocontext.Background(), connection)
}

func testConnection(ctx gocontext.Context, conn *models.AisdlcConnection) (*plugin.ApiResourceOutput, errors.Error) {
	if conn.GitlabConnectionId == 0 {
		return nil, errors.BadInput.New("gitlabConnectionId is required")
	}
	glConn, err := tasks.LoadGitlabConnection(basicRes.GetDal(), conn.GitlabConnectionId)
	if err != nil {
		return nil, err
	}
	if _, clientErr := tasks.NewGitlabClient(ctx, glConn.Endpoint, glConn.Token); clientErr != nil {
		return nil, errors.Default.Wrap(clientErr, "gitlab connection test failed")
	}
	return &plugin.ApiResourceOutput{
		Body: map[string]any{
			"success": true,
			"message": fmt.Sprintf("GitLab connection %d is usable", conn.GitlabConnectionId),
		},
		Status: http.StatusOK,
	}, nil
}
