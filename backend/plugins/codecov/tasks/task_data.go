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
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

type CodecovOptions struct {
	ConnectionId  uint64                     `json:"connectionId" mapstructure:"connectionId,omitempty"`
	ScopeConfigId uint64                     `json:"scopeConfigId" mapstructure:"scopeConfigId,omitempty"`
	FullName      string                     `json:"fullName" mapstructure:"fullName,omitempty"`
	ScopeConfig   *models.CodecovScopeConfig `mapstructure:"scopeConfig,omitempty" json:"scopeConfig"`
}

type CodecovTaskData struct {
	Options   *CodecovOptions
	ApiClient *helper.ApiAsyncClient
	Repo      *models.CodecovRepo
	Service   string // Codecov API service segment (github, gitlab, etc.)
}

// CodecovApiParams matches the models.CodecovApiParams
type CodecovApiParams models.CodecovApiParams

func DecodeAndValidateTaskOptions(options map[string]interface{}) (*CodecovOptions, errors.Error) {
	op, err := DecodeTaskOptions(options)
	if err != nil {
		return nil, err
	}
	err = ValidateTaskOptions(op)
	if err != nil {
		return nil, err
	}
	return op, nil
}

func DecodeTaskOptions(options map[string]interface{}) (*CodecovOptions, errors.Error) {
	var op CodecovOptions
	err := helper.Decode(options, &op, nil)
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func ValidateTaskOptions(op *CodecovOptions) errors.Error {
	if op.ConnectionId == 0 {
		return errors.BadInput.New("connectionId is invalid")
	}
	if op.FullName == "" {
		return errors.BadInput.New("fullName is required")
	}
	return nil
}
