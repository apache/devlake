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
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/grafana_irm/models"
)

// GrafanaIrmOptions carries the plumbing options every subtask needs.
type GrafanaIrmOptions struct {
	ConnectionId  uint64                        `json:"connectionId" mapstructure:"connectionId,omitempty"`
	ScopeId       string                        `json:"scopeId,omitempty" mapstructure:"scopeId,omitempty"`
	ScopeConfigId uint64                        `json:"scopeConfigId,omitempty" mapstructure:"scopeConfigId,omitempty"`
	ScopeConfig   *models.GrafanaIrmScopeConfig `json:"scopeConfig,omitempty" mapstructure:"scopeConfig,omitempty"`
}

type GrafanaIrmTaskData struct {
	Options *GrafanaIrmOptions
	Client  api.RateLimitedApiClient
	// Connection is needed by the extractor to build an absolute Issue.Url
	// from the wire's relative `overviewURL` (verified live, see
	// grafana_irm_plan.md §3.4).
	Connection *models.GrafanaIrmConnection
}

func (p *GrafanaIrmOptions) GetParams() any {
	return models.GrafanaIrmParams{
		ConnectionId: p.ConnectionId,
		ScopeId:      p.ScopeId,
	}
}

func DecodeAndValidateTaskOptions(options map[string]interface{}) (*GrafanaIrmOptions, errors.Error) {
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

func DecodeTaskOptions(options map[string]interface{}) (*GrafanaIrmOptions, errors.Error) {
	var op GrafanaIrmOptions
	err := api.Decode(options, &op, nil)
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func ValidateTaskOptions(op *GrafanaIrmOptions) errors.Error {
	if op.ConnectionId == 0 {
		return errors.BadInput.New("connectionId is invalid")
	}
	return nil
}
