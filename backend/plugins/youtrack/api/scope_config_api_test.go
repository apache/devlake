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

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultScopeConfigEntities(t *testing.T) {
	// a body without entities gets the plugin's fixed domain pair — the
	// config-ui sends these itself, but a raw API caller must not end up
	// with a scope config that collects nothing
	body := map[string]interface{}{"name": "my config"}
	defaultScopeConfigEntities(body)
	assert.Equal(t, []string{"TICKET", "CROSS"}, body["entities"])

	// an explicit empty list is just as absent
	body = map[string]interface{}{"name": "my config", "entities": []string{}}
	defaultScopeConfigEntities(body)
	assert.Equal(t, []string{"TICKET", "CROSS"}, body["entities"])

	// a caller's own selection is never overridden
	body = map[string]interface{}{"name": "my config", "entities": []string{"TICKET"}}
	defaultScopeConfigEntities(body)
	assert.Equal(t, []string{"TICKET"}, body["entities"])
}

func TestDefaultScopeConfigEntitiesFromJson(t *testing.T) {
	// over the wire the body is JSON-decoded into map[string]interface{}, so
	// entities arrives as []interface{} — the shape gin actually hands us

	// an explicit caller selection must survive
	body := map[string]interface{}{"name": "my config", "entities": []interface{}{"TICKET"}}
	defaultScopeConfigEntities(body)
	assert.Equal(t, []interface{}{"TICKET"}, body["entities"])

	// an explicit empty JSON array is just as absent
	body = map[string]interface{}{"name": "my config", "entities": []interface{}{}}
	defaultScopeConfigEntities(body)
	assert.Equal(t, []string{"TICKET", "CROSS"}, body["entities"])
}
