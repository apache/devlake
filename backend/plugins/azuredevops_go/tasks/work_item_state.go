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
	"fmt"
	"net/url"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

type workItemMapper struct {
	processTemplate string
	categories      map[string]map[string]string
}

func newWorkItemMapper(taskCtx plugin.SubTaskContext, data *AzuredevopsTaskData) *workItemMapper {
	mapper := &workItemMapper{categories: map[string]map[string]string{}}
	if data.Options.ScopeConfig != nil {
		mapper.processTemplate = data.Options.ScopeConfig.ProcessTemplate
	}
	if mapper.processTemplate == "" {
		detected, err := detectProcessTemplate(data)
		if err != nil {
			taskCtx.GetLogger().Warn(err, "failed to detect Azure DevOps process template for %s/%s", data.Options.OrganizationId, data.Options.ProjectId)
		} else {
			mapper.processTemplate = detected
		}
	}
	return mapper
}

func detectProcessTemplate(data *AzuredevopsTaskData) (string, errors.Error) {
	path := fmt.Sprintf("%s_apis/projects/%s/properties?keys=%s&api-version=7.1-preview.1", adoOrgPrefix(data), url.PathEscape(data.Options.ProjectId), url.QueryEscape("System.Process Template"))
	res, err := data.ApiClient.Get(path, nil, nil)
	if err != nil {
		return "", err
	}
	var body struct {
		Value []struct {
			Name  string      `json:"name"`
			Value interface{} `json:"value"`
		} `json:"value"`
	}
	if err = readJSON(res, &body); err != nil {
		return "", err
	}
	for _, item := range body.Value {
		if item.Name == "System.Process Template" {
			return asString(item.Value), nil
		}
	}
	return "", nil
}

func (mapper *workItemMapper) loadStateCategory(data *AzuredevopsTaskData, workItemType string) {
	if workItemType == "" {
		return
	}
	if _, ok := mapper.categories[workItemType]; ok {
		return
	}
	path := fmt.Sprintf("%s/_apis/wit/workitemtypes/%s/states?api-version=7.1", adoScopePrefix(data), url.PathEscape(workItemType))
	res, err := data.ApiClient.Get(path, nil, nil)
	if err != nil {
		mapper.categories[workItemType] = map[string]string{}
		return
	}
	var body struct {
		Value []struct {
			Name          string `json:"name"`
			Category      string `json:"category"`
			StateCategory string `json:"stateCategory"`
		} `json:"value"`
	}
	if err = readJSON(res, &body); err != nil {
		mapper.categories[workItemType] = map[string]string{}
		return
	}
	categories := map[string]string{}
	for _, item := range body.Value {
		if item.Name == "" {
			continue
		}
		category := item.Category
		if category == "" {
			category = item.StateCategory
		}
		categories[item.Name] = category
	}
	mapper.categories[workItemType] = categories
}
