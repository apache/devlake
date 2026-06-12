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

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

var _ plugin.MigrationScript = (*addAuthSessions)(nil)

type authSession20260426 struct {
	Jti        string     `gorm:"primaryKey;type:varchar(36)"`
	Sub        string     `gorm:"type:varchar(255);index"`
	Email      string     `gorm:"type:varchar(255)"`
	Name       string     `gorm:"type:varchar(255)"`
	IssuedAt   time.Time  `gorm:"not null"`
	ExpiresAt  time.Time  `gorm:"index;not null"`
	RevokedAt  *time.Time `gorm:"index"`
	LastSeenAt time.Time
}

func (authSession20260426) TableName() string {
	return "auth_sessions"
}

type addAuthSessions struct{}

func (*addAuthSessions) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, new(authSession20260426))
}

func (*addAuthSessions) Version() uint64 {
	return 20260426000001
}

func (*addAuthSessions) Name() string {
	return "add auth_sessions table for session revocation"
}
