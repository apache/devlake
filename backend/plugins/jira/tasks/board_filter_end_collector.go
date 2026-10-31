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
	"fmt"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/jira/models"
)

var CollectBoardFilterEndMeta = plugin.SubTaskMeta{
	Name:             "collectBoardFilterEnd",
	EntryPoint:       CollectBoardFilterEnd,
	EnabledByDefault: true,
	Description:      "Jira board filter jql checker after running",
	DomainTypes:      plugin.DOMAIN_TYPES,
}

func CollectBoardFilterEnd(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*JiraTaskData)
	logger := taskCtx.GetLogger()
	db := taskCtx.GetDal()
	logger.Info("collect board in collectBoardFilterEnd: %d", data.Options.BoardId)

	boardConfig, err := getBoardConfiguration(data)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("error getting board configuration for connection_id:%d board_id:%d", data.Options.ConnectionId, data.Options.BoardId))
	}
	filterId := boardConfig.Filter.ID
	logger.Info("collect board filter:%s", filterId)

	filterInfo, err := getBoardFilterJql(data, filterId)
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("error getting board filter jql for connection_id:%d board_id:%d", data.Options.ConnectionId, data.Options.BoardId))
	}
	jql := filterInfo.Jql
	// Fold the rendered ExtraJQL into the change-detection key. ExtraJQL filters
	// which issues are collected; if it changes we must force a full resync so
	// that issues no longer matching the filter are unlinked from the board
	// (otherwise they linger in board_issues and keep appearing on dashboards).
	if data.Options.ScopeConfig != nil && data.Options.ScopeConfig.ExtraJQL != "" {
		renderedExtra, renderErr := renderExtraJQL(data.Options.ScopeConfig.ExtraJQL, data)
		if renderErr != nil {
			return renderErr
		}
		jql = fmt.Sprintf("%s AND (%s)", jql, renderedExtra)
	}
	logger.Info("collect board filter jql:%s", jql)

	var record models.JiraBoard
	err = db.First(&record, dal.Where("connection_id = ? AND board_id = ? ", data.Options.ConnectionId, data.Options.BoardId))
	if err != nil {
		return errors.Default.Wrap(err, fmt.Sprintf("error finding record in _tool_jira_boards table for connection_id:%d board_id:%d", data.Options.ConnectionId, data.Options.BoardId))
	}
	logger.Info("get board filter jql:%s", record.Jql)

	cfg := taskCtx.GetConfigReader()
	autoRefresh := cfg.GetBool("JIRA_JQL_AUTO_FULL_REFRESH")

	if record.Jql != jql {
		if !autoRefresh {
			return errors.Default.New(fmt.Sprintf("connection_id:%d board_id:%d filter jql has changed, please use fullSync mode. And the previous jql is %s, now jql is %s", data.Options.ConnectionId, data.Options.BoardId, record.Jql, jql))
		}
		logger.Warn(nil, "connection_id:%d board_id:%d filter jql changed during collection (previous: %s, now: %s)", data.Options.ConnectionId, data.Options.BoardId, record.Jql, jql)
	}

	if record.SubQuery != boardConfig.SubQuery.Query {
		logger.Warn(nil, "connection_id:%d board_id:%d board sub-filter changed during collection (previous: %s, now: %s)", data.Options.ConnectionId, data.Options.BoardId, record.SubQuery, boardConfig.SubQuery.Query)
		if !autoRefresh {
			return errors.Default.New(fmt.Sprintf("connection_id:%d board_id:%d board sub-filter has changed during collection, please use fullSync mode. Previous sub-filter: %s, now: %s", data.Options.ConnectionId, data.Options.BoardId, record.SubQuery, boardConfig.SubQuery.Query))
		}
	}

	return nil
}
