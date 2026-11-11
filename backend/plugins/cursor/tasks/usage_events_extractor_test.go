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
)

func TestResolveUsageEventActor(t *testing.T) {
	email, key, ok := resolveUsageEventActor("user@example.com", "null")
	assert.True(t, ok)
	assert.Equal(t, "user@example.com", email)
	assert.Equal(t, "user@example.com", key)

	email, key, ok = resolveUsageEventActor("", "sa_radbot_123")
	assert.True(t, ok)
	assert.Equal(t, "", email)
	assert.Equal(t, "sa:sa_radbot_123", key)

	_, _, ok = resolveUsageEventActor("", "")
	assert.False(t, ok)

	_, _, ok = resolveUsageEventActor("  ", "null")
	assert.False(t, ok)
}
