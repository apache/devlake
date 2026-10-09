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

package service

import (
	"testing"

	"github.com/apache/devlake/plugins/cursor/models"
	"github.com/stretchr/testify/require"
)

func TestApplyOptionalPermissions_UpgradesEnterpriseAndSetsFlags(t *testing.T) {
	conn := &models.CursorConnection{
		CursorConn: models.CursorConn{KeyTier: models.KeyTierTeam},
	}
	ApplyOptionalPermissions(conn, AdminApiPermissions{
		Analytics:            true,
		BugbotReviews:        true,
		ConversationInsights: true,
		AiCodeCommitDetails:  true,
	})
	require.Equal(t, models.KeyTierEnterprise, conn.KeyTier)
	require.True(t, conn.HasBugbotReviews)
	require.True(t, conn.HasConversationInsights)
	require.True(t, conn.HasAiCodeCommitDetails)
}

func TestApplyOptionalPermissions_DoesNotDowngradeEnterprise(t *testing.T) {
	conn := &models.CursorConnection{
		CursorConn: models.CursorConn{
			KeyTier:                 models.KeyTierEnterprise,
			HasBugbotReviews:        true,
			HasConversationInsights: true,
		},
	}
	ApplyOptionalPermissions(conn, AdminApiPermissions{
		Analytics:            false,
		AiCodeTracking:       false,
		BugbotReviews:        false,
		ConversationInsights: false,
	})
	require.Equal(t, models.KeyTierEnterprise, conn.KeyTier)
	require.False(t, conn.HasBugbotReviews)
	require.False(t, conn.HasConversationInsights)
	require.False(t, conn.HasAiCodeCommitDetails)
}

func TestSnapshotConnectionCapabilitiesDetectsChanges(t *testing.T) {
	conn := &models.CursorConnection{
		CursorConn: models.CursorConn{
			KeyTier:                 models.KeyTierTeam,
			HasConversationInsights: false,
		},
	}
	before := snapshotConnectionCapabilities(conn)
	ApplyOptionalPermissions(conn, AdminApiPermissions{ConversationInsights: true})
	after := snapshotConnectionCapabilities(conn)
	require.NotEqual(t, before, after)
}
