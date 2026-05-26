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
)

var _ plugin.MigrationScript = (*addOrganizationIdToUserMetrics)(nil)

type addOrganizationIdToUserMetrics struct{}

func (script *addOrganizationIdToUserMetrics) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	return db.AutoMigrate(&userDailyMetrics20260303{})
}

func (*addOrganizationIdToUserMetrics) Version() uint64 {
	return 20260303000000
}

func (*addOrganizationIdToUserMetrics) Name() string {
	return "add organization_id to user daily metrics"
}

type userDailyMetrics20260303 struct {
	OrganizationId string `gorm:"type:varchar(100)"`
}

func (userDailyMetrics20260303) TableName() string {
	return "_tool_copilot_user_daily_metrics"
}
