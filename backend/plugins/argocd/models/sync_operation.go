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

package models

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/common"
)

type ArgocdSyncOperation struct {
	ConnectionId    uint64 `gorm:"primaryKey"`
	ApplicationName string `gorm:"primaryKey;type:varchar(255)"`
	DeploymentId    int64  `gorm:"primaryKey"`        // History ID from ArgoCD
	Revision        string `gorm:"type:varchar(255)"` // Git SHA
	RepoURL         string `gorm:"type:varchar(500)"` // Git repo URL resolved from source/sources at extraction time
	Kind            string `gorm:"type:varchar(100)"` // Kubernetes resource kind: Deployment, ReplicaSet, Rollout, StatefulSet, DaemonSet, etc.
	StartedAt       *time.Time
	FinishedAt      *time.Time
	Phase           string `gorm:"type:varchar(100)"` // Succeeded, Failed, Error, Running, Terminating
	Message         string `gorm:"type:text"`
	InitiatedBy     string `gorm:"type:varchar(255)"` // Username or automated
	SyncStatus      string `gorm:"type:varchar(100)"` // Synced, OutOfSync
	HealthStatus    string `gorm:"type:varchar(100)"` // Healthy, Degraded, etc.
	ResourcesCount  int
	ContainerImages []string `gorm:"type:json;serializer:json"`
	common.NoPKModel
}

func (ArgocdSyncOperation) TableName() string {
	return "_tool_argocd_sync_operations"
}
