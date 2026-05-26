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

package srvhelper

import (
	"testing"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/magiconair/properties/assert"
)

func Test_setDefaultEntities(t *testing.T) {
	// plugin doesn't embed the common ScopeConfig
	sc1 := &struct {
		Entities []string
	}{
		Entities: nil,
	}
	setDefaultEntities(sc1)
	assert.Equal(t, sc1.Entities, plugin.DOMAIN_TYPES)

	// plugin embedded the common ScopeConfig
	sc2 := &struct {
		common.ScopeConfig
	}{
		ScopeConfig: common.ScopeConfig{
			Entities: nil,
		},
	}
	setDefaultEntities(sc2)
	assert.Equal(t, sc2.Entities, plugin.DOMAIN_TYPES)

	// should not override a non empty slice
	sc3 := &common.ScopeConfig{
		Entities: []string{plugin.DOMAIN_TYPE_CICD},
	}
	setDefaultEntities(sc3)
	assert.Equal(t, sc3.Entities, []string{plugin.DOMAIN_TYPE_CICD})
}
