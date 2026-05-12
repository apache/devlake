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
	"encoding/json"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/rootly/models"
	"github.com/apache/incubator-devlake/plugins/rootly/models/raw"
)

var _ plugin.SubTaskEntryPoint = ExtractServices

var ExtractServicesMeta = plugin.SubTaskMeta{
	Name:             "extractServices",
	EntryPoint:       ExtractServices,
	EnabledByDefault: true,
	Description:      "Extract Rootly services",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ExtractServices(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*RootlyTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_SERVICES_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			rawService := &raw.Service{}
			if err := errors.Convert(json.Unmarshal(row.Data, rawService)); err != nil {
				return nil, err
			}
			url := ""
			if rawService.Attributes.HtmlUrl != nil {
				url = *rawService.Attributes.HtmlUrl
			}
			service := &models.Service{
				Id:   rawService.Id,
				Name: rawService.Attributes.Name,
				Url:  url,
			}
			service.ConnectionId = data.Options.ConnectionId
			return []interface{}{service}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
