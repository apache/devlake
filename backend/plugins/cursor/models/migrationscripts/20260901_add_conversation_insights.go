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

package migrationscripts

import (
	"time"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/migrationscripts/archived"
	"github.com/apache/devlake/helpers/migrationhelper"
)

type addCursorConversationInsights struct{}

type cursorConnection20260901 struct {
	HasConversationInsights bool
}

func (cursorConnection20260901) TableName() string { return "_tool_cursor_connections" }

type cursorConversationInsight20260901 struct {
	ConnectionId     uint64     `gorm:"primaryKey"`
	ScopeId          string     `gorm:"primaryKey;type:varchar(255)"`
	RecordId         string     `gorm:"primaryKey;type:varchar(64)"`
	PeriodStart      time.Time  `gorm:"index"`
	PeriodEnd        time.Time  `gorm:"index"`
	InsightType      string     `gorm:"type:varchar(32);index"`
	AggregationType  string     `gorm:"type:varchar(32);index"`
	DimensionValue   string     `gorm:"type:varchar(255);index"`
	SubcategoryGroup string     `gorm:"type:varchar(32);index"`
	MetricDate       *time.Time `gorm:"index"`
	Count            int
	archived.NoPKModel
}

func (cursorConversationInsight20260901) TableName() string {
	return "_tool_cursor_conversation_insights"
}

func (*addCursorConversationInsights) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&cursorConnection20260901{},
		&cursorConversationInsight20260901{},
	)
}

func (*addCursorConversationInsights) Version() uint64 { return 20260903120000 }

func (*addCursorConversationInsights) Name() string {
	return "cursor add conversation insights table and has_conversation_insights column"
}
