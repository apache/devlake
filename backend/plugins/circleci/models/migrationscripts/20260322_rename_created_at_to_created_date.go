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
)

type circleciWorkflow20260322 struct {
	CreatedDate *time.Time
}

func (circleciWorkflow20260322) TableName() string {
	return "_tool_circleci_workflows"
}

type circleciJob20260322 struct {
	CreatedDate *time.Time
}

func (circleciJob20260322) TableName() string {
	return "_tool_circleci_jobs"
}

type renameCreatedAtToCreatedDate20260322 struct{}

func (*renameCreatedAtToCreatedDate20260322) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	if err := db.AutoMigrate(&circleciWorkflow20260322{}); err != nil {
		return err
	}
	if err := db.Exec("UPDATE _tool_circleci_workflows SET created_date = created_at WHERE created_date IS NULL"); err != nil {
		return err
	}

	if err := db.AutoMigrate(&circleciJob20260322{}); err != nil {
		return err
	}
	if err := db.Exec("UPDATE _tool_circleci_jobs SET created_date = created_at WHERE created_date IS NULL"); err != nil {
		return err
	}

	return nil
}

func (*renameCreatedAtToCreatedDate20260322) Version() uint64 {
	return 20260322000001
}

func (*renameCreatedAtToCreatedDate20260322) Name() string {
	return "circleci rename created_at to created_date in workflows and jobs"
}
