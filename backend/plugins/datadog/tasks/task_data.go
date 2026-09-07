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
	"github.com/apache/devlake/plugins/datadog/models"
)

type DatadogOptions struct {
	ConnectionId     uint64                     `json:"connectionId" mapstructure:"connectionId,omitempty"`
	IncidentTypeId   string                     `json:"incidentTypeId,omitempty" mapstructure:"incidentTypeId,omitempty"`
	IncidentTypeName string                     `json:"incidentTypeName,omitempty" mapstructure:"incidentTypeName,omitempty"`
	ScopeConfigId    uint64                     `json:"scopeConfigId,omitempty" mapstructure:"scopeConfigId,omitempty"`
	ScopeConfig      *models.DatadogScopeConfig `json:"scopeConfig,omitempty" mapstructure:"scopeConfig,omitempty"`
}

type DatadogTaskData struct {
	Options *DatadogOptions
	Client  api.RateLimitedApiClient
	// WebUrl comes from the connection, not the API: Datadog returns no
	// incident permalink, so the domain issue URL is built from it.
	WebUrl string
}

func (p *DatadogOptions) GetParams() any {
	scopeId := p.IncidentTypeId
	if scopeId == "" {
		scopeId = "all"
	}
	return models.DatadogParams{
		ConnectionId: p.ConnectionId,
		ScopeId:      scopeId,
	}
}

// ComponentField and SeverityField read through a possibly absent scope
// config, so callers don't repeat the nil check.
func (p *DatadogOptions) ComponentField() string {
	if p.ScopeConfig == nil {
		return ""
	}
	return p.ScopeConfig.ComponentField
}

func (p *DatadogOptions) SeverityField() string {
	if p.ScopeConfig == nil {
		return ""
	}
	return p.ScopeConfig.SeverityField
}

func DecodeAndValidateTaskOptions(options map[string]interface{}) (*DatadogOptions, errors.Error) {
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

func DecodeTaskOptions(options map[string]interface{}) (*DatadogOptions, errors.Error) {
	var op DatadogOptions
	err := api.Decode(options, &op, nil)
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func ValidateTaskOptions(op *DatadogOptions) errors.Error {
	if op.ConnectionId == 0 {
		return errors.BadInput.New("connectionId is invalid")
	}
	return nil
}
