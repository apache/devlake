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
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

var _ plugin.MigrationScript = (*addDisplayNameFields)(nil)

type addDisplayNameFields struct{}

type QDevConnection20250623 struct {
	IdentityStoreId     string `gorm:"type:VARCHAR(255)"`
	IdentityStoreRegion string `gorm:"type:VARCHAR(255)"`
}

func (QDevConnection20250623) TableName() string {
	return "_tool_q_dev_connections"
}

type QDevUserData20250623 struct {
	DisplayName string `gorm:"type:VARCHAR(255)"`
}

func (QDevUserData20250623) TableName() string {
	return "_tool_q_dev_user_data"
}

type QDevUserMetrics20250623 struct {
	DisplayName string `gorm:"type:VARCHAR(255)"`
}

func (QDevUserMetrics20250623) TableName() string {
	return "_tool_q_dev_user_metrics"
}

func (*addDisplayNameFields) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes,
		&QDevConnection20250623{},
		&QDevUserData20250623{},
		&QDevUserMetrics20250623{},
	)
}

func (*addDisplayNameFields) Version() uint64 {
	return 20250623000001
}

func (*addDisplayNameFields) Name() string {
	return "add Identity Center fields to connections and display_name fields to user tables"
}
