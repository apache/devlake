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
	stdctx "context"
	"strings"

	corectx "github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

type connectionCapabilities struct {
	KeyTier                 string
	HasBugbotReviews        bool
	HasConversationInsights bool
}

func snapshotConnectionCapabilities(connection *models.CursorConnection) connectionCapabilities {
	if connection == nil {
		return connectionCapabilities{}
	}
	return connectionCapabilities{
		KeyTier:                 connection.KeyTier,
		HasBugbotReviews:        connection.HasBugbotReviews,
		HasConversationInsights: connection.HasConversationInsights,
	}
}

func probeOptionalEndpoints(apiClient *helper.ApiClient) AdminApiPermissions {
	return AdminApiPermissions{
		Analytics:            probeEndpoint(apiClient, "analytics/team/dau?startDate=7d&endDate=today"),
		AiCodeTracking:       probeEndpoint(apiClient, "analytics/ai-code/commits?page=1&pageSize=1"),
		BugbotReviews:        probeEndpoint(apiClient, "analytics/team/bugbot-reviews?page=1&pageSize=1"),
		ConversationInsights: probeEndpoint(apiClient, "analytics/team/conversation-insights?startDate=7d&endDate=today&include=intents"),
	}
}

// ApplyOptionalPermissions updates optional collection flags from live API probes.
// Key tier is upgraded to enterprise when analytics endpoints respond; it is never
// downgraded so a transient probe failure does not disable enterprise collectors.
func ApplyOptionalPermissions(connection *models.CursorConnection, perms AdminApiPermissions) {
	if connection == nil {
		return
	}
	if perms.Analytics || perms.AiCodeTracking {
		connection.KeyTier = models.KeyTierEnterprise
	}
	connection.HasBugbotReviews = perms.BugbotReviews
	connection.HasConversationInsights = perms.ConversationInsights
}

// RefreshOptionalPermissions re-probes optional Cursor Admin API endpoints and updates
// the in-memory connection. Call at pipeline start so collectors reflect current
// Cursor team settings without requiring a manual Test Connection.
func RefreshOptionalPermissions(ctx stdctx.Context, br corectx.BasicRes, connection *models.CursorConnection) (changed bool, err errors.Error) {
	if connection == nil {
		return false, nil
	}
	connection.Normalize()
	if strings.TrimSpace(connection.Token) == "" || connection.KeyTier == models.KeyTierPersonal {
		return false, nil
	}

	apiClient, err := helper.NewApiClientFromConnection(ctx, br, connection)
	if err != nil {
		return false, err
	}
	apiClient.SetHeaders(map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})

	before := snapshotConnectionCapabilities(connection)
	perms := probeOptionalEndpoints(apiClient)
	ApplyOptionalPermissions(connection, perms)
	changed = before != snapshotConnectionCapabilities(connection)

	br.GetLogger().Info(
		"cursor optional permissions refreshed: keyTier=%s bugbotReviews=%v conversationInsights=%v",
		connection.KeyTier,
		connection.HasBugbotReviews,
		connection.HasConversationInsights,
	)
	return changed, nil
}

// RefreshAndPersistOptionalPermissions probes optional endpoints and saves changes to the connection row.
func RefreshAndPersistOptionalPermissions(ctx stdctx.Context, br corectx.BasicRes, connection *models.CursorConnection) errors.Error {
	changed, err := RefreshOptionalPermissions(ctx, br, connection)
	if err != nil {
		return err
	}
	if !changed || connection.ID == 0 {
		return nil
	}
	return br.GetDal().CreateOrUpdate(connection)
}
