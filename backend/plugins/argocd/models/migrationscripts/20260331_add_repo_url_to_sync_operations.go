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

var _ plugin.MigrationScript = (*addRepoURLToSyncOperations)(nil)

type addRepoURLToSyncOperations struct{}

// addRepoURLSyncOpArchived is a snapshot of ArgocdSyncOperation used solely
// for this migration so the live model can evolve independently.
type addRepoURLSyncOpArchived struct {
	ConnectionId    uint64 `gorm:"primaryKey"`
	ApplicationName string `gorm:"primaryKey;type:varchar(255)"`
	DeploymentId    int64  `gorm:"primaryKey"`
	RepoURL         string `gorm:"type:varchar(500)"`
}

func (addRepoURLSyncOpArchived) TableName() string {
	return "_tool_argocd_sync_operations"
}

func (m *addRepoURLToSyncOperations) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	return db.AutoMigrate(&addRepoURLSyncOpArchived{})
}

func (*addRepoURLToSyncOperations) Version() uint64 {
	return 20260331000000
}

func (*addRepoURLToSyncOperations) Name() string {
	return "argocd add repo_url to sync operations"
}
