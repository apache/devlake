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

var _ plugin.MigrationScript = (*addUserPhotoUrl)(nil)

type addUserPhotoUrl struct{}

type asanaUser20250212 struct {
	PhotoUrl      string `gorm:"type:varchar(512)"`
	WorkspaceGids string `gorm:"type:text"`
}

func (asanaUser20250212) TableName() string {
	return "_tool_asana_users"
}

func (*addUserPhotoUrl) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	// Add photo_url and workspace_gids columns to _tool_asana_users table
	return db.AutoMigrate(&asanaUser20250212{})
}

func (*addUserPhotoUrl) Version() uint64 {
	return 20250212000001
}

func (*addUserPhotoUrl) Name() string {
	return "asana add photo_url and workspace_gids to users table"
}
