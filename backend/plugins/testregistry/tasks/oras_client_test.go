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

func TestGenerateUUID(t *testing.T) {
	t.Run("returns 32-char hex string", func(t *testing.T) {
		uuid, err := generateUUID()
		assert.Nil(t, err)
		assert.Len(t, uuid, 32)
	})

	t.Run("two calls produce different UUIDs", func(t *testing.T) {
		uuid1, _ := generateUUID()
		uuid2, _ := generateUUID()
		assert.NotEqual(t, uuid1, uuid2)
	})
}

func TestMin(t *testing.T) {
	assert.Equal(t, 3, min(3, 5))
	assert.Equal(t, 3, min(5, 3))
	assert.Equal(t, 3, min(3, 3))
	assert.Equal(t, 0, min(0, 1))
	assert.Equal(t, -1, min(-1, 0))
}
