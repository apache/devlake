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

package models

import (
	"testing"
	"time"

	coreModels "github.com/apache/devlake/core/models"
	"github.com/stretchr/testify/assert"
)

func TestApplySyncPolicySetsIncrementalOnlyForFullSync(t *testing.T) {
	timeAfter := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("normal sync does not force incremental", func(t *testing.T) {
		data := &RemotePluginTaskData{}
		ApplySyncPolicy(data, &coreModels.SyncPolicy{TimeAfter: &timeAfter})
		_, hasIncremental := data.Options["incremental"]
		assert.False(t, hasIncremental)
		assert.NotNil(t, data.SyncPolicy)
		assert.Equal(t, &timeAfter, data.SyncPolicy.TimeAfter)
	})

	t.Run("full sync forces non-incremental", func(t *testing.T) {
		data := &RemotePluginTaskData{}
		ApplySyncPolicy(data, &coreModels.SyncPolicy{
			TimeAfter: &timeAfter,
			TriggerSyncPolicy: coreModels.TriggerSyncPolicy{
				FullSync: true,
			},
		})
		incremental, ok := data.Options["incremental"].(bool)
		assert.True(t, ok)
		assert.False(t, incremental)
	})
}
