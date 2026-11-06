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
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
)

func init() {
	RegisterSubtaskMeta(&CollectAccountsMeta)
}

const rawUserTable = "azuredevops_go_api_users"

var CollectAccountsMeta = plugin.SubTaskMeta{
	Name:             "collectAccounts",
	EntryPoint:       CollectAccounts,
	EnabledByDefault: true,
	Description:      "Collect Azure DevOps users.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{},
	ProductTables:    []string{RawPullRequestTable},
}

func CollectAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, rawUserTable)
	logger := taskCtx.GetLogger()

	if data.Options.OrganizationId == "" {
		logger.Info("OrganizationId is empty or On-Premises mode, skipping Cloud Graph user collection")
		return nil
	}

	connectionHelper := api.NewConnectionHelper(taskCtx, nil, "azuredevops_go")
	connection := &models.AzuredevopsConnection{}
	err := connectionHelper.FirstById(connection, data.Options.ConnectionId)
	if err != nil {
		return err
	}
	// Graph users are served by vssps, not the dev.azure.com client used for the rest of the collection.
	graphConnection := *connection
	graphConnection.Endpoint = "https://vssps.dev.azure.com/"
	graphClient, err := CreateApiClient(taskCtx.TaskContext(), &graphConnection)
	if err != nil {
		return err
	}
	defer graphClient.Release()

	collector, err := api.NewApiCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs:    *rawDataSubTaskArgs,
		ApiClient:             graphClient,
		UrlTemplate:           "{{ .Params.OrganizationId }}/_apis/graph/users?api-version=7.0-preview.1",
		PageSize:              500,
		GetNextPageCustomData: ExtractContToken,
		Query:                 BuildPaginator(true),
		AfterResponse:         change203To401,
		ResponseParser:        ParseRawMessageFromValue,
	})

	if err != nil {
		logger.Error(err, "collect user error")
		return err
	}

	return collector.Execute()
}
