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
	"time"

	"github.com/apache/devlake/core/errors"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

// defaultPageSize is the page size used when the task options do not set one.
// ApiCollector only follows the next pages of a list when the page size is set.
const defaultPageSize = 50

type TeambitionOptions struct {
	ConnectionId        uint64 `json:"connectionId"`
	ProjectId           string `json:"projectId"`
	PageSize            uint64 `mapstruct:"pageSize"`
	CstZone             *time.Location
	TransformationRules TransformationRules `json:"transformationRules"`
}

type TeambitionTaskData struct {
	Options   *TeambitionOptions
	ApiClient *helper.ApiAsyncClient
	TenantId  string
}

func DecodeAndValidateTaskOptions(options map[string]interface{}) (*TeambitionOptions, errors.Error) {
	var op TeambitionOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, err
	}
	if op.ConnectionId == 0 {
		return nil, errors.Default.New("connectionId is invalid")
	}
	if op.PageSize == 0 {
		op.PageSize = defaultPageSize
	}
	return &op, nil
}

type TypeMapping struct {
	StandardType string `json:"standardType"`
}

type OriginalStatus []string

type StatusMappings map[string]OriginalStatus

type TypeMappings map[string]TypeMapping

type TransformationRules struct {
	TypeMappings   TypeMappings   `json:"typeMappings"`
	StatusMappings StatusMappings `json:"statusMappings"`
}
