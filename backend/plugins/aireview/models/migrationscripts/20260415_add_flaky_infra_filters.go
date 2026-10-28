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
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addFlakyInfraFilters)(nil)

type addFlakyInfraFilters struct{}

// Up adds ExcludeFlakyTests and ExcludeInfraFailures columns to scope config.
// Both default to false (off) so existing behavior is preserved until explicitly enabled.
func (script *addFlakyInfraFilters) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	if err := db.AutoMigrate(&scopeConfigFlakyInfra20260415{}); err != nil {
		return errors.Default.Wrap(err, "failed to migrate _tool_aireview_scope_configs for flaky/infra filters")
	}

	return nil
}

func (script *addFlakyInfraFilters) Version() uint64 {
	return 20260415000001
}

func (script *addFlakyInfraFilters) Name() string {
	return "aireview add flaky test and infrastructure failure filter toggles"
}

type scopeConfigFlakyInfra20260415 struct {
	ExcludeFlakyTests    bool `gorm:"type:boolean;default:false"`
	ExcludeInfraFailures bool `gorm:"type:boolean;default:false"`
}

func (scopeConfigFlakyInfra20260415) TableName() string {
	return "_tool_aireview_scope_configs"
}
