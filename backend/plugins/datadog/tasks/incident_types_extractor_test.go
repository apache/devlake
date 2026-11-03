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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apache/devlake/plugins/datadog/models/raw"
)

func TestNewIncidentTypeKeepsScopeConfigId(t *testing.T) {
	prefix := "IR"
	rawType := &raw.IncidentType{
		Id:         "type-uuid-1",
		Attributes: &raw.IncidentTypeAttributes{Name: "General Incident", Prefix: &prefix},
	}

	kept := newIncidentType(rawType, 1, map[string]uint64{"type-uuid-1": 7})
	assert.Equal(t, uint64(7), kept.ScopeConfigId)
	assert.Equal(t, uint64(1), kept.ConnectionId)
	assert.Equal(t, "IR", kept.Prefix)

	fresh := newIncidentType(rawType, 1, map[string]uint64{"other-type": 7})
	assert.Equal(t, uint64(0), fresh.ScopeConfigId)
}
