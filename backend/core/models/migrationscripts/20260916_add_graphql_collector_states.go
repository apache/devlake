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
	"github.com/apache/devlake/core/plugin"
)

type graphqlCollectorState20260916 struct {
	ScopeHash     string `gorm:"primaryKey;type:char(64)"`
	InputHash     string `gorm:"primaryKey;type:varchar(64)"`
	TaskID        uint64
	RawDataTable  string `gorm:"type:varchar(255)"`
	RawDataParams string `gorm:"type:text"`
	Completed     bool
	SkipCursor    string `gorm:"type:text"`
	UpdatedAt     time.Time
	Contract      string `gorm:"type:varchar(64)"`
	InputDigest   string `gorm:"type:varchar(64)"`
	RawCount      *int64
}

func (graphqlCollectorState20260916) TableName() string { return "_devlake_graphql_collector_states" }

type addGraphqlCollectorStates struct{}

var _ plugin.MigrationScript = (*addGraphqlCollectorStates)(nil)

func (*addGraphqlCollectorStates) Up(res context.BasicRes) errors.Error {
	return res.GetDal().AutoMigrate(&graphqlCollectorState20260916{})
}
func (*addGraphqlCollectorStates) Version() uint64 { return 20260916000001 }
func (*addGraphqlCollectorStates) Name() string    { return "add graphql collector checkpoints" }
