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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

var ConvertFlagsMeta = plugin.SubTaskMeta{
	Name:             "ConvertFlags",
	EntryPoint:       ConvertFlags,
	EnabledByDefault: true,
	Description:      "Convert flags from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	DependencyTables: []string{RAW_FLAGS_TABLE},
}

func ConvertFlags(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_FLAGS_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			var flag struct {
				FlagName     string   `json:"flag_name"`
				Coverage     *float64 `json:"coverage"`
				Carryforward bool     `json:"carryforward"`
				Deleted      bool     `json:"deleted"`
				Yaml         string   `json:"yaml"`
			}
			err := errors.Convert(json.Unmarshal(resData.Data, &flag))
			if err != nil {
				return nil, err
			}

			// Skip empty flag names
			if flag.FlagName == "" {
				return nil, nil
			}

			codecovFlag := &models.CodecovFlag{
				NoPKModel:    common.NoPKModel{},
				ConnectionId: data.Options.ConnectionId,
				RepoId:       data.Options.FullName,
				FlagName:     flag.FlagName,
				Coverage:     flag.Coverage,
				Carryforward: flag.Carryforward,
				Deleted:      flag.Deleted,
				Yaml:         flag.Yaml,
			}

			return []interface{}{codecovFlag}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
